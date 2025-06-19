package confgen

import (
	"fmt"
	"strconv"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type tableParser struct {
	*sheetParser
}

func (p *tableParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	header := parseroptions.MergeHeader(p.sheetOpts, p.bookOpts, nil)
	if p.sheetOpts.Transpose {
		// interchange the rows and columns
		// namerow: name column
		// [datarow, MaxCol]: data column
		// kvRow := make(map[string]string)
		p.names = make([]string, sheet.Table.RowSize())
		p.types = make([]string, sheet.Table.RowSize())
		nameCol := sheet.Table.BeginCol() + header.NameRow - 1
		typeCol := sheet.Table.BeginCol() + header.TypeRow - 1
		dataCol := sheet.Table.BeginCol() + header.DataRow - 1
		var prev *book.RowCells
		for col := dataCol; col < sheet.Table.EndCol(); col++ {
			curr := book.NewRowCells(col, prev, sheet.Name)
			for row := sheet.Table.BeginRow(); row < sheet.Table.EndRow(); row++ {
				if col == dataCol {
					nameCell, err := sheet.Table.Cell(row, nameCol)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					p.names[row] = book.ExtractFromCell(nameCell, header.NameLine)

					if header.TypeRow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(row, typeCol)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						p.types[row] = book.ExtractFromCell(typeCell, header.TypeLine)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(row, &p.names[row], &p.types[row], data, p.sheetOpts.AdjacentKey)
				if name := p.names[row]; name != "" {
					if foundRow, ok := p.lookupTable[name]; ok && foundRow != row {
						return xerrors.E0003(name, excel.Postion(foundRow, nameCol), excel.Postion(row, nameCol))
					}
					p.lookupTable[name] = row
				}
			}
			curr.SetColumnLookupTable(p.lookupTable)
			_, err := p.parseMessage(nil, msg, curr, "", "")
			if err != nil {
				return err
			}

			if prev != nil {
				prev.Free()
			}
			prev = curr
		}
	} else {
		// namerow: name row
		// [datarow, MaxRow]: data row
		p.names = make([]string, sheet.Table.ColSize())
		p.types = make([]string, sheet.Table.ColSize())
		nameRow := sheet.Table.BeginRow() + header.NameRow - 1
		typeRow := sheet.Table.BeginRow() + header.TypeRow - 1
		dataRow := sheet.Table.BeginRow() + header.DataRow - 1
		var prev *book.RowCells
		for row := dataRow; row < sheet.Table.EndRow(); row++ {
			curr := book.NewRowCells(row, prev, sheet.Name)
			for col := sheet.Table.BeginCol(); col < sheet.Table.EndCol(); col++ {
				if row == dataRow {
					nameCell, err := sheet.Table.Cell(nameRow, col)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					p.names[col] = book.ExtractFromCell(nameCell, header.NameLine)

					if header.TypeRow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(typeRow, col)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						p.types[col] = book.ExtractFromCell(typeCell, header.TypeLine)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(col, &p.names[col], &p.types[col], data, p.sheetOpts.AdjacentKey)
				if name := p.names[col]; name != "" {
					if foundCol, ok := p.lookupTable[name]; ok && foundCol != col {
						return xerrors.E0003(name, excel.Postion(nameRow, foundCol), excel.Postion(nameRow, col))
					}
					p.lookupTable[name] = col
				}
			}
			curr.SetColumnLookupTable(p.lookupTable)
			_, err := p.parseMessage(nil, msg, curr, "", "")
			if err != nil {
				return err
			}

			if prev != nil {
				prev.Free()
			}
			prev = curr
		}
	}
	return nil
}

// parseMessage parses all fields of a protobuf message.
func (p *tableParser) parseMessage(parentField *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	if rc.Ignored() {
		// ignore this row
		return false, nil
	}
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			// TODO(performance): cache the parsed field for reuse, as each table row will be parsed repeatedly.
			field := p.parseFieldDescriptor(fd)
			field.mergeParentFieldProp(parentField)
			defer field.release()
			newCardPrefix := cardPrefix + "." + string(fd.Name())
			fieldPresent, err := p.parseField(field, msg, rc, prefix, newCardPrefix)
			if err != nil {
				return xerrors.WrapKV(err,
					xerrors.KeyPBFieldType, xproto.GetFieldTypeName(fd),
					xerrors.KeyPBFieldName, fd.FullName(),
					xerrors.KeyPBFieldOpts, field.opts)
			}
			if fieldPresent {
				// The message is treated as present at least one field is present.
				present = true
			}
			return nil
		}()
		if err != nil {
			return false, err
		}
	}
	return present, nil
}

func (p *tableParser) parseField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	// log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return p.parseMapField(field, msg, rc, prefix, cardPrefix)
	} else if field.fd.IsList() {
		return p.parseListField(field, msg, rc, prefix, cardPrefix)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		if xproto.IsUnionField(field.fd) {
			return p.parseUnionField(field, msg, rc, prefix, cardPrefix)
		}
		return p.parseStructField(field, msg, rc, prefix, cardPrefix)
	} else {
		return p.parseScalarField(field, msg, rc, prefix)
	}
}

