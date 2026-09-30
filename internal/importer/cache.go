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
	"github.com/tableauio/tableau/internal/importer/xlsx"
	"github.com/tableauio/tableau/internal/profile"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/xuri/excelize/v2"
	"golang.org/x/sync/singleflight"
)

// cacheKeySeparator is NUL because valid paths and sheet names cannot
// contain it. Common separators such as "-" can occur in both and may collide.
const cacheKeySeparator = "\x00"

type cacheKey struct {
	filename        string
	sheets          string // ordered, NUL-separated sheet selection
	mode            ImporterMode
	cloned          bool
	primaryBookName string
}

// Cache reuses imported data while it remains reachable. Sources own decoded
// sheets and workbook handles; entries are immutable importer views for one
// option set. Load may be called concurrently. The cache has no shutdown step.
type Cache struct {
	profiling bool

	// entries maps an exact source and option set to an importer view.
	entries sync.Map
	// sources maps a normalized logical path to shared format-specific data.
	sources sync.Map
	// sourceLoads ensures concurrent views open each source only once.
	sourceLoads singleflight.Group

	// metricPaths deduplicates pathCount and is unused when profiling is off.
	metricPaths sync.Map
	requests    atomic.Int64 // Load calls
	imports     atomic.Int64 // opened sources
	sheets      atomic.Int64 // decoded sheets
	pathCount   atomic.Int64 // unique normalized paths
}

type cachedExcel struct {
	// Reader state and decodedSheets share one lock. Separate cachedExcel
	// instances still decode concurrently.
	mu            sync.Mutex
	filename      string
	rawReader     excelRowReader
	excelizeFile  *excelize.File // compatibility fallback, opened lazily
	sheetNames    []string
	decodedSheets map[string]*book.Sheet
	// preferRaw keeps raw XLSX reads enabled until a read failure switches this
	// source to the Excelize fallback.
	preferRaw bool
	profiling bool
}

type cachedCSV struct {
	mu            sync.Mutex
	bookName      string
	filename      string
	decodedSheets map[string]*book.Sheet
}

type cachedDocument struct {
	importer Importer
}

// cachedSource owns reusable data for one normalized origin. load creates a
// filtered importer view over that data.
type cachedSource interface {
	load(context.Context, []string) (Importer, int64, error)
}

// NewCache creates an empty importer cache.
func NewCache() *Cache {
	return &Cache{}
}

// EnableProfiling enables pprof labels and cache metrics. Call it before the
// cache is used.
func (c *Cache) EnableProfiling() {
	c.profiling = true
}

