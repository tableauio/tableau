package confgen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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

// ScatterAndExport parse multiple importer infos into separate protomsgs, then export each other.
func (x *sheetExporter) ScatterAndExport(info *SheetInfo, impInfos ...importer.ImporterInfo) error {
	// NOTE: use map-reduce pattern to accelerate parsing multiple importer Infos.
	var eg errgroup.Group
	for _, impInfo := range impInfos {
		impInfo := impInfo
		// map-reduce: map jobs for concurrent processing
		eg.Go(func() error {
			protomsg, err := parseMessageFromOneImporter(info, impInfo)
			if err != nil {
				return err
			}
			// exported conf name pattern is : <BookName>_<SheetName>
			sheetName := getRealSheetName(info, impInfo)
			name := fmt.Sprintf("%s_%s", impInfo.BookName(), sheetName)
			return storeMessage(protomsg, name, x.OutputDir, x.OutputOpt)
		})
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

// MergeAndExport parse multiple importer infos and merge into one protomsg, then export it.
func (x *sheetExporter) MergeAndExport(info *SheetInfo, impInfos ...importer.ImporterInfo) error {
	protomsg, err := ParseMessage(info, impInfos...)
	if err != nil {
		return err
	}
	return storeMessage(protomsg, string(info.MD.Name()), x.OutputDir, x.OutputOpt)
}

type oneMsg struct {
	protomsg proto.Message
	bookName string
}

// ParseMessage parses multiple importer infos into one protomsg. If an error
// occurs, then wrap it with KeyModule as ModuleConf ("confgen"), then API user
// can call `xerrors.NewDesc(err)“ to print the pretty error message.
func ParseMessage(info *SheetInfo, impInfos ...importer.ImporterInfo) (proto.Message, error) {
	if len(impInfos) == 0 {
		return nil, xerrors.ErrorKV("no importer to be parsed",
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyPrimaryBookName, info.PrimaryBookName,
			xerrors.KeySheetName, info.Opts.Name,
			xerrors.KeyPBMessage, string(info.MD.Name()))
	} else if len(impInfos) == 1 {
		protomsg, err := parseMessageFromOneImporter(info, impInfos[0])
		if err != nil {
			return nil, xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf)
		}
		return protomsg, nil
	}

	// NOTE: use map-reduce pattern to accelerate parsing multiple importer infos.
	var mu sync.Mutex // guard msgs
	var msgs []oneMsg

	var eg errgroup.Group
	for _, impInfo := range impInfos {
		impInfo := impInfo
		// map-reduce: map jobs for concurrent processing
		eg.Go(func() error {
			protomsg, err := parseMessageFromOneImporter(info, impInfo)
			if err != nil {
				return err
			}
			mu.Lock()
			msgs = append(msgs, oneMsg{
				protomsg: protomsg,
				bookName: getRelBookName(info.ExtInfo.InputDir, impInfo.Filename()),
			})
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, xerrors.WrapKV(err,
			xerrors.KeyModule, xerrors.ModuleConf,
			xerrors.KeyPrimaryBookName, info.PrimaryBookName,
			xerrors.KeyPrimarySheetName, info.Opts.Name)
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
						return nil, xerrors.WrapKV(err,
							xerrors.KeyModule, xerrors.ModuleConf,
							xerrors.KeyBookName, bookNames,
							xerrors.KeySheetName, info.Opts.Name,
							xerrors.KeyPBMessage, string(info.MD.Name()))
					}
				}
			}
			return nil, xerrors.WrapKV(err,
				xerrors.KeyModule, xerrors.ModuleConf,
				xerrors.KeyBookName, msg.bookName,
				xerrors.KeySheetName, info.Opts.Name,
				xerrors.KeyPBMessage, string(info.MD.Name()))
		}
	}
	return mainMsg, nil
}