func (p *tableParser) parseMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	layout := parseTableMapLayout(field.opts.GetLayout())
	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		// map default layout treated as virtical
		return p.parseVerticalMapField(field, msg, rc, prefix, cardPrefix)
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		return p.parseHorizontalMapField(field, msg, rc, prefix, cardPrefix)
	case tableaupb.Layout_LAYOUT_INCELL:
		// NOTE(Wenchy): Even though named as incell, it still can merge
		// multiple vertical/horizontal cells if provided, as map is a
		// composite type with cardinality. In practice, it is a very useful.
		return p.parseIncellMapField(field, msg, rc, prefix)
	default:
		return false, xerrors.Errorf("unknown layout: %v", layout)
	}
}

func (p *tableParser) parseVerticalMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	if field.fd.MapValue().Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("vertical map value as scalar type is not supported")
	}
	newPrefix := prefix + field.opts.Name
	keyColName := newPrefix + field.opts.Key
	cell, err := rc.Cell(keyColName, p.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	reflectMap := msg.Mutable(field.fd).Map()
	newMapKey, keyPresent, err := p.parseMapKey(field, reflectMap, cell.Data)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	newCardPrefix := cardPrefix + "." + escapeMapKey(newMapKey.Value())
	// value must be empty if key not present
	if !keyPresent && reflectMap.Has(newMapKey) {
		tempCheckMapValue := reflectMap.NewValue()
		valuePresent, err := p.parseMessage(field, tempCheckMapValue.Message(), rc, newPrefix, newCardPrefix)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		if valuePresent {
			return false, xerrors.WrapKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
		}
		return false, nil
	}
	var newMapValue protoreflect.Value
	newMapKeyExisted := reflectMap.Has(newMapKey)
	if newMapKeyExisted {
		// check map key unique
		if err := p.checkMapKeyUnique(field, cell.Data); err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		// check map key sequence
		if err := p.checkMapKeySequence(field, reflectMap, cell.Data, true); err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		newMapValue = reflectMap.Mutable(newMapKey)
	} else {
		// check map key sequence
		if err := p.checkMapKeySequence(field, reflectMap, cell.Data, false); err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		newMapValue = reflectMap.NewValue()
	}
	valuePresent, err := p.parseMessage(field, newMapValue.Message(), rc, newPrefix, newCardPrefix)
	if err != nil {
		return false, err
	}
	if !keyPresent && !valuePresent {
		// key and value are both not present.
		return false, nil
	}
	if !newMapKeyExisted {
		// check map value's sub-field prop
		dupName, err := p.checkSubFieldProp(field, cardPrefix, newMapValue)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(newPrefix+dupName)...)
		}
	}
	reflectMap.Set(newMapKey, newMapValue)
	return true, nil
}

