package fieldprop

import (
	"context"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/emirpasic/gods/sets/hashset"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/profile"
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

// ReferredCache caches references in two stages for one generation or load:
// refers normalizes source expressions, and entries loads each canonical target.
// Separate locks keep the frequent lookups in these stages independent.
type ReferredCache struct {
	profiling bool

	entriesMu sync.RWMutex
	// entries maps each canonical target to its loaded values or an in-flight
	// load. Entries are pointers so the loader can publish its result to waiters.
	// Retaining failed loads also prevents every referring cell from reporting
	// the same source error.
	entries map[referCacheKey]*referCacheEntry

	refersMu sync.RWMutex
	// refers caches normalization by source spelling. Different spellings can
	// resolve to the same canonical target in entries.
	refers map[rawRefer]referCacheKey
}

// referCacheKey identifies one referred column by
// <fully-qualified-message-name>.<column-name>. An unavailable key is stored
// only in refers as a sentinel for a raw refer that failed normalization; it
// must never be used as an entries key.
type referCacheKey struct {
	message     protoreflect.FullName
	column      string
	unavailable bool // the first normalization attempt failed and reported its error
}

func (k referCacheKey) String() string {
	return string(k.message) + "." + k.column
}

// referCacheEntry holds an in-flight or completed canonical target load.
type referCacheEntry struct {
	ready chan struct{} // closed after space or unavailable is published
	space *valueSpace   // values collected from the target's primary and merged sheets
	// unavailable means the target load failed. The loader returned the error;
	// waiters and later callers skip it to avoid duplicate errors per cell.
	unavailable bool
}

// rawRefer includes the protobuf package because the same refer text can name
// different messages in different packages.
type rawRefer struct {
	protoPackage string
	value        string
}

type valueSpace struct {
	*hashset.Set
	bookName  string // loaded primary workbook, relative to the input directory
	sheetName string // primary worksheet defining the referred value space
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
		refers:  make(map[rawRefer]referCacheKey),
	}
}

// EnableProfiling enables pprof labels. Call it before the cache is used.
func (r *ReferredCache) EnableProfiling() {
	r.profiling = true
}

type loadValueSpaceFunc = func() (*valueSpace, error)

// cachedEntry returns a completed entry snapshot. It waits when another caller
// is still loading the target.
func (r *ReferredCache) cachedEntry(ctx context.Context, key referCacheKey) (referCacheEntry, bool) {
	r.entriesMu.RLock()
	entry, ok := r.entries[key]
	r.entriesMu.RUnlock()
	if !ok {
		return referCacheEntry{}, false
	}
	return r.waitForEntry(ctx, key, entry), true
}

// getEntry loads each normalized message column once. Callers for the same key
// wait for its ready channel, while unrelated targets load concurrently.
func (r *ReferredCache) getEntry(ctx context.Context, key referCacheKey, loadFunc loadValueSpaceFunc) (referCacheEntry, error) {
	cached, ok := r.cachedEntry(ctx, key)
	if ok {
		return cached, nil
	}

	r.entriesMu.Lock()
	entry, ok := r.entries[key]
	if ok {
		r.entriesMu.Unlock()
		return r.waitForEntry(ctx, key, entry), nil
	}
	// Publish the placeholder before loading so concurrent callers wait on this
	// entry instead of loading the same target again.
	entry = &referCacheEntry{ready: make(chan struct{})}
	r.entries[key] = entry
	r.entriesMu.Unlock()

	var space *valueSpace
	var err error
	if r.profiling {
		err = profile.Run(ctx, func(context.Context) error {
			space, err = loadFunc()
			return err
		}, "work", "refer_load", "refer", key.String())
	} else {
		space, err = loadFunc()
	}
	r.entriesMu.Lock()
	if err == nil {
		entry.space = space
	} else {
		// Only the loader reports the error. Waiters observe unavailable and skip
		// duplicate errors for the same broken reference.
		entry.unavailable = true
	}
	// Closing ready publishes the selected outcome to every waiter.
	close(entry.ready)
	r.entriesMu.Unlock()
	return *entry, err
}

func (r *ReferredCache) waitForEntry(ctx context.Context, key referCacheKey, entry *referCacheEntry) referCacheEntry {
	if r.profiling {
		_ = profile.Run(ctx, func(context.Context) error {
			<-entry.ready
			return nil
		}, "work", "refer_wait", "refer", key.String())
	} else {
		<-entry.ready
	}
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
	ProtoPackage string
	InputDir     string
	// SourceBookName and SourceSheetName identify the referring cell.
	SourceBookName  string
	SourceSheetName string
	SubdirRewrites  map[string]string
	PRFiles         *protoregistry.Files
	ImporterCache   *importer.Cache
	Present         bool // field presence
}

