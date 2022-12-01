package confgen

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/confgen/mexporter"
	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

type sheetExporter struct {
	OutputDir string
	OutputOpt *options.ConfOutputOption // output settings.

}

func NewSheetExporter(outputDir string, output *options.ConfOutputOption) *sheetExporter {
	return &sheetExporter{
		OutputDir: outputDir,
		OutputOpt: output,
	}
}

// parse and export the protomsg message.
func (x *sheetExporter) Export(info *sheetInfo, md protoreflect.MessageDescriptor, importers ...importer.Importer) error {
	protomsg, err := ParseMessage(info, md, importers...)
	if err != nil {
		return err
	}
	exporter := mexporter.New(info.MessageName, protomsg, x.OutputDir, x.OutputOpt, info.opts)
	if err := exporter.Export(); err != nil {
		return err
	}
	return nil
}

func ParseMessage(info *sheetInfo, md protoreflect.MessageDescriptor, importers ...importer.Importer) (proto.Message, error) {
	if len(importers) == 1 {
		return parseMessageFromOneImporter(info, md, importers[0])
	} else if len(importers) == 0 {
		return nil, xerrors.ErrorKV("no protomsg parsed", xerrors.KeySheetName, info.SheetName, xerrors.KeyPBMessage, info.MessageName)
	}

	// NOTE: use map-reduce pattern to accelerate parsing multiple importers.
	// - check: first field must be map or list
	// - errgroup
	// - proto.Merge

	var mu sync.Mutex // guard msgs
	var msgs []proto.Message

	var eg errgroup.Group
	for _, imp := range importers {
		imp := imp
		// map-reduce: map jobs for concurrent processing
		eg.Go(func() error {
			protomsg, err := parseMessageFromOneImporter(info, md, imp)
			if err != nil {
				return err
			}
			mu.Lock()
			msgs = append(msgs, protomsg)
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// map-reduce: reduce results to one
	mainMsg := msgs[0] // treat the first one as main msg
	for i := 1; i < len(msgs); i++ {
		// WARNIG: panic: proto: google.protobuf.Timestamp.seconds: field descriptor does not belong to this message
		proto.Merge(mainMsg, msgs[i])
	}
	return mainMsg, nil
}

func parseMessageFromOneImporter(info *sheetInfo, md protoreflect.MessageDescriptor, imp importer.Importer) (proto.Message, error) {
	sheet := imp.GetSheet(info.SheetName)
	if sheet == nil {
		err := xerrors.E0001(info.SheetName, imp.Filename())
		return nil, xerrors.WithMessageKV(err, xerrors.KeySheetName, info.SheetName, xerrors.KeyPBMessage, info.MessageName)
	}
	parser := NewSheetParserWithGen(info.gen, info.opts)
	protomsg := dynamicpb.NewMessage(md)
	if err := parser.Parse(protomsg, sheet); err != nil {
		return nil, xerrors.WithMessageKV(err, xerrors.KeySheetName, info.SheetName, xerrors.KeyPBMessage, info.MessageName)
	}
	return protomsg, nil
}

type sheetParser struct {
	ProtoPackage string
	LocationName string

	gen  *Generator // nil if this is a simple parser
	opts *tableaupb.WorksheetOptions

	// cached name and type
	names       []string               // names[col] -> name
	types       []string               // types[col] -> name
	lookupTable book.ColumnLookupTable // name -> column index (started with 0)
}

func NewSheetParserWithGen(gen *Generator, opts *tableaupb.WorksheetOptions) *sheetParser {
	return &sheetParser{
		ProtoPackage: gen.ProtoPackage,
		LocationName: gen.LocationName,
		gen:          gen,
		opts:         opts,
		lookupTable:  map[string]uint32{},
	}
}

func NewSheetParser(protoPackage, locationName string, opts *tableaupb.WorksheetOptions) *sheetParser {
	return &sheetParser{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		gen:          nil,
		opts:         opts,
		lookupTable:  map[string]uint32{},
	}
}

func (sp *sheetParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	if sp.opts.Transpose {
		// interchange the rows and columns
		// namerow: name column
		// [datarow, MaxCol]: data column
		// kvRow := make(map[string]string)
		sp.names = make([]string, sheet.MaxRow)
		sp.types = make([]string, sheet.MaxRow)
		nameCol := int(sp.opts.Namerow) - 1
		typeCol := int(sp.opts.Typerow) - 1
		var prev *book.RowCells
		for col := int(sp.opts.Datarow) - 1; col < sheet.MaxCol; col++ {
			curr := book.NewRowCells(col, prev, sheet.Name)
			for row := 0; row < sheet.MaxRow; row++ {
				if col == int(sp.opts.Datarow)-1 {
					nameCell, err := sheet.Cell(row, nameCol)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[row] = book.ExtractFromCell(nameCell, sp.opts.Nameline)

					if sp.opts.Typerow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Cell(row, typeCol)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[row] = book.ExtractFromCell(typeCell, sp.opts.Typeline)
					}
				}

				data, err := sheet.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}

				curr.SetCell(&sp.names[row], row, data, &sp.types[row], sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[row]] = uint32(row)
			}
			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseFieldOptions(msg, curr, 0, "")
			if err != nil {
				return err
			}

			prev = curr
		}
	} else {
		// namerow: name row
		// [datarow, MaxRow]: data row
		sp.names = make([]string, sheet.MaxCol)
		sp.types = make([]string, sheet.MaxCol)
		nameRow := int(sp.opts.Namerow) - 1
		typeRow := int(sp.opts.Typerow) - 1
		var prev *book.RowCells
		for row := int(sp.opts.Datarow) - 1; row < sheet.MaxRow; row++ {
			curr := book.NewRowCells(row, prev, sheet.Name)
			for col := 0; col < sheet.MaxCol; col++ {
				if row == int(sp.opts.Datarow)-1 {
					nameCell, err := sheet.Cell(nameRow, col)
					if err != nil {
						return xerrors.WrapKV(err)
					}
					sp.names[col] = book.ExtractFromCell(nameCell, sp.opts.Nameline)

					if sp.opts.Typerow > 0 {
						// if typerow is set!
						typeCell, err := sheet.Cell(typeRow, col)
						if err != nil {
							return xerrors.WrapKV(err)
						}
						sp.types[col] = book.ExtractFromCell(typeCell, sp.opts.Typeline)
					}
				}

				data, err := sheet.Cell(row, col)
				if err != nil {
					return xerrors.WrapKV(err)
				}
				curr.SetCell(&sp.names[col], col, data, &sp.types[col], sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[col]] = uint32(col)
			}

			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseFieldOptions(msg, curr, 0, "")
			if err != nil {
				return err
			}

			prev = curr
		}
	}
	return nil
}

type Field struct {
	fd   protoreflect.FieldDescriptor
	opts *tableaupb.FieldOptions
}

// parseFieldOptions is aimed to parse the options of all the fields of a protobuf message.
func (sp *sheetParser) parseFieldOptions(msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	md := msg.Descriptor()
	pkg := md.ParentFile().Package()
	// opts := md.Options().(*descriptorpb.MessageOptions)
	// worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	// worksheetName := ""
	// if worksheet != nil {
	// 	worksheetName = worksheet.Name
	// }
	// log.Debugf("%s// %s, '%s', %v, %v, %v", printer.Indent(depth), md.FullName(), worksheetName, md.IsMapEntry(), prefix, pkg)
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if string(pkg) != sp.ProtoPackage && pkg != "google.protobuf" {
			log.Debugf("no need to process package: %v", pkg)
			return false, nil
		}

		// default value
		name := strcase.ToCamel(string(fd.FullName().Name()))
		note := ""
		span := tableaupb.Span_SPAN_DEFAULT
		key := ""
		layout := tableaupb.Layout_LAYOUT_DEFAULT
		sep := ","
		subsep := ":"
		optional := false
		var prop *tableaupb.FieldProp

		opts := fd.Options().(*descriptorpb.FieldOptions)
		fieldOpts := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
		if fieldOpts != nil {
			name = fieldOpts.Name
			note = fieldOpts.Note
			span = fieldOpts.Span
			key = fieldOpts.Key
			layout = fieldOpts.Layout
			sep = strings.TrimSpace(fieldOpts.Sep)
			subsep = strings.TrimSpace(fieldOpts.Subsep)
			optional = fieldOpts.Optional
			prop = fieldOpts.Prop
		} else {
			// default processing
			if fd.IsList() {
				// truncate suffix `List` (CamelCase) corresponding to `_list` (snake_case)
				name = strings.TrimSuffix(name, "List")
			} else if fd.IsMap() {
				// truncate suffix `Map` (CamelCase) corresponding to `_map` (snake_case)
				// name = strings.TrimSuffix(name, "Map")
				name = ""
				key = "Key"
			}
		}
		if sep == "" {
			sep = strings.TrimSpace(sp.opts.Sep)
			if sep == "" {
				sep = ","
			}
		}
		if subsep == "" {
			subsep = strings.TrimSpace(sp.opts.Subsep)
			if subsep == "" {
				subsep = ":"
			}
		}

		// get from pool
		pooledFDOpts := fieldOptionsPool.Get().(*tableaupb.FieldOptions)
		pooledFDOpts.Name = name
		pooledFDOpts.Note = note
		pooledFDOpts.Span = span
		pooledFDOpts.Key = key
		pooledFDOpts.Layout = layout
		pooledFDOpts.Sep = sep
		pooledFDOpts.Subsep = subsep
		pooledFDOpts.Optional = optional
		pooledFDOpts.Prop = prop

		field := &Field{
			fd:   fd,
			opts: pooledFDOpts,
		}
		fieldPresent, err := sp.parseField(field, msg, rc, depth, prefix)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldName, fd.FullName().Name(), xerrors.KeyPBFieldOpts, field.opts)
		}
		if fieldPresent {
			// The message is treated as present only if one field is present.
			present = true
		}
		// return back to pool
		fieldOptionsPool.Put(pooledFDOpts)
	}
	return present, nil
}