func parseMessageFromOneImporter(info *SheetInfo, impInfo importer.ImporterInfo) (proto.Message, error) {
	sheetName := getRealSheetName(info, impInfo)
	sheet := impInfo.GetSheet(sheetName)
	if sheet == nil {
		bookName := getRelBookName(info.ExtInfo.InputDir, impInfo.Filename())
		err := xerrors.E0001(sheetName, bookName)
		return nil, xerrors.WithMessageKV(err, xerrors.KeyBookName, bookName, xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
	}
	parser := NewExtendedSheetParser(info.ProtoPackage, info.LocationName, info.Opts, info.ExtInfo)
	protomsg := dynamicpb.NewMessage(info.MD)
	if err := parser.Parse(protomsg, sheet); err != nil {
		return nil, xerrors.WithMessageKV(err, xerrors.KeyBookName, getRelBookName(info.ExtInfo.InputDir, impInfo.Filename()), xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
	}
	return protomsg, nil
}

type SheetInfo struct {
	ProtoPackage    string
	LocationName    string
	PrimaryBookName string
	MD              protoreflect.MessageDescriptor
	Opts            *tableaupb.WorksheetOptions

	ExtInfo *SheetParserExtInfo
}

func (si *SheetInfo) HasScatter() bool {
	return si.Opts != nil && len(si.Opts.Scatter) != 0
}

func (si *SheetInfo) HasMerger() bool {
	return si.Opts != nil && len(si.Opts.Merger) != 0
}

type sheetParser struct {
	ProtoPackage string
	LocationName string
	opts         *tableaupb.WorksheetOptions
	extInfo      *SheetParserExtInfo

	// cached name and type
	names       []string               // names[col] -> name
	types       []string               // types[col] -> name
	lookupTable book.ColumnLookupTable // name -> column index (started with 0)
}

// SheetParserExtInfo is the extended info for refer check and so on.
type SheetParserExtInfo struct {
	InputDir       string
	SubdirRewrites map[string]string
	PRFiles        *protoregistry.Files
	BookFormat     format.Format // workbook format
}

// NewSheetParser creates a new sheet parser extended info.
func NewSheetParser(protoPackage, locationName string, opts *tableaupb.WorksheetOptions) *sheetParser {
	return NewExtendedSheetParser(protoPackage, locationName, opts, nil)
}

// NewExtendedSheetParser creates a new sheet parser with extended info.
func NewExtendedSheetParser(protoPackage, locationName string, opts *tableaupb.WorksheetOptions, extInfo *SheetParserExtInfo) *sheetParser {
	return &sheetParser{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		opts:         opts,
		extInfo:      extInfo,
		lookupTable:  map[string]uint32{},
	}
}

// GetBookFormat returns workbook format related to this sheet.
func (sp *sheetParser) GetBookFormat() format.Format {
	if sp.extInfo == nil {
		return format.UnknownFormat
	}
	return sp.extInfo.BookFormat
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

			_, err := sp.parseFieldOptions(msg, curr, "")
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

			_, err := sp.parseFieldOptions(msg, curr, "")
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

// parseFieldOptions is aimed to parse the options of all the fields of a protobuf message.
func (sp *sheetParser) parseFieldOptions(msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			field := parseFieldDescriptor(fd, sp.opts.Sep, sp.opts.Subsep)
			defer field.release()
			fieldPresent, err := sp.parseField(field, msg, rc, prefix)
			if err != nil {
				return xerrors.WithMessageKV(err,
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

func (sp *sheetParser) parseField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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

func (sp *sheetParser) parseMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
			}
			newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
			}
			// value must be empty if key not present
			if !keyPresent && reflectMap.Has(newMapKey) {
				tempCheckMapValue := reflectMap.NewValue()
				valuePresent, err := sp.parseFieldOptions(tempCheckMapValue.Message(), rc, prefix+field.opts.Name)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}
				if valuePresent {
					return false, xerrors.WithMessageKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
				}
				break
			}
			var newMapValue protoreflect.Value
			if reflectMap.Has(newMapKey) {
				newMapValue = reflectMap.Mutable(newMapKey)
			} else {
				newMapValue = reflectMap.NewValue()
			}
			valuePresent, err := sp.parseFieldOptions(newMapValue.Message(), rc, prefix+field.opts.Name)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
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
			// value is scalar type
			key := types.DefaultMapKeyOptName     // default key name
			value := types.DefaultMapValueOptName // default value name
			// key cell
			keyColName := prefix + field.opts.Name + key
			cell, err := rc.Cell(keyColName, field.opts.Optional)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
			}

			fieldValue, keyPresent, err := sp.parseFieldValue(keyFd, cell.Data, field.opts.Prop)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
			}
			newMapKey := fieldValue.MapKey()
			// value cell
			valueColName := prefix + field.opts.Name + value
			cell, err = rc.Cell(valueColName, field.opts.Optional)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(valueColName)...)
			}
			// Currently, we cannot check scalar map value, so do not input field.opts.Prop.
			newMapValue, valuePresent, err := sp.parseFieldValue(field.fd, cell.Data, nil)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(valueColName)...)
			}
			// value must be empty if key not present
			if !keyPresent && reflectMap.Has(newMapKey) {
				if valuePresent {
					return false, xerrors.WithMessageKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
				}
				break
			}
			if !keyPresent && !valuePresent {
				// key and value are both not present.
				break
			}
			// scalar map key must be unique
			if reflectMap.Has(newMapKey) {
				return false, xerrors.WrapKV(xerrors.E2005(cell.Data), rc.CellDebugKV(keyColName)...)
			}
			reflectMap.Set(newMapKey, newMapValue)
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
				cell, err := rc.Cell(keyColName, field.opts.Optional)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}

				newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, cell.Data)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}
				// value must be empty if key not present
				if !keyPresent && reflectMap.Has(newMapKey) {
					tempCheckMapValue := reflectMap.NewValue()
					valuePresent, err := sp.parseFieldOptions(tempCheckMapValue.Message(), rc, prefix+field.opts.Name+strconv.Itoa(i))
					if err != nil {
						return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
					}
					if valuePresent {
						return false, xerrors.WithMessageKV(xerrors.E2017(xproto.GetFieldTypeName(field.fd)), rc.CellDebugKV(keyColName)...)
					}
					break
				}
				var newMapValue protoreflect.Value
				if reflectMap.Has(newMapKey) {
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				valuePresent, err := sp.parseFieldOptions(newMapValue.Message(), rc, prefix+field.opts.Name+strconv.Itoa(i))
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
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
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		colName := prefix + field.opts.Name
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
		err = sp.parseIncellMap(field, reflectMap, cell.Data)
		if err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
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

func (sp *sheetParser) parseIncellMap(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	if valueFd.Kind() == protoreflect.MessageKind {
		if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
			return xerrors.Errorf("map value type is not KV struct, and is not supported")
		}
		err := sp.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cellData)
		if err != nil {
			return err
		}
	} else {
		err := sp.parseIncellMapWithSimpleKV(field, reflectMap, cellData)
		if err != nil {
			return err
		}
	}
	return nil
}