func (p *tableParser) parseHorizontalMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	if field.fd.MapValue().Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("horizontal map value as scalar type is not supported")
	}
	if msg.Has(field.fd) {
		// When the map's layout is horizontal, skip if it was already populated.
		// It means the previous continuous present cells has been parsed.
		return true, nil
	}
	newPrefix := prefix + field.opts.Name
	detectedSize := rc.GetCellCountWithPrefix(newPrefix)
	if detectedSize <= 0 {
		if p.IsFieldOptional(field) {
			// map field not found and is optional, just return false, nil.
			return false, nil
		}
		return false, xerrors.Errorf("no cell found with digit suffix for horizontal map field")
	}
	fixedSize := fieldprop.GetSize(field.opts.Prop, detectedSize)
	size := detectedSize
	if fixedSize > 0 && fixedSize < detectedSize {
		// squeeze to specified fixed size
		size = fixedSize
	}
	checkRemainFlag := false
	// log.Debug("prefix size: ", size)
	reflectMap := msg.Mutable(field.fd).Map()
	for i := 1; i <= size; i++ {
		elemPrefix := newPrefix + strconv.Itoa(i)
		keyColName := elemPrefix + field.opts.Key
		cell, err := rc.Cell(keyColName, p.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		newMapKey, keyPresent, err := p.parseMapKey(field, reflectMap, cell.Data)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		newCardPrefix := cardPrefix + "." + escapeMapKey(newMapKey.Value())
		// value must be empty if key not present
		if !keyPresent && reflectMap.Has(newMapKey) {
			tempCheckMapValue := reflectMap.NewValue()
			valuePresent, err := p.parseMessage(field, tempCheckMapValue.Message(), rc, elemPrefix, newCardPrefix)
			if err != nil {
				return false, err
			}
			if valuePresent {
				return false, xerrors.WrapKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
			}
			break
		}
		var newMapValue protoreflect.Value
		newMapKeyExisted := reflectMap.Has(newMapKey)
		if newMapKeyExisted {
			// check map key unique
			if err := p.checkMapKeyUnique(field, cell.Data); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			// check map key sequence
			if err := p.checkMapKeySequence(field, reflectMap, cell.Data, true); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			newMapValue = reflectMap.Mutable(newMapKey)
		} else {
			// check map key sequence
			if err := p.checkMapKeySequence(field, reflectMap, cell.Data, false); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			newMapValue = reflectMap.NewValue()
		}
		valuePresent, err := p.parseMessage(field, newMapValue.Message(), rc, elemPrefix, newCardPrefix)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		if checkRemainFlag {
			// Both key and value are not present.
			// Check that no empty item is existed in between, so we should guarantee
			// that all the remaining items are not present, otherwise report error!
			if keyPresent || valuePresent {
				return false, xerrors.ErrorKV("map items are not present continuously", rc.CellDebugKV(keyColName)...)
			}
			continue
		}
		if !keyPresent && !valuePresent && !fieldprop.IsFixed(field.opts.Prop) {
			checkRemainFlag = true
			continue
		}
		if !newMapKeyExisted {
			// check map value's sub-field prop
			dupName, err := p.checkSubFieldProp(field, cardPrefix, newMapValue)
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(elemPrefix+dupName)...)
			}
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return true, nil
}

func (p *tableParser) parseIncellMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, p.IsFieldOptional(field)); err == nil {
		reflectMap := msg.Mutable(field.fd).Map()
		valueFd := field.fd.MapValue()
		if valueFd.Kind() != protoreflect.MessageKind {
			err = p.parseIncellMapWithSimpleKV(field, reflectMap, cell.Data)
		} else {
			if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
				err = xerrors.Errorf("map value type is not KV struct, and is not supported")
			} else {
				err = p.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cell.Data)
			}
		}
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	return msg.Has(field.fd), nil
}

