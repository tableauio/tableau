package prop

import (
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var referRegexp *regexp.Regexp

func init() {
	// e.g.:
	// - Item(ItemConf).ID
	// - Item-(Award)(ItemConf).ID
	referRegexp = regexp.MustCompile(`(?P<Sheet>.+?)` + `(\((?P<Alias>\w+)\))?` + `\.` + `(?P<Column>\w+)`)

	referredCache = NewReferredCache()
}

var referredCache *ReferredCache

type ReferredCache struct {
	sync.RWMutex
	references map[string]*ValueSpace // message name -> sheet column value space
}

type ValueSpace struct {
	*hashset.Set
}

func NewValueSpace() *ValueSpace {
	return &ValueSpace{
		Set: hashset.New(),
	}
}

func NewReferredCache() *ReferredCache {
	return &ReferredCache{
		references: make(map[string]*ValueSpace),
	}
}

func (r *ReferredCache) Exists(refer string) bool {
	r.RLock()
	defer r.RUnlock()
	_, ok := r.references[refer]
	return ok
}

type loadValueSpaceFunc = func() (*ValueSpace, error)

func (r *ReferredCache) ExistsValue(refer string, value string, loadFunc loadValueSpaceFunc) (bool, error) {
	r.RLock()
	valueSpace, ok := r.references[refer]
	r.RUnlock()
	if ok {
		return valueSpace.Contains(value), nil
	}

	// load value space once
	r.Lock()
	defer r.Unlock()
	valueSpace, ok = r.references[refer]
	if ok {
		return valueSpace.Contains(value), nil
	}
	valueSpace, err := loadFunc()
	if err != nil {
		return false, err
	}
	r.references[refer] = valueSpace
	return valueSpace.Contains(value), nil
}

func (r *ReferredCache) Put(refer string, valueSpace *ValueSpace) {
	r.Lock()
	defer r.Unlock()
	r.references[refer] = valueSpace
}

type ReferDesc struct {
	Sheet  string // sheet name in workbook.
	Alias  string // sheet alias: if set, used as protobuf message name.
	Column string // sheet column name in name row.
}

func (r *ReferDesc) GetMessageName() string {
	if r.Alias != "" {
		return r.Alias
	}
	return r.Sheet
}

func parseRefer(text string) (*ReferDesc, error) {
	match := referRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil, xerrors.Errorf("invalid refer pattern: %s", text)
	}
	desc := &ReferDesc{}
	for i, name := range referRegexp.SubexpNames() {
		value := strings.TrimSpace(match[i])
		switch name {
		case "Sheet":
			desc.Sheet = value
		case "Alias":
			desc.Alias = value
		case "Column":
			desc.Column = value
		}
	}
	return desc, nil
}

type Input struct {
	ProtoPackage   string
	InputDir       string
	SubdirRewrites map[string]string
	PRFiles        *protoregistry.Files
	Present        bool // field presence
}

func loadValueSpace(refer string, input *Input) (*ValueSpace, error) {
	referInfo, err := parseRefer(refer)
	if err != nil {
		return nil, err
	}
	fullName := protoreflect.FullName(input.ProtoPackage + "." + referInfo.GetMessageName())
	desc, err := input.PRFiles.FindDescriptorByName(fullName)
	if err != nil {
		return nil, xerrors.E2001(refer, referInfo.GetMessageName())
	}

	// get workbook name and worksheet name
	fileOpts := desc.ParentFile().Options().(*descriptorpb.FileOptions)
	bookOpts := proto.GetExtension(fileOpts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	bookName := bookOpts.Name

	msgOpts := desc.Options().(*descriptorpb.MessageOptions)
	sheetOpts := proto.GetExtension(msgOpts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	sheetName := sheetOpts.Name

	// rewrite subdir
	rewrittenWorkbookName := fs.RewriteSubdir(bookName, input.SubdirRewrites)
	absWbPath := filepath.Join(input.InputDir, rewrittenWorkbookName)
	primaryImporter, err := importer.New(absWbPath, importer.Sheets([]string{sheetName}))
	if err != nil {
		return nil, xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, bookName)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(input.InputDir, rewrittenWorkbookName, sheetName, sheetOpts.Merger, input.SubdirRewrites)
	if err != nil {
		return nil, xerrors.WithMessageKV(err, xerrors.KeyModule, xerrors.ModuleConf, xerrors.KeyBookName, bookName)
	}

	// append self
	impInfos = append(impInfos, importer.ImporterInfo{Importer: primaryImporter})

	// new empty referred value space set
	valueSpace := NewValueSpace()
	for _, impInfo := range impInfos {
		specifiedSheetName := sheetName
		if impInfo.SpecifiedSheetName != "" {
			// sheet name is specified
			specifiedSheetName = impInfo.SpecifiedSheetName
		}
		sheet := impInfo.GetSheet(specifiedSheetName)
		if sheet == nil {
			err := xerrors.E0001(sheetName, impInfo.Filename())
			return nil, xerrors.WithMessageKV(err, xerrors.KeySheetName, sheetName, xerrors.KeyBookName, impInfo.Filename())
		}

		if sheetOpts.Transpose {
			// TODO: transpose
		} else {
			foundColumn := -1
			nameRow := int(sheetOpts.Namerow) - 1
			for col := 0; col < sheet.MaxCol; col++ {
				nameCell, err := sheet.Cell(nameRow, col)
				if err != nil {
					return nil, xerrors.WrapKV(err)
				}
				name := book.ExtractFromCell(nameCell, sheetOpts.Nameline)
				if name == referInfo.Column {
					foundColumn = col
					break
				}
			}
			if foundColumn < 0 {
				return nil, xerrors.E2015(referInfo.Column, bookName, sheetName)
			}
			for row := int(sheetOpts.Datarow) - 1; row < sheet.MaxRow; row++ {
				data, err := sheet.Cell(row, foundColumn)
				if err != nil {
					return nil, xerrors.WrapKV(err)
				}
				valueSpace.Add(data)
			}
		}
	}

	return valueSpace, nil
}

func InReferredSpace(prop *tableaupb.FieldProp, cellData string, input *Input) (bool, error) {
	if prop == nil || strings.TrimSpace(prop.Refer) == "" {
		return true, nil
	}
	// not present, and presence not required
	if !input.Present && !prop.Present {
		return true, nil
	}

	loadFunc := func() (*ValueSpace, error) {
		return loadValueSpace(prop.Refer, input)
	}
	ok, err := referredCache.ExistsValue(prop.Refer, cellData, loadFunc)
	if err != nil {
		return false, err
	}
	return ok, nil
}