// parseIncellMapWithSimpleKV parses simple incell map with key as scalar type and value as scalar or enum type.
// For example:
//   - map<int32, int32>
//   - map<int32, EnumType>
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

		fieldValue, keyPresent, err := sp.parseFieldValue(keyFd, key, field.opts.Prop)
		if err != nil {
			return err
		}

		newMapKey := fieldValue.MapKey()
		// Currently, we cannot check scalar map value, so do not input field.opts.Prop.
		fieldValue, valuePresent, err := sp.parseFieldValue(valueFd, value, nil)
		if err != nil {
			return err
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
//	 enum FruitType {
//	   FRUIT_TYPE_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//	   FRUIT_TYPE_APPLE   = 1 [(tableau.evalue).name = "Apple"];
//	   FRUIT_TYPE_ORANGE  = 2 [(tableau.evalue).name = "Orange"];
//	   FRUIT_TYPE_BANANA  = 3 [(tableau.evalue).name = "Banana"];
//	 }
//	 enum FruitFlavor {
//	   FRUIT_FLAVOR_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//	   FRUIT_FLAVOR_FRAGRANT = 1 [(tableau.evalue).name = "Fragrant"];
//	   FRUIT_FLAVOR_SOUR = 2 [(tableau.evalue).name = "Sour"];
//	   FRUIT_FLAVOR_SWEET = 3 [(tableau.evalue).name = "Sweet"];
//	 }
//
//	 map<int32, Fruit> fruit_map = 1 [(tableau.field) = {name:"Fruit" key:"Key" layout:LAYOUT_INCELL}];
//	 message Fruit {
//	   FruitType key = 1 [(tableau.field) = {name:"Key"}];
//	   int64 value = 2 [(tableau.field) = {name:"Value"}];
//		}
//
//	 map<int32, Item> item_map = 3 [(tableau.field) = {name:"Item" key:"Key" layout:LAYOUT_INCELL}];
//	 message Item {
//	   FruitType key = 1 [(tableau.field) = {name:"Key"}];
//	   FruitFlavor value = 2 [(tableau.field) = {name:"Value"}];
//	 }
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
			return err
		}

		newMapValue := reflectMap.NewValue()
		valuePresent, err := sp.parseIncellStruct(newMapValue, mapItemData, field.opts.GetProp().GetForm(), field.opts.Subsep)
		if err != nil {
			return err
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
		fieldValue, present, err = sp.parseFieldValue(keyFd, cellData, field.opts.Prop)
		if err != nil {
			return mapKey, false, err
		}
		v := protoreflect.ValueOfInt32(int32(fieldValue.Enum()))
		mapKey = v.MapKey()
	} else {
		fieldValue, present, err = sp.parseFieldValue(keyFd, cellData, field.opts.Prop)
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

// deduceMapKeyUnique deduces whether the map key unique or not.
//
// By default, map key can be duplicate, in order to aggregate sub-field
// (map or list) with cardinality. The map key should be deduced to be
// unique if nesting hierarchy is like:
//
//   - map nesting map or list with different layout (vertical or horizontal).
//   - map nesting no map or list.
//   - map layout is incell.
func (sp *sheetParser) deduceMapKeyUnique(field *Field, reflectMap protoreflect.Map) bool {
	if sp.GetBookFormat() != format.Excel && sp.GetBookFormat() != format.CSV {
		// Only Excel and CSV will be deduced.
		return false
	}
	layout := field.opts.Layout
	if field.opts.Layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// Map default layout is vertical
		layout = tableaupb.Layout_LAYOUT_VERTICAL
	}
	if field.opts.Layout == tableaupb.Layout_LAYOUT_INCELL {
		// incell map key must be unique
		return true
	}
	md := reflectMap.NewValue().Message().Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.IsMap() {
			childField := parseFieldDescriptor(fd, sp.opts.Sep, sp.opts.Subsep)
			defer childField.release()
			childLayout := childField.opts.Layout
			if childLayout == tableaupb.Layout_LAYOUT_DEFAULT {
				// Map default layout is vertical
				childLayout = tableaupb.Layout_LAYOUT_VERTICAL
			}
			if childLayout == tableaupb.Layout_LAYOUT_INCELL {
				// ignore incell map
				continue
			}
			if childLayout == layout {
				// map key must not be unique because its value has child with same layout
				// but if it's assigned to be unique, it must be unique
				return false
			}
		} else if fd.IsList() {
			childField := parseFieldDescriptor(fd, sp.opts.Sep, sp.opts.Subsep)
			defer childField.release()
			childLayout := childField.opts.Layout
			if childLayout == tableaupb.Layout_LAYOUT_DEFAULT {
				// List default layout is horizontal
				childLayout = tableaupb.Layout_LAYOUT_HORIZONTAL
			}
			if childLayout == tableaupb.Layout_LAYOUT_INCELL {
				// ignore incell list
				continue
			}
			if childLayout == layout {
				// map key must not be unique because its value has child with same layout
				// but if it's assigned to be unique, it must be unique
				return false
			}
		}
	}
	// map key must be unique because its value has no child with same layout
	return true
}

