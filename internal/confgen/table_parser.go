package confgen

import (
	"fmt"
	"strconv"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
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

func (sp *tableParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	header := parseroptions.MergeHeader(sp.sheetOpts, sp.bookOpts, nil)
	if sp.sheetOpts.Transpose {
		// interchange the rows and columns
		// namerow: name column
		// [datarow, MaxCol]: data column
		// kvRow := make(map[string]string)
		sp.names = make([]string, sheet.Table.MaxRow)
		sp.types = make([]string, sheet.Table.MaxRow)
		nameCol := header.NameRow - 1
		typeCol := header.TypeRow - 1
		var prev *book.RowCells
		for col := header.DataRow - 1; col < sheet.Table.MaxCol; col++ {
			curr := book.NewRowCells(col, prev, sheet.Name)
			for row := 0; row < sheet.Table.MaxRow; row++ {
				if col == header.DataRow-1 {
					nameCell, err := sheet.Table.Cell(row, nameCol)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[row] = book.ExtractFromCell(nameCell, header.NameLine)

					if header.TypeRow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(row, typeCol)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[row] = book.ExtractFromCell(typeCell, header.TypeLine)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(row, &sp.names[row], &sp.types[row], data, sp.sheetOpts.AdjacentKey)
				name := sp.names[row]
				if foundRow, ok := sp.lookupTable[name]; ok && foundRow != row {
					return xerrors.E0003(name, excel.Postion(foundRow, nameCol), excel.Postion(row, nameCol))
				}
				sp.lookupTable[name] = row
			}
			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseMessage(msg, curr, "")
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
		sp.names = make([]string, sheet.Table.MaxCol)
		sp.types = make([]string, sheet.Table.MaxCol)
		nameRow := header.NameRow - 1
		typeRow := header.TypeRow - 1
		var prev *book.RowCells
		for row := header.DataRow - 1; row < sheet.Table.MaxRow; row++ {
			curr := book.NewRowCells(row, prev, sheet.Name)
			for col := 0; col < sheet.Table.MaxCol; col++ {
				if row == header.DataRow-1 {
					nameCell, err := sheet.Table.Cell(nameRow, col)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[col] = book.ExtractFromCell(nameCell, header.NameLine)

					if header.TypeRow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(typeRow, col)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[col] = book.ExtractFromCell(typeCell, header.TypeLine)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(col, &sp.names[col], &sp.types[col], data, sp.sheetOpts.AdjacentKey)
				name := sp.names[col]
				if foundCol, ok := sp.lookupTable[name]; ok && foundCol != col {
					return xerrors.E0003(name, excel.Postion(nameRow, foundCol), excel.Postion(nameRow, col))
				}
				sp.lookupTable[name] = col
			}

			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseMessage(msg, curr, "")
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
func (sp *tableParser) parseMessage(msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			field := sp.parseFieldDescriptor(fd)
			defer field.release()
			fieldPresent, err := sp.parseField(field, msg, rc, prefix)
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

func (sp *tableParser) parseField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	// log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return sp.parseMapField(field, msg, rc, prefix)
	} else if field.fd.IsList() {
		return sp.parseListField(field, msg, rc, prefix)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		if xproto.IsUnionField(field.fd) {
			return sp.parseUnionField(field, msg, rc, prefix)
		}
		return sp.parseStructField(field, msg, rc, prefix)
	} else {
		return sp.parseScalarField(field, msg, rc, prefix)
	}
}

func (sp *tableParser) parseMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	switch field.opts.GetLayout() {
	case tableaupb.Layout_LAYOUT_VERTICAL, tableaupb.Layout_LAYOUT_DEFAULT:
		// map default layout treated as virtical
		return sp.parseVerticalMapField(field, msg, rc, prefix)
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		return sp.parseHorizontalMapField(field, msg, rc, prefix)
	case tableaupb.Layout_LAYOUT_INCELL:
		// NOTE(Wenchy): Even though named as incell, it still can merge
		// multiple vertical/horizontal cells if provided, as map is a
		// composite type with cardinality. In practical use cases, this
		// is a very useful feature.
		return sp.parseIncellMapField(field, msg, rc, prefix)
	default:
		return false, xerrors.Errorf("unknown layout: %v", field.opts.GetLayout())
	}
}

func (sp *tableParser) parseVerticalMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if field.fd.MapValue().Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("vertical map value as scalar type is not supported")
	}
	keyColName := prefix + field.opts.Name + field.opts.Key
	cell, err := rc.Cell(keyColName, sp.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	reflectMap := msg.Mutable(field.fd).Map()
	newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	// value must be empty if key not present
	if !keyPresent && reflectMap.Has(newMapKey) {
		tempCheckMapValue := reflectMap.NewValue()
		valuePresent, err := sp.parseMessage(tempCheckMapValue.Message(), rc, prefix+field.opts.Name)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		if valuePresent {
			return false, xerrors.WrapKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
		}
		return false, nil
	}
	var newMapValue protoreflect.Value
	if reflectMap.Has(newMapKey) {
		md := reflectMap.NewValue().Message().Descriptor()
		keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))

		fd := md.Fields().ByName(keyProtoName)
		if fd == nil {
			return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", keyProtoName), rc.CellDebugKV(keyColName)...)
		}
		keyField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
		defer keyField.release()
		if fieldprop.RequireUnique(keyField.opts.Prop) ||
			(!fieldprop.HasUnique(keyField.opts.Prop) && sp.deduceMapKeyUnique(field, reflectMap)) {
			return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
		}
		newMapValue = reflectMap.Mutable(newMapKey)
		reflectMap.Clear(newMapKey)
	} else {
		newMapValue = reflectMap.NewValue()
	}
	valuePresent, err := sp.parseMessage(newMapValue.Message(), rc, prefix+field.opts.Name)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	if !keyPresent && !valuePresent {
		// key and value are both not present.
		return false, nil
	}
	// check uniqueness
	dupName, err := sp.checkValueUniqueInMap(field, reflectMap, newMapValue)
	if err != nil {
		keyColName := prefix + field.opts.Name + dupName
		return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
	}
	reflectMap.Set(newMapKey, newMapValue)
	return true, nil
}

func (sp *tableParser) parseHorizontalMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if field.fd.MapValue().Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("horizontal map value as scalar type is not supported")
	}
	if msg.Has(field.fd) {
		// When the map's layout is horizontal, skip if it was already populated.
		// It means the previous continuous present cells has been parsed.
		return true, nil
	}
	detectedSize := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
	if detectedSize <= 0 {
		return false, xerrors.Errorf("no cell found with digit suffix")
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
		keyColName := prefix + field.opts.Name + strconv.Itoa(i) + field.opts.Key
		cell, err := rc.Cell(keyColName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}

		newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		// value must be empty if key not present
		if !keyPresent && reflectMap.Has(newMapKey) {
			tempCheckMapValue := reflectMap.NewValue()
			valuePresent, err := sp.parseMessage(tempCheckMapValue.Message(), rc, prefix+field.opts.Name+strconv.Itoa(i))
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			if valuePresent {
				return false, xerrors.WrapKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
			}
			break
		}
		var newMapValue protoreflect.Value
		if reflectMap.Has(newMapKey) {
			md := reflectMap.NewValue().Message().Descriptor()
			keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))

			fd := md.Fields().ByName(keyProtoName)
			if fd == nil {
				return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", keyProtoName), rc.CellDebugKV(keyColName)...)
			}
			keyField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
			defer keyField.release()
			if fieldprop.RequireUnique(field.opts.Prop) ||
				(!fieldprop.HasUnique(field.opts.Prop) && sp.deduceMapKeyUnique(field, reflectMap)) {
				return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
			}
			newMapValue = reflectMap.Mutable(newMapKey)
			reflectMap.Clear(newMapKey)
		} else {
			newMapValue = reflectMap.NewValue()
		}
		valuePresent, err := sp.parseMessage(newMapValue.Message(), rc, prefix+field.opts.Name+strconv.Itoa(i))
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
		// check uniqueness
		dupName, err := sp.checkValueUniqueInMap(field, reflectMap, newMapValue)
		if err != nil {
			keyColName := prefix + field.opts.Name + strconv.Itoa(i) + dupName
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return true, nil
}

