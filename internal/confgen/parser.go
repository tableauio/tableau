package confgen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/confgen/fieldprop"
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
		if info.SheetOpts.ScatterWithoutBookName {
			filename = sheetName
		} else {
			filename = fmt.Sprintf("%s_%s", impInfo.BookName(), sheetName)
		}
		if info.SheetOpts.WithParentDir {
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
	err = storeMessage(mainMsg, mainName, info.LocationName, x.OutputDir, x.OutputOpt)
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
			if info.SheetOpts.Patch == tableaupb.Patch_PATCH_MERGE {
				if info.ExtInfo.DryRun == options.DryRunPatch {
					clonedMainMsg := proto.Clone(mainMsg)
					err = xproto.PatchMessage(clonedMainMsg, msg)
					if err != nil {
						return err
					}
					msg = clonedMainMsg
				} else {
					return storePatchMergeMessage(msg, name, info.LocationName, x.OutputDir, x.OutputOpt)
				}
			}
			return storeMessage(msg, name, info.LocationName, x.OutputDir, x.OutputOpt)
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
		return xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf)
	}
	// exported conf name pattern is : [ParentDir/]<SheetName>
	getExportedConfName := func(info *SheetInfo, impInfo importer.ImporterInfo) string {
		// here filename has no ext suffix
		filename := string(info.MD.Name())
		if info.SheetOpts.WithParentDir {
			parentDirName := xfs.GetDirectParentDirName(impInfo.Filename())
			return filepath.Join(parentDirName, filename)
		}
		return filename
	}
	name := getExportedConfName(info, mainImpInfo)
	return storeMessage(protomsg, name, info.LocationName, x.OutputDir, x.OutputOpt)
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
			xerrors.KeyPrimaryBookName, info.PrimaryBookName,
			xerrors.KeySheetName, info.SheetOpts.Name,
			xerrors.KeyPBMessage, string(info.MD.Name()))
	} else if len(impInfos) == 1 {
		protomsg, err := parseMessageFromOneImporter(info, impInfos[0])
		if err != nil {
			return nil, err
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
			xerrors.KeyPrimaryBookName, info.PrimaryBookName,
			xerrors.KeyPrimarySheetName, info.SheetOpts.Name)
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
							xerrors.KeyBookName, bookNames,
							xerrors.KeySheetName, sheetNames,
							xerrors.KeyPBMessage, string(info.MD.Name()))
					}
				}
			}
			return nil, xerrors.WrapKV(err,
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
	parser := NewExtendedSheetParser(context.Background(), info.ProtoPackage, info.LocationName, info.BookOpts, info.SheetOpts, info.ExtInfo)
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
	BookOpts        *tableaupb.WorkbookOptions
	SheetOpts       *tableaupb.WorksheetOptions

	ExtInfo *SheetParserExtInfo
}

func (si *SheetInfo) HasScatter() bool {
	return si.SheetOpts != nil && len(si.SheetOpts.Scatter) != 0
}

func (si *SheetInfo) HasMerger() bool {
	return si.SheetOpts != nil && len(si.SheetOpts.Merger) != 0
}

type sheetParser struct {
	ProtoPackage string
	LocationName string
	ctx          context.Context
	bookOpts     *tableaupb.WorkbookOptions
	sheetOpts    *tableaupb.WorksheetOptions
	extInfo      *SheetParserExtInfo

	// cached names and types
	names       []string               // names[col] -> name
	types       []string               // types[col] -> name
	lookupTable book.ColumnLookupTable // column name -> column index (started with 0)

	// cached maps and lists with cardinality
	cards map[string]*cardInfo // map/list field card prefix -> cardInfo
}

type cardInfo struct {
	// option field name -> uniqueField
	uniqueFields map[string]*uniqueField
	// option field name -> sequenceField
	sequenceFields map[string]*sequenceField
}

type uniqueField struct {
	fd protoreflect.FieldDescriptor
	// value set: map[string]bool
	values   map[string]bool
	optional bool
}

type sequenceField struct {
	fd         protoreflect.FieldDescriptor
	sequence   int64
	valueCount int64 // value count in sequence
}