func (p *tableParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	layout := parseTableListLayout(field.opts.GetLayout())
	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		return p.parseVerticalListField(field, msg, rc, prefix, cardPrefix)
	case tableaupb.Layout_LAYOUT_HORIZONTAL, tableaupb.Layout_LAYOUT_DEFAULT:
		// list default layout treated as horizontal
		return p.parseHorizontalListField(field, msg, rc, prefix, cardPrefix)
	case tableaupb.Layout_LAYOUT_INCELL:
		// NOTE(Wenchy): Even though named as incell, it still can merge
		// multiple vertical/horizontal cells if provided, as list is a
		// composite type with cardinality. In practice, it is a very useful.
		return p.parseIncellListField(field, msg, rc, prefix, cardPrefix)
	default:
		return false, xerrors.Errorf("unknown layout: %v", layout)
	}
}

func (p *tableParser) parseVerticalListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	if field.fd.Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("vertical list element as scalar type is not supported")
	}
	list := msg.Mutable(field.fd).List()
	elemPresent := false
	elemValue := list.NewElement()
	newPrefix := prefix + field.opts.Name
	newCardPrefix := cardPrefix + "." + strconv.Itoa(list.Len())
	// struct list
	if field.opts.Key != "" {
		// KeyedList means the list is keyed by the specified Key option.
		keyColName := newPrefix + field.opts.Key
		md := elemValue.Message().Descriptor()
		fd := p.findFieldByName(md, field.opts.Key)
		if fd == nil {
			return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", field.opts.Key), rc.CellDebugKV(keyColName)...)
		}
		cell, err := rc.Cell(keyColName, p.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		key, keyPresent, err := p.parseFieldValue(fd, cell.Data, field.opts.Prop)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		keyedListElemExisted := false
		for i := 0; i < list.Len(); i++ {
			elemVal := list.Get(i)
			if elemVal.Message().Get(fd).Equal(key) {
				elemValue = elemVal
				keyedListElemExisted = true
				break
			}
		}
		if keyedListElemExisted {
			if err := p.checkListKeyUnique(field, cell.Data); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			if err := p.checkListKeySequence(field, list, cell.Data, true); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
		} else {
			if err := p.checkListKeySequence(field, list, cell.Data, false); err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			// set as present only if key is not existed
			elemPresent = !keyedListElemExisted
		}
		// For KeyedList, use key but not len(list) as cardinality
		newCardPrefix := cardPrefix + "." + escapeMapKey(key)
		present, err := p.parseMessage(field, elemValue.Message(), rc, newPrefix, newCardPrefix)
		if err != nil {
			return false, err
		}
		if !keyPresent && !present {
			return false, nil
		}
	} else if xproto.IsUnionField(field.fd) {
		// cross-cell union list
		elemPresent, err = p.parseUnionMessage(elemValue.Message(), field, rc, newPrefix, newCardPrefix)
		if err != nil {
			return false, xerrors.Wrapf(err, "failed to parse cross-cell union list")
		}
	} else {
		// cross-cell struct list
		elemPresent, err = p.parseMessage(field, elemValue.Message(), rc, newPrefix, newCardPrefix)
		if err != nil {
			return false, xerrors.Wrapf(err, "failed to parse cross-cell struct list")
		}
	}
	if elemPresent {
		// check list elem's sub-field prop
		subFieldOptName, err := p.checkSubFieldProp(field, cardPrefix, elemValue)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(newPrefix+subFieldOptName)...)
		}
		list.Append(elemValue)
		present = true
	}
	return
}