func (sp *tableParser) parseIncellMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
		reflectMap := msg.Mutable(field.fd).Map()
		valueFd := field.fd.MapValue()
		if valueFd.Kind() != protoreflect.MessageKind {
			err = sp.parseIncellMapWithSimpleKV(field, reflectMap, cell.Data)
		} else {
			if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
				err = xerrors.Errorf("map value type is not KV struct, and is not supported")
			} else {
				err = sp.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cell.Data)
			}
		}
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	return msg.Has(field.fd), nil
}

func (sp *tableParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	switch field.opts.GetLayout() {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		return sp.parseVerticalListField(field, msg, rc, prefix)
	case tableaupb.Layout_LAYOUT_HORIZONTAL, tableaupb.Layout_LAYOUT_DEFAULT:
		// list default layout treated as horizontal
		return sp.parseHorizontalListField(field, msg, rc, prefix)
	case tableaupb.Layout_LAYOUT_INCELL:
		// NOTE(Wenchy): Even though named as incell, it still can merge
		// multiple vertical/horizontal cells if provided, as list is a
		// composite type with cardinality. In practical use cases, this
		// is a very useful feature.
		return sp.parseIncellListField(field, msg, rc, prefix)
	default:
		return false, xerrors.Errorf("unknown layout: %v", field.opts.GetLayout())
	}
}