func (sp *sheetParser) parseField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	// log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return sp.parseMapField(field, msg, rc, depth, prefix)
	} else if field.fd.IsList() {
		return sp.parseListField(field, msg, rc, depth, prefix)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		return sp.parseStructField(field, msg, rc, depth, prefix)
	} else {
		return sp.parseScalarField(field, msg, rc, depth, prefix)
	}
}

func (sp *sheetParser) parseMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectMap := newValue.Map()
	// reflectMap := msg.Mutable(field.fd).Map()
	keyFd := field.fd.MapKey()
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
			cell, err := rc.Cell(keyColName, field.opts.Optional)
			if err != nil {
				kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
			if err != nil {
				kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			var newMapValue protoreflect.Value
			if reflectMap.Has(newMapKey) {
				// check key uniqueness
				if err := prop.CheckKeyUnique(field.opts.Prop, cell.Data, true); err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical map")
					return false, xerrors.WrapKV(err, kvs...)
				}
				newMapValue = reflectMap.Mutable(newMapKey)
			} else {
				newMapValue = reflectMap.NewValue()
			}
			valuePresent, err := sp.parseFieldOptions(newMapValue.Message(), rc, depth+1, prefix+field.opts.Name)
			if err != nil {
				kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if !keyPresent && !valuePresent {
				// key and value are both not present.
				break
			}
			reflectMap.Set(newMapKey, newMapValue)
		} else {
			// value is scalar type
			key := "Key"     // default key name
			value := "Value" // default value name
			// key cell
			keyColName := prefix + field.opts.Name + key
			cell, err := rc.Cell(keyColName, field.opts.Optional)
			if err != nil {
				kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical scalar map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}

			fieldValue, keyPresent, err := sp.parseFieldValue(keyFd, cell.Data)
			if err != nil {
				kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical scalar map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}

			newMapKey := fieldValue.MapKey()
			if reflectMap.Has(newMapKey) {
				// check key uniqueness
				if err := prop.CheckKeyUnique(field.opts.Prop, cell.Data, true); err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical scalar map")
					return false, xerrors.WrapKV(err, kvs...)
				}
			}
			// value cell
			valueColName := prefix + field.opts.Name + value
			cell, err = rc.Cell(valueColName, field.opts.Optional)
			if err != nil {
				kvs := append(rc.CellDebugKV(valueColName), xerrors.KeyPBFieldType, "vertical scalar map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}

			newMapValue, valuePresent, err := sp.parseFieldValue(field.fd, cell.Data)
			if err != nil {
				kvs := append(rc.CellDebugKV(valueColName), xerrors.KeyPBFieldType, "vertical scalar map")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if !keyPresent && !valuePresent {
				// key and value are both not present.
				break
			}
			if !reflectMap.Has(newMapKey) {
				reflectMap.Set(newMapKey, newMapValue)
			}
		}

	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		if valueFd.Kind() == protoreflect.MessageKind {
			if msg.Has(field.fd) {
				// When the map's layout is horizontal, skip if it was already present.
				// This means the front continuous present cells (related to this list)
				// has already been parsed.
				break
			}
			size := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
			// log.Debug("prefix size: ", size)
			for i := 1; i <= size; i++ {
				keyColName := prefix + field.opts.Name + strconv.Itoa(i) + field.opts.Key
				cell, err := rc.Cell(keyColName, field.opts.Optional)
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "horizontal map")
					return false, xerrors.WithMessageKV(err, kvs...)
				}

				newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "horizontal map")
					return false, xerrors.WithMessageKV(err, kvs...)
				}

				var newMapValue protoreflect.Value
				if reflectMap.Has(newMapKey) {
					// check key uniqueness
					if err := prop.CheckKeyUnique(field.opts.Prop, cell.Data, true); err != nil {
						kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "horizontal map")
						return false, xerrors.WrapKV(err, kvs...)
					}
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				valuePresent, err := sp.parseFieldOptions(newMapValue.Message(), rc, depth+1, prefix+field.opts.Name+strconv.Itoa(i))
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "horizontal map")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				if !keyPresent && !valuePresent {
					// key and value are both not present.
					// TODO: check the remaining keys all not present, otherwise report error!
					break
				}
				reflectMap.Set(newMapKey, newMapValue)
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		colName := prefix + field.opts.Name
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell map")
			return false, xerrors.WithMessageKV(err, kvs...)
		}

		if valueFd.Kind() == protoreflect.MessageKind {
			kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell map")
			return false, xerrors.ErrorKV("map value type is struct, and is not supported", kvs...)
		}

		if cell.Data != "" {
			// If s does not contain sep and sep is not empty, Split returns a
			// slice of length 1 whose only element is s.
			splits := strings.Split(cell.Data, field.opts.Sep)
			size := len(splits)
			for i := 0; i < size; i++ {
				kv := strings.SplitN(splits[i], field.opts.Subsep, 2)
				if len(kv) == 1 {
					// If value is not set, then treated it as default empty string.
					kv = append(kv, "")
				}
				key, value := kv[0], kv[1]

				fieldValue, keyPresent, err := sp.parseFieldValue(keyFd, key)
				if err != nil {
					kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell map")
					return false, xerrors.WithMessageKV(err, kvs...)
				}

				newMapKey := fieldValue.MapKey()
				fieldValue, valuePresent, err := sp.parseFieldValue(valueFd, value)
				if err != nil {
					kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell map")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				newMapValue := reflectMap.NewValue()
				newMapValue = fieldValue

				if !keyPresent && !valuePresent {
					// key and value are both not present.
					// TODO: check the remaining keys all not present, otherwise report error!
					break
				}

				reflectMap.Set(newMapKey, newMapValue)
			}
		}
	}

	if !msg.Has(field.fd) && reflectMap.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	if msg.Has(field.fd) || reflectMap.Len() != 0 {
		present = true
	}
	return present, nil
}