// Load returns a cached importer or loads it once for concurrent callers.
func (c *Cache) Load(ctx context.Context, filename string, setters ...Option) (Importer, error) {
	if c == nil {
		return New(ctx, filename, setters...)
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
	if c.profiling {
		c.requests.Add(1)
		if _, loaded := c.metricPaths.LoadOrStore(cacheFilename, struct{}{}); !loaded {
			c.pathCount.Add(1)
		}
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
	return c.loadEntry(ctx, key, opts)
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

// loadEntry opens the shared source if needed, then publishes one importer view
// for the requested option set.
func (c *Cache) loadEntry(ctx context.Context, key cacheKey, opts *Options) (Importer, error) {
	loaded, err, _ := c.sourceLoads.Do(key.filename, func() (any, error) {
		cached, ok := c.sources.Load(key.filename)
		if ok {
			return cached.(cachedSource), nil
		}
		source, decoded, err := c.openSource(ctx, key.filename)
		if err != nil {
			return nil, err
		}
		if c.profiling {
			c.imports.Add(1)
			c.sheets.Add(decoded)
		}
		actual, loaded := c.sources.LoadOrStore(key.filename, source)
		if loaded {
			return actual.(cachedSource), nil
		}
		return source, nil
	})
	if err != nil {
		return nil, err
	}

	imp, decoded, err := c.loadImporter(ctx, loaded.(cachedSource), key.filename, opts.Sheets)
	if c.profiling {
		c.sheets.Add(decoded)
	}
	if err != nil {
		return nil, err
	}
	actual, _ := c.entries.LoadOrStore(key, imp)
	return actual.(Importer), nil
}

func (c *Cache) openSource(ctx context.Context, filename string) (source cachedSource, decoded int64, err error) {
	if !c.profiling {
		return openCachedSource(ctx, filename, false)
	}
	err = profile.Run(ctx, func(ctx context.Context) error {
		source, decoded, err = openCachedSource(ctx, filename, true)
		return err
	},
		"work", "source_open",
		"format", string(format.GetFormat(filename)),
		"source", filename,
	)
	return source, decoded, err
}

func (c *Cache) loadImporter(ctx context.Context, source cachedSource, filename string, sheetNames []string) (imp Importer, decoded int64, err error) {
	if !c.profiling {
		return source.load(ctx, sheetNames)
	}
	err = profile.Run(ctx, func(ctx context.Context) error {
		imp, decoded, err = source.load(ctx, sheetNames)
		return err
	},
		"work", "sheet_decode",
		"format", string(format.GetFormat(filename)),
		"source", filename,
	)
	return imp, decoded, err
}

func openCachedSource(ctx context.Context, filename string, profiling bool) (cachedSource, int64, error) {
	inputFormat := format.GetFormat(filename)
	switch inputFormat {
	case format.Excel:
		source, err := openCachedExcel(filename, profiling)
		return source, 0, err
	case format.CSV:
		readerOpts, err := parseCSVBookReaderOptions(filename, nil, metasheet.FromContext(ctx).Name)
		if err != nil {
			return nil, 0, err
		}
		return &cachedCSV{
			bookName:      readerOpts.Name,
			filename:      readerOpts.Filename,
			decodedSheets: make(map[string]*book.Sheet),
		}, 0, nil
	case format.XML, format.YAML:
		imp, err := New(ctx, filename)
		if err != nil {
			return nil, 0, err
		}
		return &cachedDocument{importer: imp}, int64(len(imp.GetSheets())), nil
	default:
		return nil, 0, xerrors.Newf("unsupported cached source format: %v", inputFormat)
	}
}

func (c *cachedExcel) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	readerOpts := buildExcelBookReaderOptions(c.filename, c.sheetNames, sheetNames)
	loadedBook := book.NewBook(ctx, readerOpts.Name, readerOpts.Filename, nil)
	var decoded int64
	for _, sheetOpts := range readerOpts.Sheets {
		sheet := c.decodedSheets[sheetOpts.Name]
		if sheet == nil {
			var err error
			sheet, err = c.loadSheet(ctx, sheetOpts.Name)
			if err != nil {
				return nil, decoded, err
			}
			c.decodedSheets[sheetOpts.Name] = sheet
			decoded++
		}
		loadedBook.AddSheet(sheet)
	}
	return &ExcelImporter{Book: loadedBook}, decoded, nil
}

func (c *cachedExcel) loadSheet(ctx context.Context, sheetName string) (sheet *book.Sheet, err error) {
	if !c.profiling {
		return c.readSheet(sheetName)
	}
	err = profile.Run(ctx, func(context.Context) error {
		sheet, err = c.readSheet(sheetName)
		return err
	}, "sheet", sheetName, "reader", c.readerName())
	return sheet, err
}

func (c *cachedExcel) readSheet(sheetName string) (*book.Sheet, error) {
	rows, err := c.readRows(sheetName)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to get rows of sheet: %s", sheetName)
	}
	return book.NewTableSheet(sheetName, rows), nil
}

func openCachedExcel(filename string, profiling bool) (*cachedExcel, error) {
	reader, err := xlsx.Open(filename)
	if err == nil {
		return &cachedExcel{
			filename:      filename,
			rawReader:     xlsxRowReader{reader: reader},
			sheetNames:    reader.SheetNames(),
			decodedSheets: make(map[string]*book.Sheet),
			preferRaw:     true,
			profiling:     profiling,
		}, nil
	}
	if !errors.Is(err, xlsx.ErrUnsupported) {
		return nil, err
	}
	log.Debugf("raw XLSX reader unavailable for %s, using excelize: %v", filename, err)
	file, openErr := excelize.OpenFile(filename)
	if openErr != nil {
		return nil, xerrors.E3002(errors.Join(err, openErr))
	}
	return &cachedExcel{
		filename:      filename,
		excelizeFile:  file,
		sheetNames:    file.GetSheetList(),
		decodedSheets: make(map[string]*book.Sheet),
		profiling:     profiling,
	}, nil
}

func (c *cachedExcel) readerName() string {
	if c.rawReader != nil || c.preferRaw {
		return "xlsx"
	}
	return "excelize"
}

func (c *cachedExcel) readRows(sheetName string) ([][]string, error) {
	if err := c.ensureReader(); err != nil {
		return nil, err
	}
	if c.rawReader == nil {
		return readExcelizeRows(c.excelizeFile, sheetName, 0, excelize.Options{RawCellValue: true})
	}
	rows, err := c.rawReader.ReadRows(sheetName, 0)
	if err == nil {
		return rows, nil
	}
	if !errors.Is(err, xlsx.ErrUnsupported) {
		return nil, err
	}
	return c.fallbackToExcelize(sheetName, err)
}

// ensureReader reopens the selected workbook backend after Release.
func (c *cachedExcel) ensureReader() error {
	if c.rawReader != nil || c.excelizeFile != nil {
		return nil
	}
	if c.preferRaw {
		reader, err := xlsx.Open(c.filename)
		if err == nil {
			c.rawReader = xlsxRowReader{reader: reader}
			return nil
		}
		if !errors.Is(err, xlsx.ErrUnsupported) {
			return err
		}
		log.Debugf("raw XLSX reader unavailable for %s, using excelize: %v", c.filename, err)
		c.preferRaw = false
	}
	_, err := c.openExcelize()
	return err
}

// fallbackToExcelize permanently switches this source after a sheet cannot be
// decoded by the raw reader. If excelize cannot open the workbook, the raw
// reader remains available so the combined error retains both failures.
func (c *cachedExcel) fallbackToExcelize(sheetName string, readErr error) ([][]string, error) {
	file, err := c.openExcelize()
	if err != nil {
		return nil, errors.Join(readErr, err)
	}
	log.Debugf("raw XLSX reader failed for %s#%s, using excelize: %v", c.filename, sheetName, readErr)
	_ = c.rawReader.Close()
	c.rawReader = nil
	c.preferRaw = false
	rows, err := readExcelizeRows(file, sheetName, 0, excelize.Options{RawCellValue: true})
	if err != nil {
		return nil, errors.Join(readErr, err)
	}
	return rows, nil
}

func (c *cachedExcel) openExcelize() (*excelize.File, error) {
	if c.excelizeFile != nil {
		return c.excelizeFile, nil
	}
	file, err := excelize.OpenFile(c.filename)
	if err != nil {
		return nil, xerrors.E3002(err)
	}
	c.excelizeFile = file
	return file, nil
}

func (c *cachedCSV) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	readerOpts, err := parseCSVBookReaderOptions(c.filename, sheetNames, metasheet.FromContext(ctx).Name)
	if err != nil {
		return nil, 0, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	loadedBook := book.NewBook(ctx, c.bookName, c.filename, nil)
	var decoded int64
	for _, sheetOpts := range readerOpts.Sheets {
		sheet := c.decodedSheets[sheetOpts.Name]
		if sheet == nil {
			sheet, err = readCSVSheet(sheetOpts.Filename, sheetOpts.Name, 0)
			if err != nil {
				return nil, decoded, err
			}
			c.decodedSheets[sheetOpts.Name] = sheet
			decoded++
		}
		loadedBook.AddSheet(sheet)
	}
	return &CSVImporter{Book: loadedBook}, decoded, nil
}

func (c *cachedDocument) load(ctx context.Context, sheetNames []string) (Importer, int64, error) {
	view, err := newDocumentImporterView(ctx, c.importer, sheetNames)
	return view, 0, err
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

// Metrics returns load requests, importer loads, decoded sheets, and paths.
// Counters remain zero unless profiling was enabled before loading.
func (c *Cache) Metrics() (requests, imports, sheets, paths int64) {
	if c == nil {
		return 0, 0, 0, 0
	}
	return c.requests.Load(), c.imports.Load(), c.sheets.Load(), c.pathCount.Load()
}