func (sp *tableParser) parseVerticalListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if field.fd.Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("vertical list element as scalar type is not supported")
	}
	list := msg.Mutable(field.fd).List()
	elemPresent := false
	elemValue := list.NewElement()
	// struct list
	if field.opts.Key != "" {
		// KeyedList means the list is keyed by the specified Key option.
		keyedListElemExisted := false
		keyColName := prefix + field.opts.Name + field.opts.Key
		md := elemValue.Message().Descriptor()
		keyProtoName := protoreflect.Name(sp.sheetParser.strcaseCtx.ToSnake(field.opts.Key))

		fd := md.Fields().ByName(keyProtoName)
		if fd == nil {
			return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", keyProtoName), rc.CellDebugKV(keyColName)...)
		}
		cell, err := rc.Cell(keyColName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		key, keyPresent, err := sp.parseFieldValue(fd, cell.Data, field.opts.Prop)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		ignoreIdx := -1
		for i := 0; i < list.Len(); i++ {
			elemVal := list.Get(i)
			if elemVal.Message().Get(fd).Equal(key) {
				elemValue = elemVal
				keyedListElemExisted = true
				ignoreIdx = i
				break
			}
		}
		keyField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
		defer keyField.release()
		if fieldprop.RequireUnique(keyField.opts.Prop) && keyedListElemExisted {
			return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
		}
		elemPresent, err = sp.parseMessage(elemValue.Message(), rc, prefix+field.opts.Name)
		if err != nil {
			return false, err
		}
		if !keyPresent && !elemPresent {
			return false, nil
		}
		if keyedListElemExisted {
			// check uniqueness
			dupName, err := sp.checkValueUniqueInList(field, list, elemValue, ignoreIdx)
			if err != nil {
				keyColName := prefix + field.opts.Name + dupName
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
		}
		elemPresent = !keyedListElemExisted
	} else if xproto.IsUnionField(field.fd) {
		// cross-cell union list
		colName := prefix + field.opts.Name
		elemPresent, err = sp.parseUnionMessage(elemValue.Message(), field, rc, colName)
		if err != nil {
			return false, xerrors.Wrapf(err, "failed to parse cross-cell union list")
		}
	} else {
		// cross-cell struct list
		elemPresent, err = sp.parseMessage(elemValue.Message(), rc, prefix+field.opts.Name)
		if err != nil {
			return false, xerrors.Wrapf(err, "failed to parse cross-cell struct list")
		}
	}
	if elemPresent {
		// check uniqueness
		dupName, err := sp.checkValueUniqueInList(field, list, elemValue, -1)
		if err != nil {
			keyColName := prefix + field.opts.Name + dupName
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
		}
		list.Append(elemValue)
		present = true
	}
	return
}

func (sp *tableParser) parseHorizontalListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	list := msg.Mutable(field.fd).List()
	if msg.Has(field.fd) {
		// When the list's layout is horizontal, skip if it was already populated.
		// It means the previous continuous present cells has been parsed.
		return true, nil
	}
	detectedSize := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
	if detectedSize <= 0 {
		return false, xerrors.Errorf("no cell found with digit suffix")
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
		colName := prefix + field.opts.Name + strconv.Itoa(i)
		var cell *book.RowCell
		if field.fd.Kind() == protoreflect.MessageKind {
			if types.IsWellKnownMessage(field.fd.Message().FullName()) {
				// horizontal well-known list
				if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
					elemValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
				}
			} else if xproto.IsUnionField(field.fd) {
				// horizontal union list
				elemPresent, err = sp.parseUnionMessage(elemValue.Message(), field, rc, colName)
			} else if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
				// horizontal incell-struct list
				if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
					elemPresent, err = sp.parseIncellStruct(elemValue, cell.Data, field.opts.GetProp().GetForm(), field.sep)
				}
			} else {
				// horizontal struct list
				elemPresent, err = sp.parseMessage(elemValue.Message(), rc, colName)
			}
		} else {
			// scalar list
			if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
				elemValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
			}
		}
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		if firstNonePresentIndex != 0 {
			// Check that no empty elements are existed in begin or middle.
			// Guarantee all the remaining elements are not present,
			// otherwise report error!
			if elemPresent {
				return false, xerrors.WrapKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(colName)...)
			}
			continue
		}
		if !elemPresent && !fieldprop.IsFixed(field.opts.Prop) {
			firstNonePresentIndex = i
			continue
		}
		// check uniqueness
		dupName, err := sp.checkValueUniqueInList(field, list, elemValue, -1)
		if err != nil {
			keyColName := colName + dupName
			return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
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

func (sp *tableParser) parseIncellListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
		list := msg.Mutable(field.fd).List()
		present, err = sp.parseIncellList(field, list, cell.Data)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	return
}

