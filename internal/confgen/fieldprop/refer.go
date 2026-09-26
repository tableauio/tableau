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

// ReferredCache caches referred column values for one generation or load.
type ReferredCache struct {
	mu      sync.Mutex
	entries map[string]*referCacheLoad // refer expression -> cached or in-flight result
}

// referCacheEntry holds a value space or a previously reported load failure.
type referCacheEntry struct {
	space       *valueSpace
	unavailable bool // target load failed
}

type referCacheLoad struct {
	ready chan struct{}
	entry referCacheEntry
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
		// Keep the referred location distinct from the source sheet.
		return xerrors.WrapKV(err,
			xerrors.KeyReferBookName, bookName,
			xerrors.KeyReferSheetName, sheetName,
		)
	}
	return nil
}

func NewReferredCache() *ReferredCache {
	return &ReferredCache{
		entries: make(map[string]*referCacheLoad),
	}
}

type loadValueSpaceFunc = func() (*valueSpace, error)

func (r *ReferredCache) getEntry(refer string, loadFunc loadValueSpaceFunc) (referCacheEntry, error) {
	r.mu.Lock()
	load, ok := r.entries[refer]
	if ok {
		r.mu.Unlock()
		<-load.ready
		return load.entry, nil
	}
	load = &referCacheLoad{ready: make(chan struct{})}
	r.entries[refer] = load
	r.mu.Unlock()

	space, err := loadFunc()
	entry := referCacheEntry{space: space}
	if err != nil {
		// Return the first load error and remember it to suppress repeats.
		entry = referCacheEntry{unavailable: true}
	}

	r.mu.Lock()
	load.entry = entry
	close(load.ready)
	r.mu.Unlock()
	return entry, err
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

// Input is the workbook lookup context for CheckRefer.
type Input struct {
	ProtoPackage   string
	InputDir       string
	SubdirRewrites map[string]string
	PRFiles        *protoregistry.Files
	ImporterCache  *importer.Cache
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
	load := importer.New
	if input.ImporterCache != nil {
		load = input.ImporterCache.Load
	}
	primaryImporter, err := load(ctx, absWbPath, importer.Sheets([]string{sheetName}))
	if err != nil {
		return nil, xerrors.WrapKV(err,
			xerrors.KeyReferBookName, bookName,
			xerrors.KeyReferSheetName, sheetName,
		)
	}

	// get merger importer infos
	var impInfos []importer.ImporterInfo
	if input.ImporterCache == nil {
		impInfos, err = importer.GetMergerImporters(ctx, input.InputDir, rewrittenWorkbookName, sheetName, sheetOpts.Merger, input.SubdirRewrites)
	} else {
		impInfos, err = input.ImporterCache.LoadMergerImporters(ctx, input.InputDir, rewrittenWorkbookName, sheetName, sheetOpts.Merger, input.SubdirRewrites)
	}
	if err != nil {
		return nil, xerrors.WrapKV(err,
			xerrors.KeyReferBookName, bookName,
			xerrors.KeyReferSheetName, sheetName,
		)
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
		referredBookName := impInfo.Filename()
		if relBookName, err := xfs.Rel(input.InputDir, referredBookName); err == nil {
			referredBookName = relBookName
		}
		sheet := impInfo.GetSheet(specifiedSheetName)
		if sheet == nil {
			return nil, xerrors.E2030(referredBookName, specifiedSheetName)
		}

		if sheetOpts.Transpose {
			err = space.addFromTable(header, sheet.Table.Transpose(), referInfo.Column, referredBookName, specifiedSheetName)
		} else {
			err = space.addFromTable(header, sheet.Table, referInfo.Column, referredBookName, specifiedSheetName)
		}
		if err != nil {
			return nil, err
		}
	}

	return space, nil
}

// CheckRefer validates cellData against prop.Refer.
func (r *ReferredCache) CheckRefer(ctx context.Context, prop *tableaupb.FieldProp, cellData string, input *Input) error {
	if prop == nil || strings.TrimSpace(prop.Refer) == "" {
		return nil
	}
	// not present, and presence not required
	if !input.Present && !prop.Present {
		return nil
	}
	if r == nil {
		return xerrors.New("referred cache is nil")
	}

	for _, refer := range strings.Split(prop.Refer, ",") {
		entry, err := r.getEntry(refer, func() (*valueSpace, error) {
			return loadValueSpace(ctx, refer, input)
		})
		if err != nil {
			return err
		}
		if entry.unavailable {
			// The first lookup already returned the load error.
			return nil
		}
		if entry.space.Contains(cellData) {
			return nil
		}
	}
	return xerrors.E2002(cellData, prop.Refer)
}
