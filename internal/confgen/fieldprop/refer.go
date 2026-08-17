package fieldprop

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/proto/tableaupb"
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
}

// ReferredCache holds refer target column value spaces for one generate/load run.
type ReferredCache struct {
	mu         sync.RWMutex
	references map[string]*valueSpace // message name -> sheet column value space
	// failed records refer expressions whose value space failed to load.
	// The first failure is still surfaced to the caller; subsequent
	// InReferredSpace calls for the same refer short-circuit to "present" so
	// one broken refer target does not spam N duplicate errors (one per
	// row of the referring sheet).
	failed map[string]struct{}
}

type valueSpace struct {
	*hashset.Set
}

func newValueSpace() *valueSpace {
	return &valueSpace{
		Set: hashset.New(),
	}
}

func (v *valueSpace) addFromTable(header *tableparser.Header, table book.Tabler, columnName, bookName, sheetName string) error {
	err := tableparser.RangeDataRows(table, header, func(r *book.Row) error {
		cell, err := r.Cell(columnName, false)
		if err != nil {
			return xerrors.E2015(columnName, bookName, sheetName)
		}
		v.Add(cell.Data)
		return nil
	})
	if err != nil {
		// Tag with the referred target so outer confgen wrappers (which set
		// BookName/SheetName to the source sheet under generation) don't
		// obscure where the offending row actually lives.
		return xerrors.WrapKV(err,
			xerrors.KeyReferBookName, bookName,
			xerrors.KeyReferSheetName, sheetName,
		)
	}
	return nil
}

func NewReferredCache() *ReferredCache {
	return &ReferredCache{
		references: make(map[string]*valueSpace),
		failed:     make(map[string]struct{}),
	}
}

func (r *ReferredCache) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.references = make(map[string]*valueSpace)
	r.failed = make(map[string]struct{})
}

func (r *ReferredCache) exists(refer string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.references[refer]
	return ok
}

type loadValueSpaceFunc = func(refer string) (*valueSpace, error)

func (r *ReferredCache) existsValue(refer string, value string, loadFunc loadValueSpaceFunc) (bool, error) {
	r.mu.RLock()
	space, ok := r.references[refer]
	_, isFailed := r.failed[refer]
	r.mu.RUnlock()
	if isFailed {
		// A prior load surfaced the real error already; silence follow-ups
		// on the same refer target.
		return true, nil
	}
	if ok {
		return space.Contains(value), nil
	}

	// load value space once
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, isFailed = r.failed[refer]; isFailed {
		return true, nil
	}
	space, ok = r.references[refer]
	if ok {
		return space.Contains(value), nil
	}
	space, err := loadFunc(refer)
	if err != nil {
		// Remember the failure so future callers short-circuit instead of
		// re-triggering loadFunc (and re-emitting the same error).
		r.failed[refer] = struct{}{}
		return false, err
	}
	r.references[refer] = space
	return space.Contains(value), nil
}

func (r *ReferredCache) put(refer string, space *valueSpace) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.references[refer] = space
}

type referDesc struct {
	Sheet  string // sheet name in workbook.
	Alias  string // sheet alias: if set, used as protobuf message name.
	Column string // sheet column name in name row.
}

func (d *referDesc) getMessageName() string {
	if d.Alias != "" {
		return d.Alias
	}
	return d.Sheet
}

func parseRefer(text string) (*referDesc, error) {
	match := referRegexp.FindStringSubmatch(text)
	if match == nil {
		return nil, xerrors.Newf("invalid refer pattern: %s", text)
	}
	desc := &referDesc{}
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

// Input is the workbook lookup context for InReferredSpace.
type Input struct {
	ProtoPackage   string
	InputDir       string
	SubdirRewrites map[string]string
	PRFiles        *protoregistry.Files
	Present        bool // field presence
}

func loadValueSpace(ctx context.Context, refer string, input *Input) (*valueSpace, error) {
	referInfo, err := parseRefer(refer)
	if err != nil {
		return nil, err
	}
	fullName := protoreflect.FullName(input.ProtoPackage + "." + referInfo.getMessageName())
	desc, err := input.PRFiles.FindDescriptorByName(fullName)
	if err != nil {
		return nil, xerrors.E2001(refer, referInfo.getMessageName())
	}

	// get workbook name and worksheet name
	fileOpts := desc.ParentFile().Options().(*descriptorpb.FileOptions)
	bookOpts := proto.GetExtension(fileOpts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	bookName := bookOpts.Name

	msgOpts := desc.Options().(*descriptorpb.MessageOptions)
	sheetOpts := proto.GetExtension(msgOpts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	sheetName := sheetOpts.Name

	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(bookName, input.SubdirRewrites)
	absWbPath := filepath.Join(input.InputDir, rewrittenWorkbookName)
	primaryImporter, err := importer.New(ctx, absWbPath, importer.Sheets([]string{sheetName}))
	if err != nil {
		return nil, xerrors.WrapKV(err, xerrors.KeyBookName, bookName)
	}

	// get merger importer infos
	impInfos, err := importer.GetMergerImporters(ctx, input.InputDir, rewrittenWorkbookName, sheetName, sheetOpts.Merger, input.SubdirRewrites)
	if err != nil {
		return nil, xerrors.WrapKV(err, xerrors.KeyBookName, bookName)
	}

	// append self
	impInfos = append(impInfos, importer.ImporterInfo{Importer: primaryImporter})
	header := tableparser.NewHeader(sheetOpts, bookOpts, nil)
	// new empty referred value space set
	space := newValueSpace()
	for _, impInfo := range impInfos {
		specifiedSheetName := sheetName
		if impInfo.SpecifiedSheetName != "" {
			// sheet name is specified
			specifiedSheetName = impInfo.SpecifiedSheetName
		}
		sheet := impInfo.GetSheet(specifiedSheetName)
		if sheet == nil {
			err := xerrors.E0001(sheetName, impInfo.Filename())
			return nil, xerrors.WrapKV(err, xerrors.KeySheetName, sheetName, xerrors.KeyBookName, impInfo.Filename())
		}

		if sheetOpts.Transpose {
			err = space.addFromTable(header, sheet.Table.Transpose(), referInfo.Column, bookName, sheetName)
		} else {
			err = space.addFromTable(header, sheet.Table, referInfo.Column, bookName, sheetName)
		}
		if err != nil {
			return nil, err
		}
	}

	return space, nil
}

// InReferredSpace checks whether the cell data is at least in one of the other sheets'
// column value space (aka message's field value space). prop.Refer is comma separated,
// e.g.: "SheetName(SheetAlias).ColumnName[,SheetName(SheetAlias).ColumnName]..."
func (r *ReferredCache) InReferredSpace(ctx context.Context, prop *tableaupb.FieldProp, cellData string, input *Input) (bool, error) {
	if prop == nil || strings.TrimSpace(prop.Refer) == "" {
		return true, nil
	}
	// not present, and presence not required
	if !input.Present && !prop.Present {
		return true, nil
	}
	if r == nil {
		r = NewReferredCache()
	}

	loadFunc := func(refer string) (*valueSpace, error) {
		return loadValueSpace(ctx, refer, input)
	}

	// NOTE: prop.Refer is comma separated, e.g.: "SheetName(SheetAlias).ColumnName[,SheetName(SheetAlias).ColumnName]..."
	for _, refer := range strings.Split(prop.Refer, ",") {
		ok, err := r.existsValue(refer, cellData, loadFunc)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