func (sp *tableParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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
	colName := prefix + field.opts.Name
	if types.IsWellKnownMessage(field.fd.Message().FullName()) {
		// well-known struct
		if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
			structValue, present, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
		}
	} else if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
			present, err = sp.parseIncellStruct(structValue, cell.Data, field.opts.GetProp().GetForm(), field.sep)
		}
	} else {
		// cross-cell struct
		present, err = sp.parseMessage(structValue.Message(), rc, prefix+field.opts.Name)
	}

	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (sp *tableParser) parseUnionField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell union
		if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
			present, err = sp.parseIncellUnion(structValue, cell.Data, field.opts.GetProp().GetForm())
		}
	} else {
		// cross-cell union
		present, err = sp.parseUnionMessage(structValue.Message(), field, rc, colName)
	}

	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (sp *tableParser) parseUnionMessage(msg protoreflect.Message, field *Field, rc *book.RowCells, prefix string) (present bool, err error) {
	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeColName := prefix + sp.strcaseCtx.ToCamel(unionDesc.TypeName())
	cell, err := rc.Cell(typeColName, sp.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(typeColName)...)
	}

	var typeVal protoreflect.Value
	typeVal, present, err = sp.parseFieldValue(unionDesc.Type, cell.Data, nil)
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
		return false, xerrors.Errorf("union value (oneof) as scalar type not supported: %s", valueFD.FullName())
	}
	// MUST be message type.
	md := valueFD.Message()
	fieldMsg := fieldValue.Message()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		valColName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
		err := func() error {
			subField := sp.parseFieldDescriptor(fd)
			defer subField.release()
			// incell scalar
			cell, err := rc.Cell(valColName, sp.IsFieldOptional(subField))
			if err != nil {
				return err
			}
			crossCellDataList := []string{cell.Data}
			if fieldCount := fieldprop.GetUnionCrossFieldCount(subField.opts.Prop); fieldCount > 0 {
				for j := 1; j < fieldCount; j++ {
					colName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number())+j)
					c, err := rc.Cell(colName, sp.IsFieldOptional(subField))
					if err != nil {
						break
					}
					crossCellDataList = append(crossCellDataList, c.Data)
				}
			}
			return sp.parseUnionMessageField(subField, fieldMsg, crossCellDataList)
		}()
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(valColName)...)
		}
	}
	msg.Set(valueFD, fieldValue)
	return
}

func (sp *tableParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not populated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
	var cell *book.RowCell
	colName := prefix + field.opts.Name
	if cell, err = rc.Cell(colName, sp.IsFieldOptional(field)); err == nil {
		newValue, present, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	if present {
		msg.Set(field.fd, newValue)
	}
	return
}