func (p *tableParser) parseHorizontalListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	list := msg.Mutable(field.fd).List()
	if msg.Has(field.fd) {
		// When the list's layout is horizontal, skip if it was already populated.
		// It means the previous continuous present cells has been parsed.
		return true, nil
	}
	newPrefix := prefix + field.opts.Name
	detectedSize := rc.GetCellCountWithPrefix(newPrefix)
	if detectedSize <= 0 {
		if p.IsFieldOptional(field) {
			// list field not found and is optional, just return false, nil.
			return false, nil
		}
		return false, xerrors.Errorf("no cell found with digit suffix for horizontal list field")
	}
	fixedSize := fieldprop.GetSize(field.opts.Prop, detectedSize)
	size := detectedSize
	if fixedSize > 0 && fixedSize < detectedSize {
		// squeeze to specified fixed size
		size = fixedSize
	}
	var firstNonePresentIndex int
	for i := 1; i <= size; i++ {
		elemPresent := false
		elemValue := list.NewElement()
		elemPrefix := newPrefix + strconv.Itoa(i)
		newCardPrefix := cardPrefix + "." + strconv.Itoa(list.Len())
		var cell *book.RowCell
		if field.fd.Kind() == protoreflect.MessageKind {
			if types.IsWellKnownMessage(field.fd.Message().FullName()) {
				// horizontal well-known list
				if cell, err = rc.Cell(elemPrefix, p.IsFieldOptional(field)); err == nil {
					elemValue, elemPresent, err = p.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
				}
			} else if xproto.IsUnionField(field.fd) {
				// horizontal union list
				elemPresent, err = p.parseUnionMessage(elemValue.Message(), field, rc, elemPrefix, newCardPrefix)
			} else if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
				// horizontal incell-struct list
				if cell, err = rc.Cell(elemPrefix, p.IsFieldOptional(field)); err == nil {
					elemPresent, err = p.parseIncellStruct(field, elemValue, cell.Data, field.sep)
				}
			} else {
				// horizontal struct list
				elemPresent, err = p.parseMessage(field, elemValue.Message(), rc, elemPrefix, newCardPrefix)
			}
			// TODO: support horizontal KeyedList
		} else {
			// scalar list
			if cell, err = rc.Cell(elemPrefix, p.IsFieldOptional(field)); err == nil {
				elemValue, elemPresent, err = p.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
			}
		}
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(elemPrefix)...)
		}
		if firstNonePresentIndex != 0 {
			// Check that no empty elements are existed in begin or middle.
			// Guarantee all the remaining elements are not present,
			// otherwise report error!
			if elemPresent {
				return false, xerrors.WrapKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(elemPrefix)...)
			}
			continue
		}
		if !elemPresent && !fieldprop.IsFixed(field.opts.Prop) {
			firstNonePresentIndex = i
			continue
		}
		// check list elem's sub-field prop
		subFieldOptName, err := p.checkSubFieldProp(field, cardPrefix, elemValue)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(elemPrefix+subFieldOptName)...)
		}
		list.Append(elemValue)
	}
	if fieldprop.IsFixed(field.opts.Prop) {
		for list.Len() < fixedSize {
			// append empty elements to the specified length.
			list.Append(list.NewElement())
		}
	}
	return msg.Has(field.fd), nil
}

func (p *tableParser) parseIncellListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, p.IsFieldOptional(field)); err == nil {
		list := msg.Mutable(field.fd).List()
		present, err = p.parseIncellList(field, list, cardPrefix, cell.Data)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	return
}

