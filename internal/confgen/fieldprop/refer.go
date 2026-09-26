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
	mu sync.RWMutex
	// Entries are pointers because a loader publishes into the same in-flight
	// entry that waiters obtained before the load completed.
	entries map[referCacheKey]*referCacheEntry

	targetsMu sync.RWMutex
	// targets avoids splitting and parsing the same field refer for every cell.
	targets map[referTargetsKey]referTargets
}

// referCacheKey identifies one referred column. Its canonical string form is
// <fully-qualified-message-name>.<column-name>.
type referCacheKey struct {
	message protoreflect.FullName
	column  string
}

func (k referCacheKey) String() string {
	return string(k.message) + "." + k.column
}

// referCacheEntry holds an in-flight or completed reference load. Closing ready
// publishes space or unavailable to every waiter.
type referCacheEntry struct {
	ready       chan struct{}
	space       *valueSpace
	unavailable bool // target load failed
}

type referTargets struct {
	values []*referTarget
	err    error
}

type referTargetsKey struct {
	protoPackage string
	refer        string
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
		entries: make(map[referCacheKey]*referCacheEntry),
		targets: make(map[referTargetsKey]referTargets),
	}
}

type loadValueSpaceFunc = func() (*valueSpace, error)

// getEntry loads each normalized message column once. Callers for the same key
// wait for its ready channel, while unrelated targets load concurrently.
func (r *ReferredCache) getEntry(key referCacheKey, loadFunc loadValueSpaceFunc) (referCacheEntry, error) {
	r.mu.RLock()
	entry, ok := r.entries[key]
	r.mu.RUnlock()
	if ok {
		return waitForReferEntry(entry), nil
	}

	r.mu.Lock()
	entry, ok = r.entries[key]
	if ok {
		r.mu.Unlock()
		return waitForReferEntry(entry), nil
	}
	entry = &referCacheEntry{ready: make(chan struct{})}
	r.entries[key] = entry
	r.mu.Unlock()

	space, err := loadFunc()
	r.mu.Lock()
	if err == nil {
		entry.space = space
	} else {
		// Only the loader reports the error. Waiters observe unavailable and skip
		// duplicate errors for the same broken reference.
		entry.unavailable = true
	}
	close(entry.ready)
	r.mu.Unlock()
	return *entry, err
}

func waitForReferEntry(entry *referCacheEntry) referCacheEntry {
	<-entry.ready
	return *entry
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

type referTarget struct {
	refer    string
	fullName protoreflect.FullName
	column   string
}

func (t *referTarget) key() referCacheKey {
	return referCacheKey{message: t.fullName, column: t.column}
}

func normalizeRefer(refer string, input *Input) (*referTarget, error) {
	referInfo, err := parseRefer(refer)
	if err != nil {
		return nil, err
	}
	messageName := referInfo.getMessageName()
	return &referTarget{
		refer:    refer,
		fullName: protoreflect.FullName(input.ProtoPackage + "." + messageName),
		column:   referInfo.Column,
	}, nil
}

func (r *ReferredCache) getTargets(refer string, input *Input) ([]*referTarget, error) {
	key := referTargetsKey{protoPackage: input.ProtoPackage, refer: refer}
	r.targetsMu.RLock()
	targets, ok := r.targets[key]
	r.targetsMu.RUnlock()
	if ok {
		return targets.values, targets.err
	}

	r.targetsMu.Lock()
	defer r.targetsMu.Unlock()
	targets, ok = r.targets[key]
	if ok {
		return targets.values, targets.err
	}
	for _, item := range strings.Split(refer, ",") {
		target, err := normalizeRefer(item, input)
		if err != nil {
			targets.err = err
			break
		}
		targets.values = append(targets.values, target)
	}
	r.targets[key] = targets
	return targets.values, targets.err
}

func loadValueSpaceForTarget(ctx context.Context, target *referTarget, input *Input) (*valueSpace, error) {
	desc, err := input.PRFiles.FindDescriptorByName(target.fullName)
	if err != nil {
		return nil, xerrors.E2001(target.refer, string(target.fullName.Name()))
	}
	messageDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, xerrors.E2001(target.refer, string(target.fullName.Name()))
	}

	// get workbook name and worksheet name
	fileOpts := messageDesc.ParentFile().Options().(*descriptorpb.FileOptions)
	bookOpts := proto.GetExtension(fileOpts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	bookName := bookOpts.Name

	msgOpts := messageDesc.Options().(*descriptorpb.MessageOptions)
	sheetOpts := proto.GetExtension(msgOpts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	sheetName := sheetOpts.Name

	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(bookName, input.SubdirRewrites)
	absWbPath := filepath.Join(input.InputDir, rewrittenWorkbookName)
	primaryImporter, err := input.ImporterCache.Load(ctx, absWbPath, importer.Sheets([]string{sheetName}))
	if err != nil {
		return nil, xerrors.WrapKV(err,
			xerrors.KeyReferBookName, bookName,
			xerrors.KeyReferSheetName, sheetName,
		)
	}

	// get merger importer infos
	impInfos, err := input.ImporterCache.LoadMergerImporters(ctx, input.InputDir, rewrittenWorkbookName, sheetName, sheetOpts.Merger, input.SubdirRewrites)
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
			err = space.addFromTable(header, sheet.Table.Transpose(), target.column, referredBookName, specifiedSheetName)
		} else {
			err = space.addFromTable(header, sheet.Table, target.column, referredBookName, specifiedSheetName)
		}
		if err != nil {
			return nil, err
		}
	}

	return space, nil
}

func loadValueSpace(ctx context.Context, refer string, input *Input) (*valueSpace, error) {
	target, err := normalizeRefer(refer, input)
	if err != nil {
		return nil, err
	}
	return loadValueSpaceForTarget(ctx, target, input)
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

	targets, err := r.getTargets(prop.Refer, input)
	if err != nil {
		return err
	}
	for _, target := range targets {
		entry, err := r.getEntry(target.key(), func() (*valueSpace, error) {
			return loadValueSpaceForTarget(ctx, target, input)
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
