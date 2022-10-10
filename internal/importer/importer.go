package importer

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
)

type Importer interface {
	// Filename returns the parsed filename of the original inputed filename.
	// 	- Excel: same as the inputed filename.
	// 	- CSV: recognizes pattern: "<BookName>#<SheetName>.csv", and returns Glob name "<BookName>#*.csv".
	// 	- XML: same as the inputed filename.
	Filename() string
	// Bookname returns the book name after parsing the original inputed filename.
	// 	- Excel: the base filename without file extension.
	// 	- CSV: recognizes pattern: "<BookName>#<SheetName>.csv", and returns "<BookName>".
	// 	- XML: the base filename without file extension.
	BookName() string
	// GetSheets returns all sheets in order of the book.
	GetSheets() []*book.Sheet
	// GetSheet returns a Sheet of the specified sheet name.
	GetSheet(name string) *book.Sheet
}

// New creates a new importer.
func New(filename string, setters ...Option) (Importer, error) {
	opts := parseOptions(setters...)
	fmt := format.Ext2Format(filepath.Ext(filename))
	switch fmt {
	case format.Excel:
		return NewExcelImporter(filename, opts.Sheets, opts.Parser, opts.TopN)
	case format.CSV:
		return NewCSVImporter(filename, opts.Sheets, opts.Parser)
	case format.XML:
		return NewXMLImporter(filename, opts.Sheets, opts.Parser)
	default:
		return nil, errors.Errorf("unsupported format: %v", fmt)
	}
}

// GetMergerImporters gathers all merger importers.
// 	1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
// 	2. exclude self
func GetMergerImporters(primaryWorkbookPath, sheetName string, merger []string) ([]Importer, error) {
	if len(merger) == 0 {
		return nil, nil
	}

	curDir := filepath.Dir(primaryWorkbookPath)
	mergerWorkbookPaths := map[string]bool{}
	for _, merger := range merger {
		pattern := filepath.Join(curDir, merger)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to glob pattern: %s", pattern)
		}
		for _, match := range matches {
			if fs.IsSamePath(match, primaryWorkbookPath) {
				// exclude self
				continue
			}
			mergerWorkbookPaths[match] = true
		}
	}
	var importers []Importer
	for fpath := range mergerWorkbookPaths {
		log.Infof("%18s: %s", "merge workbook", fpath)
		importer, err := New(fpath, Sheets([]string{sheetName}))
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to create importer: %s", fpath)
		}
		importers = append(importers, importer)
	}
	return importers, nil
}
