package confgen

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
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
func (x *sheetExporter) ScatterAndExport(info *SheetInfo,
	mainImpInfo importer.ImporterInfo,
	impInfos ...importer.ImporterInfo) error {
	// exported conf name pattern is : [ParentDir/]<BookName>_<SheetName>
	getExportedConfName := func(info *SheetInfo, impInfo importer.ImporterInfo) string {
		sheetName := getRealSheetName(info, impInfo)
		// here filename has no ext suffix
		var filename string
		if info.Opts.ScatterWithoutBookName {
			filename = sheetName
		} else {
			filename = fmt.Sprintf("%s_%s", impInfo.BookName(), sheetName)
		}
		if info.Opts.WithParentDir {
			parentDirName := xfs.GetDirectParentDirName(impInfo.Filename())
			return filepath.Join(parentDirName, filename)
		}
		return filename
	}
	// parse main sheet
	mainMsg, err := parseMessageFromOneImporter(info, mainImpInfo)
	if err != nil {
		return err
	}
	mainName := getExportedConfName(info, mainImpInfo)
	err = storeMessage(mainMsg, mainName, x.OutputDir, x.OutputOpt)
	if err != nil {
		return err
	}

	var eg errgroup.Group
	for _, impInfo := range impInfos {
		impInfo := impInfo
		// map-reduce: map jobs for concurrent processing
		eg.Go(func() error {
			msg, err := parseMessageFromOneImporter(info, impInfo)
			if err != nil {
				return err
			}
			name := getExportedConfName(info, impInfo)
			if info.Opts.Patch == tableaupb.Patch_PATCH_MERGE {
				if info.ExtInfo.DryRun == options.DryRunPatch {
					clonedMainMsg := proto.Clone(mainMsg)
					xproto.PatchMessage(clonedMainMsg, msg)
					msg = clonedMainMsg
				} else {
					return storePatchMergeMessage(msg, name, x.OutputDir, x.OutputOpt)
				}
			}
			return storeMessage(msg, name, x.OutputDir, x.OutputOpt)
		})
	}
	return eg.Wait()
}

// MergeAndExport parse multiple importer infos and merge into one protomsg, then export it.
func (x *sheetExporter) MergeAndExport(info *SheetInfo,
	mainImpInfo importer.ImporterInfo,
	impInfos ...importer.ImporterInfo) error {
	// append main
	allImpInfos := append(impInfos, importer.ImporterInfo{Importer: mainImpInfo})
	protomsg, err := ParseMessage(info, allImpInfos...)
	if err != nil {
		return err
	}
	// exported conf name pattern is : [ParentDir/]<SheetName>
	getExportedConfName := func(info *SheetInfo, impInfo importer.ImporterInfo) string {
		// here filename has no ext suffix
		filename := string(info.MD.Name())
		if info.Opts.WithParentDir {
			parentDirName := xfs.GetDirectParentDirName(impInfo.Filename())
			return filepath.Join(parentDirName, filename)
		}
		return filename
	}
	name := getExportedConfName(info, mainImpInfo)
	return storeMessage(protomsg, name, x.OutputDir, x.OutputOpt)
}

