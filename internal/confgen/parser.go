package confgen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/confgen/mexporter"
	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
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
func (x *sheetExporter) Export(info *SheetInfo, importers ...importer.Importer) error {
	protomsg, err := ParseMessage(info, importers...)
	if err != nil {
		return err
	}
	exporter := mexporter.New(string(info.MD.Name()), protomsg, x.OutputDir, x.OutputOpt, info.Opts)
	if err := exporter.Export(); err != nil {
		return err
	}
	return nil
}

type oneMsg struct {
	protomsg proto.Message
	bookName string
}

func ParseMessage(info *SheetInfo, importers ...importer.Importer) (proto.Message, error) {
	if len(importers) == 1 {
		return parseMessageFromOneImporter(info, importers[0])
	} else if len(importers) == 0 {
		return nil, xerrors.ErrorKV("no protomsg parsed", xerrors.KeySheetName, info.Opts.Name, xerrors.KeyPBMessage, string(info.MD.Name()))
	}

	// NOTE: use map-reduce pattern to accelerate parsing multiple importers.
	var mu sync.Mutex // guard msgs
	var msgs []oneMsg

	var eg errgroup.Group
	for _, imp := range importers {
		imp := imp
		// map-reduce: map jobs for concurrent processing
		eg.Go(func() error {
			protomsg, err := parseMessageFromOneImporter(info, imp)
			if err != nil {
				return err
			}
			mu.Lock()
			msgs = append(msgs, oneMsg{
				protomsg: protomsg,
				bookName: imp.Filename(),
			})
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// map-reduce: reduce results to one
	mainMsg := dynamicpb.NewMessage(info.MD) // treat the first one as main msg
	for i := 0; i < len(msgs); i++ {
		msg := msgs[i]
		err := xproto.Merge(mainMsg, msg.protomsg)
		if err != nil {
			if errors.Is(err, xproto.ErrDuplicateKey) {
				// find the already existed key before
				for j := 0; j < i; j++ {
					prevMsg := msgs[j]
					err := xproto.CheckMapDuplicateKey(prevMsg.protomsg, msg.protomsg)
					if err != nil {
						bookNames := prevMsg.bookName + ", " + msg.bookName
						return nil, xerrors.WrapKV(err, xerrors.KeyBookName, bookNames, xerrors.KeySheetName, info.Opts.Name, xerrors.KeyPBMessage, string(info.MD.Name()))
					}
				}
			}
			return nil, xerrors.WrapKV(err, xerrors.KeyBookName, msg.bookName, xerrors.KeySheetName, info.Opts.Name, xerrors.KeyPBMessage, string(info.MD.Name()))
		}
	}
	return mainMsg, nil
}

func parseMessageFromOneImporter(info *SheetInfo, imp importer.Importer) (proto.Message, error) {
	sheetName := info.Opts.Name
	sheet := imp.GetSheet(sheetName)
	if sheet == nil {
		err := xerrors.E0001(sheetName, imp.Filename())
		return nil, xerrors.WithMessageKV(err, xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
	}
	parser := newSheetParserInternal(info)
	protomsg := dynamicpb.NewMessage(info.MD)
	if err := parser.Parse(protomsg, sheet); err != nil {
		return nil, xerrors.WithMessageKV(err, xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
	}
	return protomsg, nil
}

type SheetInfo struct {
	ProtoPackage string
	LocationName string

	MD   protoreflect.MessageDescriptor
	Opts *tableaupb.WorksheetOptions

	gen *Generator // NOTE: only set in internal package, currently only for refer check.
}

func NewSheetInfo(protoPackage, locationName string, md protoreflect.MessageDescriptor, opts *tableaupb.WorksheetOptions) *SheetInfo {
	return &SheetInfo{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		MD:           md,
		Opts:         opts,
	}
}

func newSheetParserInternal(info *SheetInfo) *sheetParser {
	parser := NewSheetParser(info.ProtoPackage, info.LocationName, info.Opts)
	parser.gen = info.gen // set generator
	return parser
}

// NewSheetParser create a new sheet parser without Generator.
func NewSheetParser(protoPackage, locationName string, opts *tableaupb.WorksheetOptions) *sheetParser {
	return &sheetParser{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		gen:          nil,
		opts:         opts,
		lookupTable:  map[string]uint32{},
	}
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
				curr.NewCell(row, &sp.names[row], &sp.types[row], data, sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[row]] = uint32(row)
			}
			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseFieldOptions(msg, curr, 0, "")
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
				curr.NewCell(col, &sp.names[col], &sp.types[col], data, sp.opts.AdjacentKey)
				sp.lookupTable[sp.names[col]] = uint32(col)
			}

			curr.SetColumnLookupTable(sp.lookupTable)

			_, err := sp.parseFieldOptions(msg, curr, 0, "")
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
				name = strings.TrimSuffix(name, types.DefaultListFieldOptNameSuffix)
			} else if fd.IsMap() {
				// truncate suffix `Map` (CamelCase) corresponding to `_map` (snake_case)
				name = strings.TrimSuffix(name, types.DefaultMapFieldOptNameSuffix)
				key = types.DefaultMapKeyOptName
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
		if IsUnionField(field.fd) {
			return sp.parseUnionField(field, msg, rc, depth, prefix)
		}
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
			key := types.DefaultMapKeyOptName     // default key name
			value := types.DefaultMapValueOptName // default value name
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
				// This means the front continuous present cells (related to this map)
				// has already been parsed.
				break
			}
			detectedSize := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
			if detectedSize <= 0 {
				return false, xerrors.ErrorKV("no cell found with digit suffix", xerrors.KeyPBFieldType, "horizontal map")
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
				if checkRemainFlag {
					// Both key and value are not present.
					// Check that no empty item is existed in between, so we should guarantee
					// that all the remaining items are not present, otherwise report error!
					if keyPresent || valuePresent {
						kvs := append(rc.CellDebugKV(keyColName), xerrors.KeyPBFieldType, "horizontal struct map")
						return false, xerrors.ErrorKV("map items are not present continuously", kvs...)
					}
					continue
				}
				if !keyPresent && !valuePresent && !prop.IsFixed(field.opts.Prop) {
					checkRemainFlag = true
					continue
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
			if !types.CheckMessageWithOnlyKVFields(reflectMap.NewValue().Message()) {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "incell map")
				return false, xerrors.ErrorKV("map value type is not KV struct, and is not supported", kvs...)
			}
			err := sp.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cell.Data)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName))
			}
		} else {
			err := sp.parseIncellMapWithSimpleKV(field, reflectMap, cell.Data)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName))
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