func (sp *sheetParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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
				listItemValue := list.NewElement()
				keyedListItemExisted := false
				keyColName := prefix + field.opts.Name + field.opts.Key
				md := listItemValue.Message().Descriptor()
				keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))

				fd := md.Fields().ByName(keyProtoName)
				if fd == nil {
					return false, xerrors.ErrorKV(fmt.Sprintf("key field not found in proto definition: %s", keyProtoName), rc.CellDebugKV(keyColName)...)
				}
				cell, err := rc.Cell(keyColName, field.opts.Optional)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}
				key, keyPresent, err := sp.parseFieldValue(fd, cell.Data, field.opts.Prop)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}
				for i := 0; i < list.Len(); i++ {
					item := list.Get(i)
					if xproto.EqualValue(fd, item.Message().Get(fd), key) {
						listItemValue = item
						keyedListItemExisted = true
						break
					}
				}
				elemPresent, err := sp.parseFieldOptions(listItemValue.Message(), rc, prefix+field.opts.Name)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(keyColName)...)
				}
				if !keyPresent && !elemPresent {
					break
				}
				if !keyedListItemExisted {
					list.Append(listItemValue)
				}
			} else {
				elemPresent := false
				newListValue := list.NewElement()
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// incell-struct list
					colName := prefix + field.opts.Name
					cell, err := rc.Cell(colName, field.opts.Optional)
					if err != nil {
						return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
					}
					if elemPresent, err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.GetProp().GetForm(), field.opts.Sep); err != nil {
						return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
					}
				} else {
					// cross-cell struct list
					elemPresent, err = sp.parseFieldOptions(newListValue.Message(), rc, prefix+field.opts.Name)
					if err != nil {
						return false, xerrors.WithMessageKV(err, "cross-cell struct list", "failed to parse struct")
					}
				}
				if elemPresent {
					list.Append(newListValue)
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
					cell, err := rc.Cell(colName, field.opts.Optional)
					if err != nil {
						return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
					}
					subMsgName := string(field.fd.Message().FullName())
					if types.IsWellKnownMessage(subMsgName) {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
						if err != nil {
							return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
						}
					} else {
						if elemPresent, err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.GetProp().GetForm(), field.opts.Sep); err != nil {
							return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
						}
					}
				} else {
					// horizontal struct list
					subMsgName := string(field.fd.Message().FullName())
					if types.IsWellKnownMessage(subMsgName) {
						// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
						cell, err := rc.Cell(colName, field.opts.Optional)
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WithMessageKV(err, kvs...)
						}
						newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
						if err != nil {
							kvs := rc.CellDebugKV(colName)
							return false, xerrors.WithMessageKV(err, kvs...)
						}
					} else {
						elemPresent, err = sp.parseFieldOptions(newListValue.Message(), rc, colName)
						if err != nil {
							return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
						}
					}
				}
				if firstNonePresentIndex != 0 {
					// Check that no empty element is existed in between, so we should guarantee
					// that all the remaining elements are not present, otherwise report error!
					if elemPresent {
						return false, xerrors.WithMessageKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(colName)...)
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
				cell, err := rc.Cell(colName, field.opts.Optional)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
				}
				newListValue, elemPresent, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
				if err != nil {
					return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
				}
				if firstNonePresentIndex != 0 {
					// check the remaining scalar elements are not present, otherwise report error!
					if elemPresent {
						return false, xerrors.WithMessageKV(xerrors.E2016(firstNonePresentIndex, i), rc.CellDebugKV(colName)...)
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
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
		present, err = sp.parseIncellListField(field, list, cell.Data)
		if err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
	}
	if !msg.Has(field.fd) && list.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	if msg.Has(field.fd) || list.Len() != 0 {
		present = true
	}
	return present, nil
}

