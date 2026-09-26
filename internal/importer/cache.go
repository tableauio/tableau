package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/singleflight"
)

// cacheKeySeparator is NUL because valid paths and sheet names cannot
// contain it. Common separators such as "-" can occur in both and may collide.
const cacheKeySeparator = "\x00"

var errCacheClosed = errors.New("importer cache is closed")

type cacheKey struct {
	filename        string
	sheets          string
	mode            ImporterMode
	cloned          bool
	primaryBookName string
}

// Cache reuses imported data during one generator run. It shares decoded
// sheets between read-only confgen importers and owns cached Excel handles
// until Close. Callers must not mutate cached sheets or call Load concurrently
// with Close.
type Cache struct {
	// entries caches the importer view for an exact source and option set.
	entries sync.Map
	// sources caches format-specific decoded data by logical source path.
	sources sync.Map
	paths   sync.Map
	// loads coalesces concurrent initialization of the same cached source.
	loads singleflight.Group

	requests  atomic.Int64
	imports   atomic.Int64
	sheets    atomic.Int64
	pathCount atomic.Int64
	closed    atomic.Bool
}

type cachedExcel struct {
	// Excelize reads and the sheets map share one lock because an excelize.File
	// is not read concurrently here. Different workbooks still load in parallel.
	mu     sync.Mutex
	file   *excelize.File
	sheets map[string]*book.Sheet
}

type cachedCSV struct {
	mu       sync.Mutex
	name     string
	filename string
	sheets   map[string]*book.Sheet
}

type cachedDocument struct {
	importer Importer
}

// cachedSource represents one normalized origin. Implementations retain the
// decoded data that can be shared safely and create filtered importer views.
type cachedSource interface {
	load(context.Context, []string) (Importer, int64, error)
	close() error
}

// NewCache creates an empty importer cache.
func NewCache() *Cache {
	return &Cache{}
}

// Load returns a cached importer or loads it once for concurrent callers.
func (c *Cache) Load(ctx context.Context, filename string, setters ...Option) (Importer, error) {
	if c == nil {
		return New(ctx, filename, setters...)
	}
	if c.closed.Load() {
		return nil, errCacheClosed
	}
	opts := parseOptions(setters...)
	// Protogen may truncate, parse, and purge sheets. A custom parser may also
	// carry state. Neither is safe to identify or share through this cache.
	if opts.Mode == Protogen || opts.Parser != nil {
		return New(ctx, filename, setters...)
	}
	cacheFilename, err := normalizeCacheFilename(filename)
	if err != nil {
		return nil, err
	}
	c.requests.Add(1)
	if _, loaded := c.paths.LoadOrStore(cacheFilename, struct{}{}); !loaded {
		c.pathCount.Add(1)
	}
	key := cacheKey{
		filename:        cacheFilename,
		sheets:          strings.Join(opts.Sheets, cacheKeySeparator),
		mode:            opts.Mode,
		cloned:          opts.Cloned,
		primaryBookName: filepath.Clean(opts.PrimaryBookName),
	}
	if cached, ok := c.entries.Load(key); ok {
		return cached.(Importer), nil
	}
	return c.loadSource(ctx, key, opts)
}

// LoadScatterImporters returns related scatter importers through the cache. A
// nil cache performs direct loads.
func (c *Cache) LoadScatterImporters(ctx context.Context, inputDir, primaryBookName, primarySheetName string, sheetSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	return loadSheetSpecifierImporters(ctx, inputDir, primaryBookName, primarySheetName, sheetSpecifiers, subdirRewrites, "scatter sheet", c.Load)
}

// LoadMergerImporters returns related merger importers through the cache. A
// nil cache performs direct loads.
func (c *Cache) LoadMergerImporters(ctx context.Context, inputDir, primaryBookName, primarySheetName string, sheetSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	return loadSheetSpecifierImporters(ctx, inputDir, primaryBookName, primarySheetName, sheetSpecifiers, subdirRewrites, "merge sheet", c.Load)
}

func (c *Cache) loadSource(ctx context.Context, key cacheKey, opts *Options) (Importer, error) {
	loaded, err, _ := c.loads.Do(key.filename, func() (any, error) {
		if cached, ok := c.sources.Load(key.filename); ok {
			return cached.(cachedSource), nil
		}
		source, decoded, err := openCachedSource(ctx, key.filename)
		if err != nil {
			return nil, err
		}
		c.imports.Add(1)
		c.sheets.Add(decoded)
		c.sources.Store(key.filename, source)
		return source, nil
	})
	if err != nil {
		return nil, err
	}

	view, decoded, err := loaded.(cachedSource).load(ctx, opts.Sheets)
	c.sheets.Add(decoded)
	if err != nil {
		return nil, err
	}
	actual, _ := c.entries.LoadOrStore(key, view)
	return actual.(Importer), nil
}