func (sp *sheetParser) parseMapKey(field *Field, reflectMap protoreflect.Map, cellData string) (mapKey protoreflect.MapKey, present bool, err error) {
	var keyFd protoreflect.FieldDescriptor

	md := reflectMap.NewValue().Message().Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		fdOpts := fd.Options().(*descriptorpb.FieldOptions)
		if fdOpts != nil {
			tableauFieldOpts := proto.GetExtension(fdOpts, tableaupb.E_Field).(*tableaupb.FieldOptions)
			if tableauFieldOpts != nil && tableauFieldOpts.Name == field.opts.Key {
				keyFd = fd
				break
			}
		}
	}
	if keyFd == nil {
		return mapKey, false, xerrors.Errorf("opts.Key %s not found in map value-type definition", field.opts.Key)
	}
	var fieldValue protoreflect.Value
	if keyFd.Kind() == protoreflect.EnumKind {
		fieldValue, present, err = sp.parseFieldValue(keyFd, cellData)
		if err != nil {
			return mapKey, false, err
		}
		v := protoreflect.ValueOfInt32(int32(fieldValue.Enum()))
		mapKey = v.MapKey()
	} else {
		fieldValue, present, err = sp.parseFieldValue(keyFd, cellData)
		if err != nil {
			return mapKey, false, xerrors.WrapKV(err)
		}
		mapKey = fieldValue.MapKey()
	}
	if !prop.CheckMapKeySequence(field.opts.Prop, keyFd.Kind(), mapKey, reflectMap) {
		return mapKey, false, xerrors.E2003(cellData, field.opts.Prop.GetSequence())
	}
	return mapKey, present, nil
}