func (sp *sheetParser) parseIncellListField(field *Field, list protoreflect.List, cellData string) (present bool, err error) {
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, field.opts.Sep)
	detectedSize := len(splits)
	fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
	size := detectedSize
	if fixedSize > 0 && fixedSize < detectedSize {
		// squeeze to specified fixed size
		size = fixedSize
	}
	for i := 0; i < size; i++ {
		elem := splits[i]
		fieldValue, elemPresent, err := sp.parseFieldValue(field.fd, elem, field.opts.Prop)
		if err != nil {
			return false, err
		}
		if !elemPresent && !prop.IsFixed(field.opts.Prop) {
			// TODO: check the remaining keys all not present, otherwise report error!
			break
		}
		if field.opts.Key != "" {
			// keyed list
			keyedListItemExisted := false
			for i := 0; i < list.Len(); i++ {
				item := list.Get(i)
				if xproto.EqualValue(field.fd, item, fieldValue) {
					keyedListItemExisted = true
					break
				}
			}
			if !keyedListItemExisted {
				list.Append(fieldValue)
			}
		} else {
			// normal list
			list.Append(fieldValue)
		}
	}
	if prop.IsFixed(field.opts.Prop) {
		for list.Len() < fixedSize {
			// append empty elements to the specified length.
			list.Append(list.NewElement())
		}
	}
	return list.Len() != 0, nil
}

