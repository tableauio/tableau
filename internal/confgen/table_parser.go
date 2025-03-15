package confgen

import (
	"fmt"
	"strconv"

	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer/book"
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

func (sp *tableParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	if sp.opts.Transpose {
		// interchange the rows and columns
		// namerow: name column
		// [datarow, MaxCol]: data column
		// kvRow := make(map[string]string)
		sp.names = make([]string, sheet.Table.MaxRow)
		sp.types = make([]string, sheet.Table.MaxRow)
		nameCol := int(sp.opts.Namerow) - 1
		typeCol := int(sp.opts.Typerow) - 1
		var prev *book.RowCells
		for col := int(sp.opts.Datarow) - 1; col < sheet.Table.MaxCol; col++ {
			curr := book.NewRowCells(col, prev, sheet.Name)
			for row := 0; row < sheet.Table.MaxRow; row++ {
				if col == int(sp.opts.Datarow)-1 {
					nameCell, err := sheet.Table.Cell(row, nameCol)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[row] = book.ExtractFromCell(nameCell, sp.opts.Nameline)

					if sp.opts.Typerow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(row, typeCol)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[row] = book.ExtractFromCell(typeCell, sp.opts.Typeline)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(row, &sp.names[row], &sp.types[row], data, sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[row]] = uint32(row)
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
		nameRow := int(sp.opts.Namerow) - 1
		typeRow := int(sp.opts.Typerow) - 1
		var prev *book.RowCells
		for row := int(sp.opts.Datarow) - 1; row < sheet.Table.MaxRow; row++ {
			curr := book.NewRowCells(row, prev, sheet.Name)
			for col := 0; col < sheet.Table.MaxCol; col++ {
				if row == int(sp.opts.Datarow)-1 {
					nameCell, err := sheet.Table.Cell(nameRow, col)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[col] = book.ExtractFromCell(nameCell, sp.opts.Nameline)

					if sp.opts.Typerow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Table.Cell(typeRow, col)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[col] = book.ExtractFromCell(typeCell, sp.opts.Typeline)
					}
				}

				data, err := sheet.Table.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.NewCell(col, &sp.names[col], &sp.types[col], data, sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[col]] = uint32(col)
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
			field := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
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
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectMap := newValue.Map()
	// reflectMap := msg.Mutable(field.fd).Map()
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()

	layout := field.opts.Layout
	if field.opts.Layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// Map default layout is vertical
		layout = tableaupb.Layout_LAYOUT_VERTICAL
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		if valueFd.Kind() == protoreflect.MessageKind {
			keyColName := prefix + field.opts.Name + field.opts.Key
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
				valuePresent, err := sp.parseMessage(tempCheckMapValue.Message(), rc, prefix+field.opts.Name)
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
				newMapValue = reflectMap.Mutable(newMapKey)
			} else {
				newMapValue = reflectMap.NewValue()
			}
			valuePresent, err := sp.parseMessage(newMapValue.Message(), rc, prefix+field.opts.Name)
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
			}
			// check key uniqueness
			if reflectMap.Has(newMapKey) {
				if prop.RequireUnique(field.opts.Prop) ||
					(!prop.HasUnique(field.opts.Prop) && sp.deduceMapKeyUnique(field, reflectMap)) {
					return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
				}
			}
			if !keyPresent && !valuePresent {
				// key and value are both not present.
				break
			}
			reflectMap.Set(newMapKey, newMapValue)
		} else {
			return false, xerrors.Errorf("vertical map value scalar type is not supported")
		}

	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		if valueFd.Kind() == protoreflect.MessageKind {
			if msg.Has(field.fd) {
				// When the map's layout is horizontal, skip if it was already present.
				// This means the front continuous present cells (related to this map)
				// has already been parsed.
				break
			}
			detectedSize := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
			if detectedSize <= 0 {
				return false, xerrors.Errorf("no cell found with digit suffix")
			}
			fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
			size := detectedSize
			if fixedSize > 0 && fixedSize < detectedSize {
				// squeeze to specified fixed size
				size = fixedSize
			}
			checkRemainFlag := false
			// log.Debug("prefix size: ", size)
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
					newMapValue = reflectMap.Mutable(newMapKey)
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
				if !keyPresent && !valuePresent && !prop.IsFixed(field.opts.Prop) {
					checkRemainFlag = true
					continue
				}
				// check key uniqueness
				if reflectMap.Has(newMapKey) {
					if prop.RequireUnique(field.opts.Prop) ||
						(!prop.HasUnique(field.opts.Prop) && sp.deduceMapKeyUnique(field, reflectMap)) {
						return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
					}
				}
				reflectMap.Set(newMapKey, newMapValue)
			}
		} else {
			return false, xerrors.Errorf("horizontal map value scalar type is not supported")
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		colName := prefix + field.opts.Name
		cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		err = sp.parseIncellMap(field, reflectMap, cell.Data)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
	}

	if msg.Has(field.fd) {
		present = true
	}
	return present, nil
}

func (sp *tableParser) parseIncellMap(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	if valueFd.Kind() != protoreflect.MessageKind {
		return sp.parseIncellMapWithSimpleKV(field, reflectMap, cellData)
	}

	if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
		return xerrors.Errorf("map value type is not KV struct, and is not supported")
	}
	return sp.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cellData)
}

func (sp *tableParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	list := newValue.List()

	layout := field.opts.Layout
	if field.opts.Layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// List default layout is horizontal
		layout = tableaupb.Layout_LAYOUT_HORIZONTAL
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		// vertical list
		if field.fd.Kind() == protoreflect.MessageKind {
			// struct list
			if field.opts.Key != "" {
				// KeyedList means the list is keyed by the specified Key option.
				listElemValue := list.NewElement()
				keyedListElemExisted := false
				keyColName := prefix + field.opts.Name + field.opts.Key
				md := listElemValue.Message().Descriptor()
				keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))

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
				for i := 0; i < list.Len(); i++ {
					elemVal := list.Get(i)
					if elemVal.Message().Get(fd).Equal(key) {
						listElemValue = elemVal
						keyedListElemExisted = true
						break
					}
				}
				elemPresent, err := sp.parseMessage(listElemValue.Message(), rc, prefix+field.opts.Name)
				if err != nil {
					return false, xerrors.WrapKV(err, rc.CellDebugKV(keyColName)...)
				}
				if !keyPresent && !elemPresent {
					break
				}
				if !keyedListElemExisted {
					list.Append(listElemValue)
				}
			} else if xproto.IsUnionField(field.fd) {
				elemPresent := false
				newListValue := list.NewElement()
				colName := prefix + field.opts.Name
				// cross-cell union list
				elemPresent, err = sp.parseUnionMessage(newListValue.Message(), field, rc, colName)
				if err != nil {
					return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
				}
				if elemPresent {
					list.Append(newListValue)
				}
			} else {
				elemPresent := false
				newListValue := list.NewElement()
				// cross-cell struct list
				elemPresent, err = sp.parseMessage(newListValue.Message(), rc, prefix+field.opts.Name)
				if err != nil {
					return false, xerrors.WrapKV(err, "cross-cell struct list", "failed to parse struct")
				}
				if elemPresent {
					list.Append(newListValue)
				}
			}
		} else {
			return false, xerrors.Errorf("vertical list value scalar type is not supported")
		}
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		// horizontal list
		if msg.Has(field.fd) {
			// When the list's layout is horizontal, skip if it was already present.
			// This means the front continuous present cells (related to this list)
			// has already been parsed.
			break
		}
		detectedSize := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
		if detectedSize <= 0 {
			return false, xerrors.Errorf("no cell found with digit suffix")
		}
		fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
		size := detectedSize
		if fixedSize > 0 && fixedSize < detectedSize {
			// squeeze to specified fixed size
			size = fixedSize
		}
		var firstNonePresentIndex int
		for i := 1; i <= size; i++ {
			newListValue := list.NewElement()
			colName := prefix + field.opts.Name + strconv.Itoa(i)
			elemPresent := false
			if field.fd.Kind() == protoreflect.MessageKind {
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// horizontal incell-struct list
					cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
					if err != nil {
						return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
					}
					subMsgName := string(field.fd.Message().FullName())
					if types.IsWellKnownMessage(subMsgName) {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
						if err != nil {
							return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
						}
					} else {
						if elemPresent, err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.GetProp().GetForm(), field.sep); err != nil {
							return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
						}
					}
				} else if xproto.IsUnionField(field.fd) {
					// horizontal union list
					elemPresent, err = sp.parseUnionMessage(newListValue.Message(), field, rc, colName)
					if err != nil {
						return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
					}
				} else {
					// horizontal struct list
					subMsgName := string(field.fd.Message().FullName())
					if types.IsWellKnownMessage(subMsgName) {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WrapKV(err, kvs...)
						}
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WrapKV(err, kvs...)
						}
					} else {
						elemPresent, err = sp.parseMessage(newListValue.Message(), rc, colName)
						if err != nil {
							return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
						}
					}
				}
				if firstNonePresentIndex != 0 {
					// Check that no empty element is existed in between, so we should guarantee
					// that all the remaining elements are not present, otherwise report error!
					if elemPresent {
						return false, xerrors.WrapKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(colName)...)
					}
					continue
				}
				if !elemPresent && !prop.IsFixed(field.opts.Prop) {
					firstNonePresentIndex = i
					continue
				}
				list.Append(newListValue)
			} else {
				// scalar list
				cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
				if err != nil {
					return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
				}
				newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
				if err != nil {
					return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
				}
				if firstNonePresentIndex != 0 {
					// check the remaining scalar elements are not present, otherwise report error!
					if elemPresent {
						return false, xerrors.WrapKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(colName)...)
					}
					continue
				}
				if !elemPresent && !prop.IsFixed(field.opts.Prop) {
					firstNonePresentIndex = i
					continue
				}
				list.Append(newListValue)
			}
		}

		if prop.IsFixed(field.opts.Prop) {
			for list.Len() < fixedSize {
				// append empty elements to the specified length.
				list.Append(list.NewElement())
			}
		}
	case tableaupb.Layout_LAYOUT_INCELL:
		// incell list
		colName := prefix + field.opts.Name
		cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		present, err = sp.parseIncellListField(field, list, cell.Data)
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
	}
	if msg.Has(field.fd) {
		present = true
	}
	return present, nil
}