func (sp *sheetParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectList := newValue.List()

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
				listItemValue := reflectList.NewElement()
				keyedListItemExisted := false
				keyColName := prefix + field.opts.Name + field.opts.Key
				md := listItemValue.Message().Descriptor()
				keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))

				fd := md.Fields().ByName(keyProtoName)
				if fd == nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical keyed list")
					return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", keyProtoName), kvs...)
				}
				cell, err := rc.Cell(keyColName, field.opts.Optional)
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical keyed list")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				key, keyPresent, err := sp.parseFieldValue(fd, cell.Data)
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical keyed list")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				for i := 0; i < reflectList.Len(); i++ {
					item := reflectList.Get(i)
					if xproto.EqualValue(fd, item.Message().Get(fd), key) {
						listItemValue = item
						keyedListItemExisted = true
						break
					}
				}
				elemPresent, err := sp.parseFieldOptions(listItemValue.Message(), rc, depth+1, prefix+field.opts.Name)
				if err != nil {
					kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "vertical keyed list")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				if !keyPresent && !elemPresent {
					break
				}
				if !keyedListItemExisted {
					reflectList.Append(listItemValue)
				}
			} else {
				elemPresent := false
				newListValue := reflectList.NewElement()
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// incell-struct list
					colName := prefix + field.opts.Name
					cell, err := rc.Cell(colName, field.opts.Optional)
					if err != nil {
						kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell struct list")
						return false, xerrors.WithMessageKV(err, kvs...)
					}
					if elemPresent, err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.Sep); err != nil {
						kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell struct list")
						return false, xerrors.WithMessageKV(err, kvs...)
					}
				} else {
					// cross-cell struct list
					elemPresent, err = sp.parseFieldOptions(newListValue.Message(), rc, depth+1, prefix+field.opts.Name)
					if err != nil {
						return false, xerrors.WithMessageKV(err, "cross-cell struct list", "failed to parse struct")
					}
				}
				if elemPresent {
					reflectList.Append(newListValue)
				}
			}
		} else {
			// TODO: support list of scalar type when layout is vertical?
			// NOTE(wenchyzhu): we don't support list of scalar type when layout is vertical
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
			return false, xerrors.ErrorKV("no cell found with digit suffix", xerrors.KeyPBFieldType, "horizontal list")
		}
		fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
		size := detectedSize
		if fixedSize > 0 && fixedSize < detectedSize {
			// squeeze to specified fixed size
			size = fixedSize
		}
		checkRemainFlag := false
		for i := 1; i <= size; i++ {
			newListValue := reflectList.NewElement()
			colName := prefix + field.opts.Name + strconv.Itoa(i)
			elemPresent := false
			if field.fd.Kind() == protoreflect.MessageKind {
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// horizontal incell-struct list
					cell, err := rc.Cell(colName, field.opts.Optional)
					if err != nil {
						kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal incell-struct list")
						return false, xerrors.WithMessageKV(err, kvs...)
					}
					subMsgName := string(field.fd.Message().FullName())
					_, found := xproto.WellKnownMessages[subMsgName]
					if found {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data)
						if err != nil {
							kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal incell-struct list")
							return false, xerrors.WithMessageKV(err, kvs...)
						}
					} else {
						if elemPresent, err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.Sep); err != nil {
							kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal incell-struct list")
							return false, xerrors.WithMessageKV(err, kvs...)
						}
					}
				} else {
					// horizontal struct list
					subMsgName := string(field.fd.Message().FullName())
					_, found := xproto.WellKnownMessages[subMsgName]
					if found {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						cell, err := rc.Cell(colName, field.opts.Optional)
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WithMessageKV(err, kvs...)
						}
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data)
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WithMessageKV(err, kvs...)
						}
					} else {
						elemPresent, err = sp.parseFieldOptions(newListValue.Message(), rc, depth+1, colName)
						if err != nil {
							kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal struct list")
							return false, xerrors.WithMessageKV(err, kvs...)
						}
					}
				}
				if checkRemainFlag {
					// check the remaining keys all not present, otherwise report error!
					if elemPresent {
						kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal struct list")
						return false, xerrors.ErrorKV("elements are not present continuously", kvs...)
					}
					continue
				}
				if !elemPresent && !prop.IsFixed(field.opts.Prop) {
					checkRemainFlag = true
					continue
				}
				reflectList.Append(newListValue)
			} else {
				// scalar list
				cell, err := rc.Cell(colName, field.opts.Optional)
				if err != nil {
					kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal scalar list")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data)
				if err != nil {
					kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal scalar list")
					return false, xerrors.WithMessageKV(err, kvs...)
				}
				if checkRemainFlag {
					// check the remaining keys all not present, otherwise report error!
					if elemPresent {
						kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "horizontal scalar list")
						return false, xerrors.ErrorKV("elements are not present continuously", kvs...)
					}
					continue
				}
				if !elemPresent && !prop.IsFixed(field.opts.Prop) {
					checkRemainFlag = true
					continue
				}
				reflectList.Append(newListValue)
			}
		}

		if prop.IsFixed(field.opts.Prop) {
			for reflectList.Len() < fixedSize {
				// append empty elements to the specified length.
				reflectList.Append(reflectList.NewElement())
			}
		}
	case tableaupb.Layout_LAYOUT_INCELL:
		// incell list
		colName := prefix + field.opts.Name
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell list")
			return false, xerrors.WithMessageKV(err, kvs...)
		}

		// If s does not contain sep and sep is not empty, Split returns a
		// slice of length 1 whose only element is s.
		splits := strings.Split(cell.Data, field.opts.Sep)
		detectedSize := len(splits)
		fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
		size := detectedSize
		if fixedSize > 0 && fixedSize < detectedSize {
			// squeeze to specified fixed size
			size = fixedSize
		}
		for i := 0; i < size; i++ {
			elem := splits[i]
			fieldValue, elemPresent, err := sp.parseFieldValue(field.fd, elem)
			if err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell list")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if !elemPresent && !prop.IsFixed(field.opts.Prop) {
				// TODO: check the remaining keys all not present, otherwise report error!
				break
			}
			// check incell list element range
			if err := prop.CheckInRange(field.opts.Prop, field.fd, fieldValue); err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell list")
				return false, xerrors.WrapKV(err, kvs...)
			}
			if field.opts.Key != "" {
				// keyed list
				keyedListItemExisted := false
				for i := 0; i < reflectList.Len(); i++ {
					item := reflectList.Get(i)
					if xproto.EqualValue(field.fd, item, fieldValue) {
						keyedListItemExisted = true
						break
					}
				}
				if !keyedListItemExisted {
					reflectList.Append(fieldValue)
				}
			} else {
				// normal list
				reflectList.Append(fieldValue)
			}
		}
		if prop.IsFixed(field.opts.Prop) {
			for reflectList.Len() < fixedSize {
				// append empty elements to the specified length.
				reflectList.Append(reflectList.NewElement())
			}
		}
	}
	if !msg.Has(field.fd) && reflectList.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	if msg.Has(field.fd) || reflectList.Len() != 0 {
		present = true
	}
	return present, nil
}