func (p *tableParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	// NOTE(wenchy): [proto.Equal] treats a nil message as not equal to an empty one.
	// doc: [Equal](https://pkg.go.dev/google.golang.org/protobuf/proto?tab=doc#Equal)
	// issue: [APIv2: protoreflect: consider Message nilness test](https://github.com/golang/protobuf/issues/966)
	//
	//   nilMessage = (*MyMessage)(nil)
	//   emptyMessage = new(MyMessage)
	//
	//   Equal(nil, nil)                   // true
	//   Equal(nil, nilMessage)            // false
	//   Equal(nil, emptyMessage)          // false
	//   Equal(nilMessage, nilMessage)     // true
	//   Equal(nilMessage, emptyMessage)   // ??? false
	//   Equal(emptyMessage, emptyMessage) // true
	//
	// Case: `subMsg := msg.Mutable(fd).Message()`
	// `Message.Mutable` will allocate new "empty message", and is not equal to "nil"
	//
	// Solution:
	//  1. spawn two values: `emptyValue` and `structValue`
	//  2. set `structValue` back to field if `structValue` is not equal to `emptyValue`
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	var cell *book.RowCell
	newPrefix := prefix + field.opts.Name
	if types.IsWellKnownMessage(field.fd.Message().FullName()) {
		// well-known struct
		if cell, err = rc.Cell(newPrefix, p.IsFieldOptional(field)); err == nil {
			structValue, present, err = p.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
		}
	} else if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		if cell, err = rc.Cell(newPrefix, p.IsFieldOptional(field)); err == nil {
			present, err = p.parseIncellStruct(field, structValue, cell.Data, field.sep)
		}
	} else {
		// cross-cell struct
		present, err = p.parseMessage(field, structValue.Message(), rc, newPrefix, cardPrefix)
	}

	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(newPrefix)...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (p *tableParser) parseUnionField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	var cell *book.RowCell
	newPrefix := prefix + field.opts.Name
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell union
		if cell, err = rc.Cell(newPrefix, p.IsFieldOptional(field)); err == nil {
			present, err = p.parseIncellUnion(structValue, cell.Data, field.opts.GetProp().GetForm())
		}
	} else {
		// cross-cell union
		present, err = p.parseUnionMessage(structValue.Message(), field, rc, newPrefix, cardPrefix)
	}

	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(newPrefix)...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (p *tableParser) parseUnionMessage(msg protoreflect.Message, field *Field, rc *book.RowCells, prefix, cardPrefix string) (present bool, err error) {
	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeColName := prefix + strcase.FromContext(p.ctx).ToCamel(unionDesc.TypeName())
	cell, err := rc.Cell(typeColName, p.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(typeColName)...)
	}

	var typeVal protoreflect.Value
	typeVal, present, err = p.parseFieldValue(unionDesc.Type, cell.Data, nil)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(typeColName)...)
	}
	if !present {
		return false, nil
	}
	msg.Set(unionDesc.Type, typeVal)

	// parse value
	fieldNumber := int32(typeVal.Enum())
	if fieldNumber == 0 {
		// default enum value 0, no need to parse.
		return false, nil
	}
	valueFD := unionDesc.GetValueByNumber(fieldNumber)
	if valueFD == nil {
		// This enum value has not bound to a oneof field.
		return true, nil
	}
	fieldValue := msg.NewField(valueFD)
	if valueFD.Kind() != protoreflect.MessageKind {
		// just parse field 1 for scalar types.
		valColName := prefix + unionDesc.ValueFieldName() + "1"
		err := func() error {
			cell, err := rc.Cell(valColName, false)
			if err != nil {
				return err
			}
			fieldValue, present, err = p.parseFieldValue(valueFD, cell.Data, nil)
			if err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(valColName)...)
		}
	} else {
		// parse all fields for message types.
		md := valueFD.Message()
		fieldMsg := fieldValue.Message()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			valColName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
			err := func() error {
				subField := p.parseFieldDescriptor(fd)
				subField.mergeParentFieldProp(field)
				defer subField.release()
				// incell scalar
				cell, err := rc.Cell(valColName, p.IsFieldOptional(subField))
				if err != nil {
					return err
				}
				crossCellDataList := []string{cell.Data}
				if fieldCount := fieldprop.GetUnionCrossFieldCount(subField.opts.Prop); fieldCount > 0 {
					for j := 1; j < fieldCount; j++ {
						colName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number())+j)
						c, err := rc.Cell(colName, p.IsFieldOptional(subField))
						if err != nil {
							break
						}
						crossCellDataList = append(crossCellDataList, c.Data)
					}
				}
				return p.parseUnionMessageField(subField, fieldMsg, cardPrefix, crossCellDataList)
			}()
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(valColName)...)
			}
		}
	}
	msg.Set(valueFD, fieldValue)
	return
}

func (p *tableParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not populated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, p.IsFieldOptional(field)); err == nil {
		newValue, present, err = p.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	if present {
		msg.Set(field.fd, newValue)
	}
	return
}