// parseIncellMapWithSimpleKV parses simple incell map with key as scalar type and value as scalar or enum type.
// For example:
//  - map<int32, int32>
//  - map<int32, EnumType>
func (sp *sheetParser) parseIncellMapWithSimpleKV(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	if cellData == "" {
		return nil
	}
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	splits := strings.Split(cellData, field.opts.Sep)
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
			return xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell map")
		}

		newMapKey := fieldValue.MapKey()
		fieldValue, valuePresent, err := sp.parseFieldValue(valueFd, value)
		if err != nil {
			return xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell map")
		}
		newMapValue := fieldValue

		if !keyPresent && !valuePresent {
			// key and value are both not present.
			// TODO: check the remaining keys all not present, otherwise report error!
			break
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return nil
}

// parseIncellMapWithValueAsSimpleKVMessage parses simple incell map with key as scalar or enum type
// and value as simple message with only key and value fields. For example:
//
//  enum FruitType {
//    FRUIT_TYPE_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//    FRUIT_TYPE_APPLE   = 1 [(tableau.evalue).name = "Apple"];
//    FRUIT_TYPE_ORANGE  = 2 [(tableau.evalue).name = "Orange"];
//    FRUIT_TYPE_BANANA  = 3 [(tableau.evalue).name = "Banana"];
//  }
//  enum FruitFlavor {
//    FRUIT_FLAVOR_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//    FRUIT_FLAVOR_FRAGRANT = 1 [(tableau.evalue).name = "Fragrant"];
//    FRUIT_FLAVOR_SOUR = 2 [(tableau.evalue).name = "Sour"];
//    FRUIT_FLAVOR_SWEET = 3 [(tableau.evalue).name = "Sweet"];
//  }
//
//  map<int32, Fruit> fruit_map = 1 [(tableau.field) = {name:"Fruit" key:"Key" layout:LAYOUT_INCELL}];
//  message Fruit {
//    FruitType key = 1 [(tableau.field) = {name:"Key"}];
//    int64 value = 2 [(tableau.field) = {name:"Value"}];
// 	}
//
//  map<int32, Item> item_map = 3 [(tableau.field) = {name:"Item" key:"Key" layout:LAYOUT_INCELL}];
//  message Item {
//    FruitType key = 1 [(tableau.field) = {name:"Key"}];
//    FruitFlavor value = 2 [(tableau.field) = {name:"Value"}];
//  }
func (sp *sheetParser) parseIncellMapWithValueAsSimpleKVMessage(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	if cellData == "" {
		return nil
	}
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, field.opts.Sep)
	size := len(splits)
	for i := 0; i < size; i++ {
		mapItemData := splits[i]
		kv := strings.SplitN(mapItemData, field.opts.Subsep, 2)
		if len(kv) == 1 {
			// If value is not set, then treated it as default empty string.
			kv = append(kv, "")
		}
		keyData := kv[0]

		newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, keyData)
		if err != nil {
			return xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell map")
		}

		newMapValue := reflectMap.NewValue()
		valuePresent, err := sp.parseIncellStruct(newMapValue, mapItemData, field.opts.Subsep)
		if err != nil {
			return xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell struct list")
		}

		if !keyPresent && !valuePresent {
			// key and value are both not present.
			// TODO: check the remaining keys all not present, otherwise report error!
			break
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return nil
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
			// NOTE(wenchy): we don't support list of scalar type when layout is vertical
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
					// Check that no empty element is existed in between, so we should guarantee
					// that all the remaining elements are not present, otherwise report error!
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

func (sp *sheetParser) parseUnionField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	unionDesc := ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse type
	typeColName := prefix + field.opts.Name + unionDesc.TypeName()
	cell, err := rc.Cell(typeColName, field.opts.Optional)
	if err != nil {
		kvs := append(rc.CellDebugKV(typeColName), xerrors.KeyPBFieldType, "union type")
		return false, xerrors.WithMessageKV(err, kvs...)
	}

	typeVal, present, err := sp.parseFieldValue(unionDesc.Type, cell.Data)
	if err != nil {
		kvs := append(rc.CellDebugKV(typeColName), xerrors.KeyPBFieldType, "enum")
		return false, xerrors.WithMessageKV(err, kvs...)
	}
	structValue.Message().Set(unionDesc.Type, typeVal)

	// parse value
	valueFD := unionDesc.GetValueByNumber(int32(typeVal.Enum()))
	fieldValue := structValue.Message().NewField(valueFD)
	if valueFD.Kind() == protoreflect.MessageKind {
		// MUST be message type.
		md := valueFD.Message()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			// incell scalar
			colName := prefix + field.opts.Name + unionDesc.ValueFieldName() + strconv.Itoa(i+1)
			cell, err := rc.Cell(colName, field.opts.Optional)
			if err != nil {
				kvs := append(rc.CellDebugKV(colName), xerrors.KeyPBFieldType, "union type")
				return false, xerrors.WithMessageKV(err, kvs...)
			}
			if fd.IsMap() {
				// incell map
				// TODO
				continue
			} else if fd.IsList() {
				// incell list
				// TODO
				continue
			} else if fd.Kind() == protoreflect.MessageKind {
				// incell struct
				value := fieldValue.Message().NewField(fd)
				present, err := sp.parseIncellStruct(value, cell.Data, field.opts.Sep)
				if err != nil {
					return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "incell struct list")
				}
				if present {
					fieldValue.Message().Set(fd, value)
				}
			} else {
				val, present, err := sp.parseFieldValue(fd, cell.Data)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
				}
				if present {
					fieldValue.Message().Set(fd, val)
				}
			}
		}
	} else {
		// scalar: not supported yet.
		return false, xerrors.Errorf("union value (oneof) as scalar type not supported: %s", valueFD.FullName())
	}
	structValue.Message().Set(valueFD, fieldValue)

	if present {
		msg.Set(field.fd, structValue)
	}
	return present, nil
}

func (sp *sheetParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not polulated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
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