// SheetParserExtInfo is the extended info for refer check and so on.
type SheetParserExtInfo struct {
	InputDir       string
	SubdirRewrites map[string]string
	PRFiles        *protoregistry.Files
	BookFormat     format.Format // workbook format
	DryRun         options.DryRun
}

// NewSheetParser creates a new sheet parser.
func NewSheetParser(ctx context.Context, protoPackage, locationName string, opts *tableaupb.WorksheetOptions) *sheetParser {
	return NewExtendedSheetParser(ctx, protoPackage, locationName, &tableaupb.WorkbookOptions{}, opts, nil)
}

// NewExtendedSheetParser creates a new sheet parser with extended info.
func NewExtendedSheetParser(ctx context.Context, protoPackage, locationName string, bookOpts *tableaupb.WorkbookOptions, sheetOpts *tableaupb.WorksheetOptions, extInfo *SheetParserExtInfo) *sheetParser {
	sp := &sheetParser{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		ctx:          ctx,
		bookOpts:     bookOpts,
		sheetOpts:    sheetOpts,
		extInfo:      extInfo,
	}
	sp.reset()
	return sp
}

// reset resets the runtime data of sheet parser for reuse
func (p *sheetParser) reset() {
	p.names = nil
	p.types = nil
	p.lookupTable = book.ColumnLookupTable{}
	p.cards = map[string]*cardInfo{}
}

// GetSep returns sheet-level separator.
func (p *sheetParser) GetSep() string {
	// sheet-level
	if p.sheetOpts.Sep != "" {
		return p.sheetOpts.Sep
	}
	// book-level
	if p.bookOpts.Sep != "" {
		return p.bookOpts.Sep
	}
	// default
	return options.DefaultSep
}

// GetSubsep returns sheet-level subseparator.
func (p *sheetParser) GetSubsep() string {
	// sheet-level
	if p.sheetOpts.Subsep != "" {
		return p.sheetOpts.Subsep
	}
	// book-level
	if p.bookOpts.Subsep != "" {
		return p.bookOpts.Subsep
	}
	// default
	return options.DefaultSubsep
}

// GetBookFormat returns workbook format related to this sheet.
func (p *sheetParser) GetBookFormat() format.Format {
	if p.extInfo == nil {
		return format.UnknownFormat
	}
	return p.extInfo.BookFormat
}

// IsTable checks whether the sheet format is a table sheet.
func (p *sheetParser) IsTable() bool {
	return p.GetBookFormat() == format.Excel || p.GetBookFormat() == format.CSV
}

// IsFieldOptional returns whether this field is optional (field name existence).
//   - table formats (Excel/CSV): field's column can be absent.
//   - document formats (XML/YAML): field's name can be absent.
func (p *sheetParser) IsFieldOptional(field *Field) bool {
	return p.sheetOpts.GetOptional() || field.opts.GetProp().GetOptional()
}

func (p *sheetParser) Parse(protomsg proto.Message, sheet *book.Sheet) (err error) {
	defer func() {
		p.reset()
		if err != nil {
			err = xerrors.WrapKV(err, xerrors.KeyModule, xerrors.ModuleConf)
		}
	}()
	if sheet.Document != nil {
		docParser := &documentParser{sheetParser: p}
		return docParser.Parse(protomsg, sheet)
	}
	tableParser := &tableParser{sheetParser: p}
	return tableParser.Parse(protomsg, sheet)
}

func (p *sheetParser) parseIncellMap(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	if valueFd.Kind() != protoreflect.MessageKind {
		return p.parseIncellMapWithSimpleKV(field, reflectMap, cellData)
	}

	if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
		return xerrors.Errorf("map value type is not KV struct, and is not supported")
	}
	return p.parseIncellMapWithValueAsSimpleKVMessage(field, reflectMap, cellData)
}

