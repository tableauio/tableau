package importer

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/singleflight"
)

// cacheKeySeparator is NUL because valid paths and Excel sheet names cannot
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

// Cache reuses imported data during one generator run. It owns cached Excel
// handles until Close and shares decoded sheets between read-only confgen
// importers. Callers must not mutate sheets returned by a cached importer or
// call Load concurrently with Close.
type Cache struct {
	// entries caches the importer view for an exact filename and option set.
	entries sync.Map
	// excels caches one open handle and its decoded sheets per Excel file.
	excels sync.Map
	paths  sync.Map
	// loads coalesces concurrent opens of the same importer or Excel file.
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
	c.requests.Add(1)
	if _, loaded := c.paths.LoadOrStore(filepath.Clean(filename), struct{}{}); !loaded {
		c.pathCount.Add(1)
	}
	key := cacheKey{
		filename:        filepath.Clean(filename),
		sheets:          strings.Join(opts.Sheets, cacheKeySeparator),
		mode:            opts.Mode,
		cloned:          opts.Cloned,
		primaryBookName: filepath.Clean(opts.PrimaryBookName),
	}
	if cached, ok := c.entries.Load(key); ok {
		return cached.(Importer), nil
	}
	// Keep the workbook open so later requests can decode only the additional
	// sheets they need instead of reopening and reparsing the entire XLSX file.
	if strings.EqualFold(filepath.Ext(key.filename), ".xlsx") {
		return c.loadExcel(ctx, key, opts)
	}

	loaded, err, _ := c.loads.Do(key.String(), func() (any, error) {
		if cached, ok := c.entries.Load(key); ok {
			return cached.(Importer), nil
		}
		imp, err := New(ctx, filename, setters...)
		if err != nil {
			return nil, err
		}
		c.imports.Add(1)
		c.entries.Store(key, imp)
		return imp, nil
	})
	if err != nil {
		return nil, err
	}
	return loaded.(Importer), nil
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

func (c *Cache) loadExcel(ctx context.Context, key cacheKey, opts *Options) (Importer, error) {
	loaded, err, _ := c.loads.Do("excel"+cacheKeySeparator+key.filename, func() (any, error) {
		if cached, ok := c.excels.Load(key.filename); ok {
			return cached.(*cachedExcel), nil
		}
		file, err := excelize.OpenFile(key.filename)
		if err != nil {
			return nil, xerrors.E3002(err)
		}
		cached := &cachedExcel{file: file, sheets: make(map[string]*book.Sheet)}
		c.imports.Add(1)
		c.excels.Store(key.filename, cached)
		return cached, nil
	})
	if err != nil {
		return nil, err
	}

	cached := loaded.(*cachedExcel)
	// The lock protects both Excelize access and the check-then-decode sequence,
	// ensuring that each sheet is decoded at most once per generator run.
	cached.mu.Lock()
	defer cached.mu.Unlock()
	readerOpts, err := parseExcelBookReaderOptions(key.filename, cached.file, opts.Sheets)
	if err != nil {
		return nil, err
	}
	// Each option set gets its own Book view containing only the requested
	// sheets. The underlying immutable Sheet values are shared across views.
	loadedBook := book.NewBook(ctx, readerOpts.Name, readerOpts.Filename, nil)
	for _, sheetOpts := range readerOpts.Sheets {
		sheet := cached.sheets[sheetOpts.Name]
		if sheet == nil {
			rows, err := readExcelSheetRows(cached.file, sheetOpts.Name, 0, excelize.Options{RawCellValue: true})
			if err != nil {
				return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetOpts.Name)
			}
			sheet = book.NewTableSheet(sheetOpts.Name, rows)
			cached.sheets[sheetOpts.Name] = sheet
			c.sheets.Add(1)
		}
		loadedBook.AddSheet(sheet)
	}
	view := &ExcelImporter{Book: loadedBook}
	actual, _ := c.entries.LoadOrStore(key, view)
	return actual.(Importer), nil
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
	c.excels.Range(func(_, value any) bool {
		if err := value.(*cachedExcel).file.Close(); err != nil {
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

func (k cacheKey) String() string {
	return strings.Join([]string{
		k.filename,
		k.sheets,
		strconv.Itoa(int(k.mode)),
		strconv.FormatBool(k.cloned),
		k.primaryBookName,
	}, cacheKeySeparator)
}