func normalizeRefer(refer string, input *Input) (referCacheKey, error) {
	referInfo, err := parseRefer(refer)
	if err != nil {
		return referCacheKey{}, err
	}
	messageName := referInfo.getMessageName()
	return referCacheKey{
		message: protoreflect.FullName(input.ProtoPackage + "." + messageName),
		column:  referInfo.Column,
	}, nil
}

// resolveKey normalizes each raw refer once. The first failed normalization is
// returned as an error; later calls receive an unavailable sentinel so repeated
// cells do not report the same malformed refer.
func (r *ReferredCache) resolveKey(refer string, input *Input) (referCacheKey, error) {
	raw := rawRefer{protoPackage: input.ProtoPackage, value: refer}
	r.refersMu.RLock()
	key, ok := r.refers[raw]
	r.refersMu.RUnlock()
	if ok {
		return key, nil
	}

	r.refersMu.Lock()
	defer r.refersMu.Unlock()
	// Another goroutine may have normalized this refer while the write lock was
	// pending.
	key, ok = r.refers[raw]
	if ok {
		return key, nil
	}
	key, err := normalizeRefer(refer, input)
	if err != nil {
		r.refers[raw] = referCacheKey{unavailable: true}
		return referCacheKey{}, err
	}
	r.refers[raw] = key
	return key, nil
}

func loadValueSpaceForKey(ctx context.Context, refer string, key referCacheKey, input *Input) (*valueSpace, error) {
	desc, err := input.PRFiles.FindDescriptorByName(key.message)
	if err != nil {
		return nil, xerrors.E2001(refer, string(key.message.Name()))
	}
	messageDesc, ok := desc.(protoreflect.MessageDescriptor)
	if !ok {
		return nil, xerrors.E2001(refer, string(key.message.Name()))
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
	space.bookName = primaryImporter.Filename()
	if relBookName, err := xfs.Rel(input.InputDir, space.bookName); err == nil {
		space.bookName = relBookName
	}
	space.sheetName = sheetName
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
			err = space.addFromTable(header, sheet.Table.Transpose(), key.column, referredBookName, specifiedSheetName)
		} else {
			err = space.addFromTable(header, sheet.Table, key.column, referredBookName, specifiedSheetName)
		}
		if err != nil {
			return nil, err
		}
	}

	return space, nil
}

func wrapSourceLocation(err error, input *Input) error {
	if err == nil || input == nil || (input.SourceBookName == "" && input.SourceSheetName == "") {
		return err
	}
	return xerrors.WrapKV(err,
		xerrors.KeyBookName, input.SourceBookName,
		xerrors.KeySheetName, input.SourceSheetName,
	)
}

func loadValueSpace(ctx context.Context, refer string, input *Input) (space *valueSpace, err error) {
	defer func() { err = wrapSourceLocation(err, input) }()
	key, err := normalizeRefer(refer, input)
	if err != nil {
		return nil, err
	}
	return loadValueSpaceForKey(ctx, refer, key, input)
}

// CheckRefer validates cellData against prop.Refer.
func (r *ReferredCache) CheckRefer(ctx context.Context, prop *tableaupb.FieldProp, cellData string, input *Input) (err error) {
	defer func() { err = wrapSourceLocation(err, input) }()
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

	var failedSpaces []*valueSpace
	for _, refer := range strings.Split(prop.Refer, ",") {
		refer = strings.TrimSpace(refer)
		if refer == "" {
			continue
		}
		key, err := r.resolveKey(refer, input)
		if err != nil {
			return err
		}
		if key.unavailable {
			// The first lookup already returned the normalization error.
			return nil
		}
		entry, ok := r.cachedEntry(ctx, key)
		if !ok {
			entry, err = r.getEntry(ctx, key, func() (*valueSpace, error) {
				return loadValueSpaceForKey(ctx, refer, key, input)
			})
		}
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
		failedSpaces = append(failedSpaces, entry.space)
	}
	err = xerrors.E2002(cellData, prop.Refer)
	if len(failedSpaces) == 0 {
		return err
	}
	// Any target may satisfy the reference. Report their locations only after
	// the value is absent from every target, keeping each location independent.
	failures := make([]error, len(failedSpaces))
	for i, space := range failedSpaces {
		failures[i] = xerrors.WrapKV(err,
			xerrors.KeyReferBookName, space.bookName,
			xerrors.KeyReferSheetName, space.sheetName,
		)
	}
	return errors.Join(failures...)
}