// parseIncellMapWithSimpleKV parses simple incell map with key as scalar type and value as scalar or enum type.
// For example:
//   - map<int32, int32>
//   - map<int32, EnumType>
func (p *sheetParser) parseIncellMapWithSimpleKV(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
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

		fieldValue, keyPresent, err := p.parseFieldValue(keyFd, key, field.opts.Prop)
		if err != nil {
			return err
		}

		newMapKey := fieldValue.MapKey()
		if reflectMap.Has(newMapKey) {
			// incell map key must be unique
			return xerrors.WrapKV(xerrors.E2005(key))
		}
		// Currently, we cannot check scalar map value, so do not input field.opts.Prop.
		fieldValue, valuePresent, err := p.parseFieldValue(valueFd, value, nil)
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
func (p *sheetParser) parseIncellMapWithValueAsSimpleKVMessage(field *Field, reflectMap protoreflect.Map, cellData string) (err error) {
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

		newMapKey, keyPresent, err := p.parseMapKey(field, reflectMap, keyData)
		if err != nil {
			return err
		}
		if reflectMap.Has(newMapKey) {
			// incell map key must be unique
			return xerrors.WrapKV(xerrors.E2005(keyData))
		}

		newMapValue := reflectMap.NewValue()
		valuePresent, err := p.parseIncellStruct(field, newMapValue, mapItemData, field.subsep)
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

func (p *sheetParser) parseMapKey(field *Field, reflectMap protoreflect.Map, cellData string) (mapKey protoreflect.MapKey, present bool, err error) {
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
		fieldValue, present, err = p.parseFieldValue(keyFd, cellData, field.opts.Prop)
		if err != nil {
			return mapKey, false, err
		}
		v := protoreflect.ValueOfInt32(int32(fieldValue.Enum()))
		mapKey = v.MapKey()
	} else {
		fieldValue, present, err = p.parseFieldValue(keyFd, cellData, field.opts.Prop)
		if err != nil {
			return mapKey, false, xerrors.WrapKV(err)
		}
		mapKey = fieldValue.MapKey()
	}
	return mapKey, present, nil
}

func (p *sheetParser) checkMapKeyUnique(field *Field, keyData string) error {
	return p.checkKeyUnique(field.fd.MapValue().Message(), field.opts, keyData)
}

func (p *sheetParser) checkListKeyUnique(field *Field, keyData string) error {
	return p.checkKeyUnique(field.fd.Message(), field.opts, keyData)
}

func (p *sheetParser) checkKeyUnique(md protoreflect.MessageDescriptor, fdOpts *tableaupb.FieldOptions, keyData string) error {
	fd := p.findFieldByName(md, fdOpts.Key)
	if fd == nil {
		return xerrors.Errorf(fmt.Sprintf("key field not found in proto definition: %s", fdOpts.Key))
	}
	keyField := p.parseFieldDescriptor(fd)
	defer keyField.release()
	if fieldprop.RequireUnique(keyField.opts.Prop) ||
		(!fieldprop.HasUnique(keyField.opts.Prop) && p.deduceKeyUnique(fdOpts.Layout, md)) {
		return xerrors.E2005(keyData)
	}
	return nil
}

// deduceKeyUnique deduces whether the Map/KeyedList key is unique or not.
//
// # Document sheet
//
// Key must be unique.
//
// # Table sheet
//
// By default, the key should be unique. However, in order to aggregate
// sub-field (map or list) with cardinality, the key can be duplicate.
//
// It should be deduced to be unique if nesting hierarchy is like:
//   - Map/KeyedList layout is incell.
//   - Map/KeyedList nesting map or list with different layout (vertical or horizontal).
//   - Map/KeyedList nesting no map or list.
func (p *sheetParser) deduceKeyUnique(fieldLayout tableaupb.Layout, md protoreflect.MessageDescriptor) bool {
	if !p.IsTable() {
		return true
	}
	layout := parseTableMapLayout(fieldLayout)
	if layout == tableaupb.Layout_LAYOUT_INCELL {
		// incell layout must be unique
		return true
	}
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.IsMap() || fd.IsList() {
			childField := p.parseFieldDescriptor(fd)
			defer childField.release()
			childLayout := parseTableMapLayout(childField.opts.Layout)
			if childLayout == layout {
				// same layout (vertical/horizontal), the key can be duplicate
				// to aggregate sub-field (map or list) with cardinality.
				return false
			}
		}
	}
	return true
}