func (sp *sheetParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}

		if present, err = sp.parseIncellStruct(structValue, cell.Data, field.opts.GetProp().GetForm(), field.opts.Sep); err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	} else {
		subMsgName := string(field.fd.Message().FullName())
		if types.IsWellKnownMessage(subMsgName) {
			// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
			cell, err := rc.Cell(colName, field.opts.Optional)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
			}
			value, present, err := sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
			}
			if present {
				msg.Set(field.fd, value)
			}
			return present, nil
		} else {
			present, err := sp.parseFieldOptions(structValue.Message(), rc, prefix+field.opts.Name)
			if err != nil {
				return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
			}
			if present {
				// only set field if it is present.
				msg.Set(field.fd, structValue)
			}
			return present, nil
		}
	}
}

func (sp *sheetParser) parseIncellStruct(structValue protoreflect.Value, cellData string, form tableaupb.Form, sep string) (present bool, err error) {
	if cellData == "" {
		return false, nil
	}
	switch form {
	case tableaupb.Form_FORM_TEXT:
		if err := prototext.Unmarshal([]byte(cellData), structValue.Message().Interface()); err != nil {
			return false, xerrors.Errorf("unmarshal from text failed: %v", err)
		}
		return true, nil
	case tableaupb.Form_FORM_JSON:
		if err := protojson.Unmarshal([]byte(cellData), structValue.Message().Interface()); err != nil {
			return false, xerrors.Errorf("unmarshal from JSON failed: %v", err)
		}
		return true, nil
	default:
		// If s does not contain sep and sep is not empty, Split returns a
		// slice of length 1 whose only element is s.
		splits := strings.Split(cellData, sep)
		md := structValue.Message().Descriptor()
		for i := 0; i < md.Fields().Len() && i < len(splits); i++ {
			fd := md.Fields().Get(i)
			// log.Debugf("fd.FullName().Name(): ", fd.FullName().Name())
			incell := splits[i]
			value, fieldPresent, err := sp.parseFieldValue(fd, incell, nil)
			if err != nil {
				return false, err
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
}

func (sp *sheetParser) parseUnionField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
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
		cell, err := rc.Cell(colName, field.opts.Optional)
		if err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
		if present, err = sp.parseIncellUnion(structValue, cell.Data, field.opts.GetProp().GetForm()); err != nil {
			return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}

	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeColName := prefix + field.opts.Name + unionDesc.TypeName()
	cell, err := rc.Cell(typeColName, field.opts.Optional)
	if err != nil {
		return false, xerrors.WithMessageKV(err, rc.CellDebugKV(typeColName)...)
	}

	typeVal, present, err := sp.parseFieldValue(unionDesc.Type, cell.Data, nil)
	if err != nil {
		return false, xerrors.WithMessageKV(err, rc.CellDebugKV(typeColName)...)
	}
	structValue.Message().Set(unionDesc.Type, typeVal)

	// parse value
	fieldNumber := int32(typeVal.Enum())
	if fieldNumber == 0 {
		// default enum value 0, no need to parse.
		return false, nil
	}
	valueFD := unionDesc.GetValueByNumber(fieldNumber)
	if valueFD == nil {
		typeValue := unionDesc.Type.Enum().Values().ByNumber(protoreflect.EnumNumber(fieldNumber)).Name()
		return false, xerrors.WithMessageKV(xerrors.E2010(typeValue, fieldNumber), rc.CellDebugKV(typeColName)...)
	}
	fieldValue := structValue.Message().NewField(valueFD)
	if valueFD.Kind() == protoreflect.MessageKind {
		// MUST be message type.
		md := valueFD.Message()
		msg := fieldValue.Message()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			colName := prefix + field.opts.Name + unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
			err := func() error {
				subField := parseFieldDescriptor(fd, sp.opts.Sep, sp.opts.Subsep)
				defer subField.release()
				// incell scalar
				cell, err := rc.Cell(colName, subField.opts.Optional)
				if err != nil {
					return xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
				}
				err = sp.parseUnionValueField(subField, msg, cell.Data)
				if err != nil {
					return xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
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
	structValue.Message().Set(valueFD, fieldValue)

	if present {
		msg.Set(field.fd, structValue)
	}
	return present, nil
}

func (sp *sheetParser) parseUnionValueField(field *Field, msg protoreflect.Message, cellData string) error {
	if field.fd.IsMap() {
		// incell map
		value := msg.NewField(field.fd)
		err := sp.parseIncellMap(field, value.Map(), cellData)
		if err != nil {
			return err
		}
		if !msg.Has(field.fd) && value.Map().Len() != 0 {
			msg.Set(field.fd, value)
		}
	} else if field.fd.IsList() {
		// incell list
		value := msg.NewField(field.fd)
		present, err := sp.parseIncellListField(field, value.List(), cellData)
		if err != nil {
			return err
		}
		if present {
			msg.Set(field.fd, value)
		}

	} else if field.fd.Kind() == protoreflect.MessageKind {
		subMsgName := string(field.fd.Message().FullName())
		if types.IsWellKnownMessage(subMsgName) {
			// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
			value, present, err := sp.parseFieldValue(field.fd, cellData, field.opts.Prop)
			if err != nil {
				return err
			}
			if present {
				msg.Set(field.fd, value)
			}
			return nil
		}
		// incell struct
		value := msg.NewField(field.fd)
		present, err := sp.parseIncellStruct(value, cellData, field.opts.GetProp().GetForm(), field.opts.Sep)
		if err != nil {
			return err
		}
		if present {
			msg.Set(field.fd, value)
		}
	} else {
		val, present, err := sp.parseFieldValue(field.fd, cellData, field.opts.Prop)
		if err != nil {
			return err
		}
		if present {
			msg.Set(field.fd, val)
		}
	}
	return nil
}

func (sp *sheetParser) parseIncellUnion(structValue protoreflect.Value, cellData string, form tableaupb.Form) (present bool, err error) {
	if cellData == "" {
		return false, nil
	}
	switch form {
	case tableaupb.Form_FORM_TEXT:
		if err := prototext.Unmarshal([]byte(cellData), structValue.Message().Interface()); err != nil {
			return false, xerrors.Errorf("unmarshal from text failed: %v", err)
		}
		return true, nil
	case tableaupb.Form_FORM_JSON:
		if err := protojson.Unmarshal([]byte(cellData), structValue.Message().Interface()); err != nil {
			return false, xerrors.Errorf("unmarshal from JSON failed: %v", err)
		}
		return true, nil
	default:
		return false, xerrors.Errorf("illegal cell data form: %s", form.String())
	}
}

func (sp *sheetParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, prefix string) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not populated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
	colName := prefix + field.opts.Name
	cell, err := rc.Cell(colName, field.opts.Optional)
	if err != nil {
		return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
	}

	newValue, present, err = sp.parseFieldValue(field.fd, cell.Data, field.opts.Prop)
	if err != nil {
		return false, xerrors.WithMessageKV(err, rc.CellDebugKV(colName)...)
	}
	if !present {
		return false, nil
	}
	msg.Set(field.fd, newValue)
	return true, nil
}

func (sp *sheetParser) parseFieldValue(fd protoreflect.FieldDescriptor, rawValue string, fprop *tableaupb.FieldProp) (v protoreflect.Value, present bool, err error) {
	v, present, err = xproto.ParseFieldValue(fd, rawValue, sp.LocationName)
	if err != nil {
		return v, present, err
	}

	if fprop != nil {
		// check presence
		if err := prop.CheckPresence(fprop, present); err != nil {
			return v, present, err
		}
		// check range
		if err := prop.CheckInRange(fprop, fd, v, present); err != nil {
			return v, present, err
		}
		// check refer
		// NOTE: if use NewSheetParser, sp.extInfo is nil, which means SheetParserExtInfo is not provided.
		if fprop.Refer != "" && sp.extInfo != nil {
			input := &prop.Input{
				ProtoPackage:   sp.ProtoPackage,
				InputDir:       sp.extInfo.InputDir,
				SubdirRewrites: sp.extInfo.SubdirRewrites,
				PRFiles:        sp.extInfo.PRFiles,
				Present:        present,
			}
			ok, err := prop.InReferredSpace(fprop, rawValue, input)
			if err != nil {
				return v, present, err
			}
			if !ok {
				return v, present, xerrors.E2002(rawValue, fprop.Refer)
			}
		}
	}

	return v, present, err
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
