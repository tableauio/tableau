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

type cacheKey struct {
	filename        string
	sheets          string
	mode            ImporterMode
	cloned          bool
	primaryBookName string
}

// Cache reuses immutable imported books during one generator run.
type Cache struct {
	entries sync.Map
	excels  sync.Map
	paths   sync.Map
	loads   singleflight.Group

	requests  atomic.Int64
	imports   atomic.Int64
	sheets    atomic.Int64
	pathCount atomic.Int64
}

type cachedExcel struct {
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
	opts := parseOptions(setters...)
	c.requests.Add(1)
	if _, loaded := c.paths.LoadOrStore(filepath.Clean(filename), struct{}{}); !loaded {
		c.pathCount.Add(1)
	}
	key := cacheKey{
		filename:        filepath.Clean(filename),
		sheets:          strings.Join(opts.Sheets, "\x00"),
		mode:            opts.Mode,
		cloned:          opts.Cloned,
		primaryBookName: filepath.Clean(opts.PrimaryBookName),
	}
	if cached, ok := c.entries.Load(key); ok {
		return cached.(Importer), nil
	}
	if opts.Mode != Protogen && opts.Parser == nil && strings.EqualFold(filepath.Ext(key.filename), ".xlsx") {
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

func (c *Cache) loadExcel(ctx context.Context, key cacheKey, opts *Options) (Importer, error) {
	loaded, err, _ := c.loads.Do("excel\x00"+key.filename, func() (any, error) {
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
	cached.mu.Lock()
	defer cached.mu.Unlock()
	readerOpts, err := parseExcelBookReaderOptions(key.filename, cached.file, opts.Sheets)
	if err != nil {
		return nil, err
	}
	loadedBook := book.NewBook(ctx, readerOpts.Name, readerOpts.Filename, opts.Parser)
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

// Close releases cached workbook handles.
func (c *Cache) Close() error {
	if c == nil {
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

// Stats returns load requests, importer loads, decoded sheets, and paths.
func (c *Cache) Stats() (requests, imports, sheets, paths int64) {
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
	}, "\x00")
}