func (p *sheetParser) checkMapKeySequence(field *Field, reflectMap protoreflect.Map, keyData string, merged bool) error {
	return p.checkKeySequence(field.fd.MapValue().Message(), field.opts, keyData, int64(reflectMap.Len()), merged)
}

func (p *sheetParser) checkListKeySequence(field *Field, reflectList protoreflect.List, keyData string, merged bool) error {
	return p.checkKeySequence(field.fd.Message(), field.opts, keyData, int64(reflectList.Len()), merged)
}

func (p *sheetParser) checkKeySequence(md protoreflect.MessageDescriptor, fdOpts *tableaupb.FieldOptions, keyData string, valueCount int64, merged bool) error {
	fd := p.findFieldByName(md, fdOpts.Key)
	if fd == nil {
		return xerrors.Errorf(fmt.Sprintf("key field not found in proto definition: %s", fdOpts.Key))
	}
	keyField := p.parseFieldDescriptor(fd)
	defer keyField.release()
	if !fieldprop.RequireSequence(keyField.opts.Prop) {
		// do not require sequence
		return nil
	}
	seq := keyField.opts.Prop.GetSequence()
	val, err := strconv.ParseInt(keyData, 10, 64)
	if err != nil {
		return xerrors.Wrapf(err, "failed to parse integer: %q", keyData)
	}
	if merged && val == seq+valueCount-1 {
		return nil
	}
	if !merged && val == seq+valueCount {
		return nil
	}
	return xerrors.E2003(keyData, keyField.opts.Prop.GetSequence())
}

// checkSubFieldProp checks whether the map value's or list element's sub-field value
// meets its unique or sequence conditions. The uniqueness check ignores optional
// fields with default value.
//
// If an error occured, it will return the field option name which fails the condition.
//
// # Performance improvement
//
//  1. parse sub fields metadata only once, and cache it for later use.
//  2. cache the field's values in map to speed up checking.
func (p *sheetParser) checkSubFieldProp(field *Field, cardPrefix string, newValue protoreflect.Value) (string, error) {
	if field.fd.IsMap() {
		if field.fd.MapValue().Kind() != protoreflect.MessageKind || xproto.IsUnionField(field.fd.MapValue()) {
			// no need to check
			return "", nil
		}
	} else if field.fd.IsList() {
		if field.fd.Kind() != protoreflect.MessageKind || xproto.IsUnionField(field.fd) {
			// no need to check
			return "", nil
		}
	} else {
		return "", xerrors.Errorf("field %s is not map or list", field.fd.FullName())
	}
	info := p.cards[cardPrefix]
	if info == nil {
		// parse sub fields metadata only once, and cache it for later use
		md := newValue.Message().Descriptor()
		info = &cardInfo{
			uniqueFields:   map[string]*uniqueField{},
			sequenceFields: map[string]*sequenceField{},
		}
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			subField := p.parseFieldDescriptor(fd)
			subField.mergeParentFieldProp(field)
			defer subField.release()
			if subField.opts.GetName() == field.opts.GetKey() {
				// key field not checked
				continue
			}
			if fieldprop.RequireUnique(subField.opts.Prop) {
				info.uniqueFields[subField.opts.GetName()] = &uniqueField{
					fd:       subField.fd,
					values:   map[string]bool{},
					optional: p.IsFieldOptional(subField),
				}
			}
			if fieldprop.RequireSequence(subField.opts.Prop) {
				info.sequenceFields[subField.opts.GetName()] = &sequenceField{
					fd:       subField.fd,
					sequence: subField.opts.Prop.GetSequence(),
				}
			}
		}
		// add new unique value
		p.cards[cardPrefix] = info
	}
	for name, field := range info.uniqueFields {
		if field.optional && !newValue.Message().Has(field.fd) {
			// ignore optional fields with default value
			continue
		}
		valStr := newValue.Message().Get(field.fd).String()
		if field.values[valStr] {
			return name, xerrors.E2022(string(field.fd.Name()), valStr)
		}
		// add new unique value
		field.values[valStr] = true
	}
	for name, field := range info.sequenceFields {
		valStr := newValue.Message().Get(field.fd).String()
		val, err := strconv.ParseInt(newValue.Message().Get(field.fd).String(), 10, 64)
		if err != nil {
			return "", xerrors.Wrapf(err, "failed to parse integer: %q", valStr)
		}
		if val != field.sequence+field.valueCount {
			return name, xerrors.E2003(valStr, field.sequence)
		}
		// increase value count in sequence
		field.valueCount++
	}
	return "", nil
}