type oneMsg struct {
	protomsg  proto.Message
	bookName  string
	sheetName string
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
				protomsg:  protomsg,
				bookName:  getRelBookName(info.ExtInfo.InputDir, impInfo.Filename()),
				sheetName: getRealSheetName(info, impInfo),
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
						sheetNames := prevMsg.sheetName + ", " + msg.sheetName
						return nil, xerrors.WrapKV(err,
							xerrors.KeyModule, xerrors.ModuleConf,
							xerrors.KeyBookName, bookNames,
							xerrors.KeySheetName, sheetNames,
							xerrors.KeyPBMessage, string(info.MD.Name()))
					}
				}
			}
			return nil, xerrors.WrapKV(err,
				xerrors.KeyModule, xerrors.ModuleConf,
				xerrors.KeyBookName, msg.bookName,
				xerrors.KeySheetName, msg.sheetName,
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
		return nil, xerrors.WrapKV(err, xerrors.KeyBookName, bookName, xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
	}
	parser := NewExtendedSheetParser(info.ProtoPackage, info.LocationName, info.Opts, info.ExtInfo)
	protomsg := dynamicpb.NewMessage(info.MD)
	if err := parser.Parse(protomsg, sheet); err != nil {
		return nil, xerrors.WrapKV(err, xerrors.KeyBookName, getRelBookName(info.ExtInfo.InputDir, impInfo.Filename()), xerrors.KeySheetName, sheetName, xerrors.KeyPBMessage, string(info.MD.Name()))
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
	Sep            string // global-level separator, generally set by options.ConfInputOption.Sep
	Subsep         string // global-level subseparator, generally set by options.ConfInputOption.Subsep
	PRFiles        *protoregistry.Files
	BookFormat     format.Format // workbook format
	DryRun         options.DryRun
}

// NewSheetParser creates a new sheet parser.
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

// GetSep returns sheet-level separator.
func (sp *sheetParser) GetSep() string {
	// sheet-level
	if sp.opts.Sep != "" {
		return sp.opts.Sep
	}
	// global-level
	if sp.extInfo != nil && sp.extInfo.Sep != "" {
		return sp.extInfo.Sep
	}
	// default
	return options.DefaultSep
}

// GetSubsep returns sheet-level subseparator.
func (sp *sheetParser) GetSubsep() string {
	// sheet-level
	if sp.opts.Subsep != "" {
		return sp.opts.Subsep
	}
	// global-level
	if sp.extInfo != nil && sp.extInfo.Subsep != "" {
		return sp.extInfo.Subsep
	}
	// default
	return options.DefaultSubsep
}

// GetBookFormat returns workbook format related to this sheet.
func (sp *sheetParser) GetBookFormat() format.Format {
	if sp.extInfo == nil {
		return format.UnknownFormat
	}
	return sp.extInfo.BookFormat
}

// IsFieldOptional returns whether this field is optional (field name existence).
//   - table formats (Excel/CSV): field's column can be absent.
//   - document formats (XML/YAML): field's name can be absent.
func (sp *sheetParser) IsFieldOptional(field *Field) bool {
	return sp.opts.GetOptional() || field.opts.GetProp().GetOptional()
}

func (sp *sheetParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	if sheet.Document != nil {
		docParser := &documentParser{sheetParser: sp}
		return docParser.Parse(protomsg, sheet)
	}
	tableParser := &tableParser{sheetParser: sp}
	return tableParser.Parse(protomsg, sheet)
}

func (sp *sheetParser) parseIncellMap(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
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
	splits := strings.Split(cellData, field.sep)
	size := len(splits)
	for i := 0; i < size; i++ {
		kv := strings.SplitN(splits[i], field.subsep, 2)
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
//	enum FruitType {
//		FRUIT_TYPE_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//		FRUIT_TYPE_APPLE   = 1 [(tableau.evalue).name = "Apple"];
//		FRUIT_TYPE_ORANGE  = 2 [(tableau.evalue).name = "Orange"];
//		FRUIT_TYPE_BANANA  = 3 [(tableau.evalue).name = "Banana"];
//	}
//	enum FruitFlavor {
//		FRUIT_FLAVOR_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//		FRUIT_FLAVOR_FRAGRANT = 1 [(tableau.evalue).name = "Fragrant"];
//		FRUIT_FLAVOR_SOUR = 2 [(tableau.evalue).name = "Sour"];
//		FRUIT_FLAVOR_SWEET = 3 [(tableau.evalue).name = "Sweet"];
//	}
//
//	map<int32, Fruit> fruit_map = 1 [(tableau.field) = {name:"Fruit" key:"Key" layout:LAYOUT_INCELL}];
//	message Fruit {
//		FruitType key = 1 [(tableau.field) = {name:"Key"}];
//		int64 value = 2 [(tableau.field) = {name:"Value"}];
//	}
//
//	map<int32, Item> item_map = 3 [(tableau.field) = {name:"Item" key:"Key" layout:LAYOUT_INCELL}];
//	message Item {
//		FruitType key = 1 [(tableau.field) = {name:"Key"}];
//		FruitFlavor value = 2 [(tableau.field) = {name:"Value"}];
//	}
func (sp *sheetParser) parseIncellMapWithValueAsSimpleKVMessage(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	if cellData == "" {
		return nil
	}
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, field.sep)
	size := len(splits)
	for i := 0; i < size; i++ {
		mapItemData := splits[i]
		kv := strings.SplitN(mapItemData, field.subsep, 2)
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
		valuePresent, err := sp.parseIncellStruct(newMapValue, mapItemData, field.opts.GetProp().GetForm(), field.subsep)
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
			childField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
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
			childField := parseFieldDescriptor(fd, sp.GetSep(), sp.GetSubsep())
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

func (sp *sheetParser) parseIncellListField(field *Field, list protoreflect.List, cellData string) (present bool, err error) {
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, field.sep)
	detectedSize := len(splits)
	fixedSize := prop.GetSize(field.opts.Prop, detectedSize)
	size := detectedSize
	if fixedSize > 0 && fixedSize < detectedSize {
		// squeeze to specified fixed size
		size = fixedSize
	}
	for i := 0; i < size; i++ {
		elem := splits[i]
		var (
			fieldValue  protoreflect.Value
			elemPresent bool
		)
		if field.fd.Kind() == protoreflect.MessageKind && !types.IsWellKnownMessage(string(field.fd.Message().FullName())) {
			fieldValue = list.NewElement()
			elemPresent, err = sp.parseIncellStruct(fieldValue, elem, field.opts.GetProp().GetForm(), field.subsep)
		} else {
			fieldValue, elemPresent, err = sp.parseFieldValue(field.fd, elem, field.opts.Prop)
		}
		if err != nil {
			return false, err
		}
		if !elemPresent && !prop.IsFixed(field.opts.Prop) {
			// TODO: check the remaining keys all not present, otherwise report error!
			break
		}
		if field.opts.Key != "" {
			// keyed list
			keyedListElemExisted := false
			for i := 0; i < list.Len(); i++ {
				elemVal := list.Get(i)
				if elemVal.Equal(fieldValue) {
					keyedListElemExisted = true
					break
				}
			}
			if !keyedListElemExisted {
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

func (sp *sheetParser) parseUnionMessageField(field *Field, msg protoreflect.Message, cellData string) error {
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
		present, err := sp.parseIncellStruct(value, cellData, field.opts.GetProp().GetForm(), field.sep)
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