func (sp *sheetParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
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

	structValue := msg.NewField(field.fd)
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will override it if present.
		structValue = msg.Mutable(field.fd)
	}

	colName := prefix + field.opts.Name
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell struct")
			return false, xerrors.WithMessageKV(err, kvs...)
		}

		if present, err = sp.parseIncellStruct(structValue, cell.Data, field.opts.Sep); err != nil {
			kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell struct")
			return false, xerrors.WithMessageKV(err, kvs...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	} else {
		subMsgName := string(field.fd.Message().FullName())
		_, found := xproto.WellKnownMessages[subMsgName]
		if found {
			// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
			cell, err := rc.Cell(colName, field.opts.Optional)
			if err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "wellknown struct")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			value, present, err := sp.parseFieldValue(field.fd, cell.Data)
			if err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "wellknown struct")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if present {
				msg.Set(field.fd, value)
			}
			return present, nil
		} else {
			pkgName := structValue.Message().Descriptor().ParentFile().Package()
			if string(pkgName) != sp.ProtoPackage {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "cross-cell struct")
				return false, xerrors.ErrorKV(fmt.Sprintf("unknown message %v in package %s", subMsgName, pkgName), kvs...)
			}
			present, err := sp.parseFieldOptions(structValue.Message(), rc, depth+1, prefix+field.opts.Name)
			if err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "cross-cell struct")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if present {
				// only set field if it is present.
				msg.Set(field.fd, structValue)
			}
			return present, nil
		}
	}
}