func (p *sheetParser) parseIncellList(field *Field, list protoreflect.List, cardPrefix string, elemData string) (present bool, err error) {
	splits := strings.Split(elemData, field.sep)
	return p.parseListElems(field, list, cardPrefix, field.subsep, splits)
}

// parseListElems parses the given string slice to a list. Each elem's
// corresponding type can be: scalar, enum, well-known, and struct.
func (p *sheetParser) parseListElems(field *Field, list protoreflect.List, cardPrefix string, sep string, elemDataList []string) (present bool, err error) {
	fd := field.fd
	fdopts := field.opts
	detectedSize := len(elemDataList)
	fixedSize := fieldprop.GetSize(fdopts.Prop, detectedSize)
	size := detectedSize
	if fixedSize > 0 && fixedSize < detectedSize {
		// squeeze to specified fixed size
		size = fixedSize
	}
	var firstNonePresentIndex int
	for i := 1; i <= size; i++ {
		elem := elemDataList[i-1]
		var elemValue protoreflect.Value
		var elemPresent bool
		if fd.Kind() == protoreflect.MessageKind && !types.IsWellKnownMessage(fd.Message().FullName()) {
			elemValue = list.NewElement()
			elemPresent, err = p.parseIncellStruct(field, elemValue, elem, sep)
		} else {
			elemValue, elemPresent, err = p.parseFieldValue(fd, elem, fdopts.Prop)
		}
		if err != nil {
			return false, err
		}
		if firstNonePresentIndex != 0 {
			// Check that no empty elements are existed in begin or middle.
			// Guarantee all the remaining elements are not present,
			// otherwise report error!
			if elemPresent {
				return false, xerrors.E2016(firstNonePresentIndex, i)
			}
			continue
		}
		if !elemPresent && !fieldprop.IsFixed(fdopts.Prop) {
			firstNonePresentIndex = i
			continue
		}
		if fdopts.GetKey() != "" && fd.Kind() != protoreflect.MessageKind {
			// check key unique for scalar/enum list
			for j := 0; j < list.Len(); j++ {
				elemVal := list.Get(j)
				if elemVal.Equal(elemValue) {
					return false, xerrors.WrapKV(xerrors.E2005(elemValue))
				}
			}
		}
		// check list elem's sub-field prop
		_, err := p.checkSubFieldProp(field, cardPrefix, elemValue)
		if err != nil {
			return false, err
		}
		list.Append(elemValue)
	}
	if fieldprop.IsFixed(fdopts.Prop) {
		for list.Len() < fixedSize {
			// append empty elements to the specified length.
			list.Append(list.NewElement())
		}
	}
	return list.Len() != 0, nil
}

func (p *sheetParser) parseIncellStruct(field *Field, structValue protoreflect.Value, cellData string, sep string) (present bool, err error) {
	if cellData == "" {
		return false, nil
	}
	switch field.opts.GetProp().GetForm() {
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
		splits := strings.Split(cellData, sep)
		md := structValue.Message().Descriptor()
		for i := 0; i < md.Fields().Len() && i < len(splits); i++ {
			fd := md.Fields().Get(i)
			rawValue := splits[i]
			err := func() error {
				subField := p.parseFieldDescriptor(fd)
				subField.mergeParentFieldProp(field)
				defer subField.release()
				// log.Debugf("fd.FullName().Name(): ", fd.FullName().Name())
				value, fieldPresent, err := p.parseFieldValue(fd, rawValue, subField.opts.Prop)
				if err != nil {
					return err
				}
				if fieldPresent {
					// The struct is treated as present as long as one field is present.
					present = true
					structValue.Message().Set(fd, value)
				}
				return nil
			}()
			if err != nil {
				return false, err
			}
		}
		return present, nil
	}
}