func openCachedSource(ctx context.Context, filename string) (cachedSource, int64, error) {
	switch format.GetFormat(filename) {
	case format.Excel:
		file, err := excelize.OpenFile(filename)
		if err != nil {
			return nil, 0, xerrors.E3002(err)
		}
		return &cachedExcel{file: file, sheets: make(map[string]*book.Sheet)}, 0, nil
	case format.CSV:
		readerOpts, err := parseCSVBookReaderOptions(filename, nil, metasheet.FromContext(ctx).Name)
		if err != nil {
			return nil, 0, err
		}
		return &cachedCSV{
			name:     readerOpts.Name,
			filename: readerOpts.Filename,
			sheets:   make(map[string]*book.Sheet),
		}, 0, nil
	case format.XML, format.YAML:
		imp, err := New(ctx, filename)
		if err != nil {
			return nil, 0, err
		}
		return &cachedDocument{importer: imp}, int64(len(imp.GetSheets())), nil
	default:
		return nil, 0, xerrors.Newf("unsupported cached source format: %v", format.GetFormat(filename))
	}
}

func (c *cachedExcel) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	// The lock protects both Excelize access and the check-then-decode sequence,
	// ensuring that each sheet is decoded at most once per generator run.
	c.mu.Lock()
	defer c.mu.Unlock()

	readerOpts, err := parseExcelBookReaderOptions(c.file.Path, c.file, sheetNames)
	if err != nil {
		return nil, 0, err
	}
	loadedBook := book.NewBook(ctx, readerOpts.Name, readerOpts.Filename, nil)
	var decoded int64
	for _, sheetOpts := range readerOpts.Sheets {
		sheet := c.sheets[sheetOpts.Name]
		if sheet == nil {
			rows, err := readExcelSheetRows(c.file, sheetOpts.Name, 0, excelize.Options{RawCellValue: true})
			if err != nil {
				return nil, decoded, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetOpts.Name)
			}
			sheet = book.NewTableSheet(sheetOpts.Name, rows)
			c.sheets[sheetOpts.Name] = sheet
			decoded++
		}
		loadedBook.AddSheet(sheet)
	}
	return &ExcelImporter{Book: loadedBook}, decoded, nil
}

func (c *cachedExcel) close() error {
	return c.file.Close()
}

func (c *cachedCSV) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	readerOpts, err := parseCSVBookReaderOptions(c.filename, sheetNames, metasheet.FromContext(ctx).Name)
	if err != nil {
		return nil, 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	loadedBook := book.NewBook(ctx, c.name, c.filename, nil)
	var decoded int64
	for _, sheetOpts := range readerOpts.Sheets {
		sheet := c.sheets[sheetOpts.Name]
		if sheet == nil {
			sheet, err = readCSVSheet(sheetOpts.Filename, sheetOpts.Name, 0)
			if err != nil {
				return nil, decoded, err
			}
			c.sheets[sheetOpts.Name] = sheet
			decoded++
		}
		loadedBook.AddSheet(sheet)
	}
	return &CSVImporter{Book: loadedBook}, decoded, nil
}

func (*cachedCSV) close() error {
	return nil
}

func (c *cachedDocument) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	view, err := newDocumentImporterView(ctx, c.importer, sheetNames)
	return view, 0, err
}

func (*cachedDocument) close() error {
	return nil
}

func newDocumentImporterView(ctx context.Context, source Importer, sheetNames []string) (Importer, error) {
	loadedBook := book.NewBook(ctx, source.BookName(), source.Filename(), nil)
	for _, sheet := range source.GetSheets() {
		if wantSheet(sheet.Name, sheetNames) {
			loadedBook.AddSheet(sheet)
		}
	}
	switch source.Format() {
	case format.XML:
		return &XMLImporter{Book: loadedBook}, nil
	case format.YAML:
		return &YAMLImporter{Book: loadedBook}, nil
	default:
		return nil, xerrors.Newf("unsupported cached document format: %v", source.Format())
	}
}

func normalizeCacheFilename(filename string) (string, error) {
	cleaned := filepath.Clean(filename)
	if format.GetFormat(cleaned) != format.CSV {
		return cleaned, nil
	}
	pattern, err := xfs.ParseCSVBooknamePatternFrom(cleaned)
	if err != nil {
		return "", err
	}
	return filepath.Clean(pattern), nil
}

// Close releases cached workbook handles. It is safe to call more than once.
func (c *Cache) Close() error {
	if c == nil {
		return nil
	}
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	var errs []error
	c.sources.Range(func(_, value any) bool {
		if err := value.(cachedSource).close(); err != nil {
			errs = append(errs, err)
		}
		return true
	})
	return errors.Join(errs...)
}

// Metrics returns load requests, importer loads, decoded sheets, and paths.
func (c *Cache) Metrics() (requests, imports, sheets, paths int64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	return c.requests.Load(), c.imports.Load(), c.sheets.Load(), c.pathCount.Load()
}