func (sp *sheetParser) parseIncellStruct(structValue protoreflect.Value, cellData, sep string) (present bool, err error) {
	if cellData == "" {
		return false, nil
	}
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, sep)
	subMd := structValue.Message().Descriptor()
	for i := 0; i < subMd.Fields().Len() && i < len(splits); i++ {
		fd := subMd.Fields().Get(i)
		// log.Debugf("fd.FullName().Name(): ", fd.FullName().Name())
		incell := splits[i]
		value, fieldPresent, err := sp.parseFieldValue(fd, incell)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell struct")
		}
		structValue.Message().Set(fd, value)
		if fieldPresent {
			// The struct is treated as present only if one field is present.
			present = true
			structValue.Message().Set(fd, value)
		}
	}
	return present, nil
}

func (sp *sheetParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not polulated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}

	newValue := msg.NewField(field.fd)
	colName := prefix + field.opts.Name
	cell, err := rc.Cell(colName, field.opts.Optional)
	if err != nil {
		kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "scalar")
		return false, xerrors.WithMessageKV(err, kvs...)
	}

	newValue, present, err = sp.parseFieldValue(field.fd, cell.Data)
	if err != nil {
		kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "scalar")
		return false, xerrors.WithMessageKV(err, kvs...)
	}
	if !present {
		return false, nil
	}
	// check range
	if err := prop.CheckInRange(field.opts.Prop, field.fd, newValue); err != nil {
		kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "scalar")
		return false, xerrors.WrapKV(err, kvs...)
	}
	if field.opts.Prop != nil {
		// NOTE: if use NewSheetParser, sp.gen is nil, which means Geneator is not provided.
		if field.opts.Prop.Refer != "" && sp.gen != nil {
			input := &prop.Input{
				ProtoPackage:   sp.gen.ProtoPackage,
				InputDir:       sp.gen.InputDir,
				SubdirRewrites: sp.gen.InputOpt.SubdirRewrites,
				PRFiles:        sp.gen.prFiles,
			}
			ok, err := prop.InReferredSpace(field.opts.Prop.Refer, cell.Data, input)
			if err != nil {
				return false, err
			}
			if !ok {
				err := xerrors.E2002(cell.Data, field.opts.Prop.Refer)
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "scalar")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
		}
	}
	msg.Set(field.fd, newValue)
	return true, nil
}