func (p *sheetParser) parseUnionMessageField(field *Field, msg protoreflect.Message, cardPrefix string, dataList []string) (err error) {
	if len(dataList) == 0 {
		return xerrors.Errorf("union field data not provided")
	}
	var present bool
	var fieldValue protoreflect.Value
	if field.fd.IsMap() {
		// incell map
		fieldValue = msg.NewField(field.fd)
		err := p.parseIncellMap(field, fieldValue.Map(), dataList[0])
		if err != nil {
			return err
		}
		if !msg.Has(field.fd) && fieldValue.Map().Len() != 0 {
			present = true
		}
	} else if field.fd.IsList() {
		// incell list
		fieldValue = msg.NewField(field.fd)
		list := fieldValue.List()
		switch field.opts.GetLayout() {
		case tableaupb.Layout_LAYOUT_INCELL, tableaupb.Layout_LAYOUT_DEFAULT:
			present, err = p.parseIncellList(field, list, cardPrefix, dataList[0])
		case tableaupb.Layout_LAYOUT_HORIZONTAL:
			present, err = p.parseListElems(field, list, cardPrefix, field.sep, dataList)
		default:
			return xerrors.Errorf("union list field has illegal layout: %s", field.opts.GetLayout())
		}
	} else if field.fd.Kind() == protoreflect.MessageKind {
		if types.IsWellKnownMessage(field.fd.Message().FullName()) {
			// well-known message
			fieldValue, present, err = p.parseFieldValue(field.fd, dataList[0], field.opts.Prop)
		} else {
			// incell struct
			fieldValue = msg.NewField(field.fd)
			present, err = p.parseIncellStruct(field, fieldValue, dataList[0], field.sep)
		}
	} else {
		fieldValue, present, err = p.parseFieldValue(field.fd, dataList[0], field.opts.Prop)
	}
	if err != nil {
		return err
	}
	if present {
		msg.Set(field.fd, fieldValue)
	}
	return nil
}

func (p *sheetParser) parseIncellUnion(structValue protoreflect.Value, cellData string, form tableaupb.Form) (present bool, err error) {
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

// parseFieldValue parses field value by [protoreflect.FieldDescriptor] and
// [tableaupb.FieldProp]. It can parse following basic types:
//   - Scalar types
//   - Enum types
//   - Well-known types
func (p *sheetParser) parseFieldValue(fd protoreflect.FieldDescriptor, rawValue string, fprop *tableaupb.FieldProp) (v protoreflect.Value, present bool, err error) {
	v, present, err = xproto.ParseFieldValue(fd, rawValue, p.LocationName, fprop)
	if err != nil {
		return v, present, err
	}

	if fprop != nil {
		// check presence
		if err := fieldprop.CheckPresence(fprop, present); err != nil {
			return v, present, err
		}
		// check range
		if err := fieldprop.CheckInRange(fprop, fd, v, present); err != nil {
			return v, present, err
		}
		// check refer
		// NOTE: if use NewSheetParser, sp.extInfo is nil, which means SheetParserExtInfo is not provided.
		if fprop.Refer != "" && p.extInfo != nil {
			input := &fieldprop.Input{
				ProtoPackage:   p.ProtoPackage,
				InputDir:       p.extInfo.InputDir,
				SubdirRewrites: p.extInfo.SubdirRewrites,
				PRFiles:        p.extInfo.PRFiles,
				Present:        present,
			}
			ok, err := fieldprop.InReferredSpace(p.ctx, fprop, rawValue, input)
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

func (p *sheetParser) findFieldByName(md protoreflect.MessageDescriptor, name string) protoreflect.FieldDescriptor {
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		field := p.parseFieldDescriptor(fd)
		defer field.release()
		if field.opts.Name == name {
			return fd
		}
	}
	return nil
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
	wsOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	// log.Debugf("msg: %v, wsOpts: %+v", md.Name(), wsOpts)
	return string(md.Name()), wsOpts
}