func (sp *tableParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	// NOTE(wenchy): `proto.Equal` treats a nil message as not equal to an empty one.
	// doc: [Equal](https://pkg.go.dev/google.golang.org/protobuf/proto?tab=doc#Equal)
	// issue: [APIv2: protoreflect: consider Message nilness test](https://github.com/golang/protobuf/issues/966)
	// ```
	// nilMessage = (*MyMessage)(nil)
	// emptyMessage = new(MyMessage)
	//
	// Equal(nil, nil)                   // true
	// Equal(nil, nilMessage)            // false
	// Equal(nil, emptyMessage)          // false
	// Equal(nilMessage, nilMessage)     // true
	// Equal(nilMessage, emptyMessage)   // ??? false
	// Equal(emptyMessage, emptyMessage) // true
	// ```
	//
	// Case: `subMsg := msg.Mutable(fd).Message()`
	// `Message.Mutable` will allocate new "empty message", and is not equal to "nil"
	//
	// Solution:
	// 1. spawn two values: `emptyValue` and `structValue`
	// 2. set `structValue` back to field if `structValue` is not equal to `emptyValue`
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	colName := prefix + field.opts.Name
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}

		if present, err = sp.parseIncellStruct(structValue, cell.Data, field.opts.GetProp().GetForm(), field.sep); err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	} else {
		subMsgName := string(field.fd.Message().FullName())
		if types.IsWellKnownMessage(subMsgName) {
			// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
			cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
			}
			value, present, err := sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
			}
			if present {
				msg.Set(field.fd, value)
			}
			return present, nil
		} else {
			present, err := sp.parseMessage(structValue.Message(), rc, prefix+field.opts.Name)
			if err != nil {
				return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
			}
			if present {
				// only set field if it is present.
				msg.Set(field.fd, structValue)
			}
			return present, nil
		}
	}
}