func (sp *sheetParser) parseFieldValue(fd protoreflect.FieldDescriptor, rawValue string) (v protoreflect.Value, present bool, err error) {
	return xproto.ParseFieldValue(fd, rawValue, sp.LocationName)
}

// ParseFileOptions parse the options of a protobuf definition file.
func ParseFileOptions(fd protoreflect.FileDescriptor) (string, *tableaupb.WorkbookOptions) {
	opts := fd.Options().(*descriptorpb.FileOptions)
	protofile := string(fd.FullName())
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	return protofile, workbook
}

// ParseMessageOptions parse the options of a protobuf message.
func ParseMessageOptions(md protoreflect.MessageDescriptor) (string, *tableaupb.WorksheetOptions) {
	opts := md.Options().(*descriptorpb.MessageOptions)
	msgName := string(md.Name())
	wsOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	if wsOpts.Namerow == 0 {
		wsOpts.Namerow = 1 // default
	}
	if wsOpts.Typerow == 0 {
		wsOpts.Typerow = 2 // default
	}

	if wsOpts.Noterow == 0 {
		wsOpts.Noterow = 3 // default
	}

	if wsOpts.Datarow == 0 {
		wsOpts.Datarow = 4 // default
	}
	// log.Debugf("msg: %v, wsOpts: %+v", msgName, wsOpts)
	return msgName, wsOpts
}