func (sp *tableParser) parseUnionField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		colName := prefix + field.opts.Name
		// incell union
		cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
		if err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		if present, err = sp.parseIncellUnion(structValue, cell.Data, field.opts.GetProp().GetForm()); err != nil {
			return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}

	present, err = sp.parseUnionMessage(structValue.Message(), field, rc, prefix+field.opts.Name)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(prefix+field.opts.Name)...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return present, nil
}

func (sp *tableParser) parseUnionMessage(msg protoreflect.Message, field *Field, rc *book.RowCells, prefix string) (present bool, err error) {
	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeColName := prefix + unionDesc.TypeName()
	cell, err := rc.Cell(typeColName, sp.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(typeColName)...)
	}

	var typeVal protoreflect.Value
	typeVal, present, err = sp.parseFieldValue(unionDesc.Type, cell.Data, nil)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(typeColName)...)
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
		typeValue := unionDesc.Type.Enum().Values().ByNumber(protoreflect.EnumNumber(fieldNumber)).Name()
		return false, xerrors.WrapKV(xerrors.E2010(typeValue, fieldNumber), rc.CellDebugKV(prefix)...)
	}
	fieldValue := msg.NewField(valueFD)
	if valueFD.Kind() == protoreflect.MessageKind {
		// MUST be message type.
		md := valueFD.Message()
		msg := fieldValue.Message()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			valColName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
			err := func() error {
				subField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
				defer subField.release()
				// incell scalar
				cell, err := rc.Cell(valColName, sp.IsFieldOptional(subField))
				if err != nil {
					return xerrors.WrapKV(err, rc.CellDebugKV(valColName)...)
				}
				var crossCellDataList []string
				if fieldCount := prop.GetUnionCrossFieldCount(subField.opts.Prop); fieldCount > 0 {
					crossCellDataList = []string{cell.Data}
					for j := 1; j < fieldCount; j++ {
						colName := prefix + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number())+j)
						c, err := rc.Cell(colName, sp.IsFieldOptional(subField))
						if err != nil {
							break
						}
						crossCellDataList = append(crossCellDataList, c.Data)
					}
				}
				err = sp.parseUnionMessageField(subField, msg, cell.Data, crossCellDataList)
				if err != nil {
					return xerrors.WrapKV(err, rc.CellDebugKV(valColName)...)
				}
				return nil
			}()
			if err != nil {
				return false, err
			}
		}
	} else {
		// scalar: not supported yet.
		return false, xerrors.Errorf("union value (oneof) as scalar type not supported: %s", valueFD.FullName())
	}
	msg.Set(valueFD, fieldValue)
	return present, nil
}

func (sp *tableParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not populated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
	colName := prefix + field.opts.Name
	cell, err := rc.Cell(colName, sp.IsFieldOptional(field))
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}

	newValue, present, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
	if err != nil {
		return false, xerrors.WrapKV(err, rc.CellDebugKV(colName)...)
	}
	if !present {
		return false, nil
	}
	msg.Set(field.fd, newValue)
	return true, nil
}
