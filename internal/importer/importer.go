package importer

import (
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
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
	// Metabook returns the metadata of the book.
	Metabook() *tableaupb.Metabook
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
		return NewExcelImporter(filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	case format.CSV:
		return NewCSVImporter(filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	case format.XML:
		return NewXMLImporter(filename, opts.Sheets, opts.Parser, opts.Mode)
	default:
		return nil, errors.Errorf("unsupported format: %v", fmt)
	}
}

// GetMergerImporters return all related importers.
//    1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//    2. exclude self
//    3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetMergerImporters(primaryBookPath, sheetName string, bookNameGlobs []string) ([]Importer, error) {
	bookPaths, err := resolveBookPaths(primaryBookPath, sheetName, bookNameGlobs)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to resolve workbook paths")
	}
	var importers []Importer
	for fpath := range bookPaths {
		log.Infof("%18s: %s", "merge workbook", fpath)
		importer, err := New(fpath, Sheets([]string{sheetName}), Cloned())
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to create importer: %s", fpath)
		}
		importers = append(importers, importer)
	}
	return importers, nil
}

// GetScatterImporters return all related importers.
// 	1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
// 	2. exclude self
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetScatterImporters(primaryBookPath, sheetName string, bookNameGlobs []string) ([]Importer, error) {
	bookPaths, err := resolveBookPaths(primaryBookPath, sheetName, bookNameGlobs)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to resolve workbook paths")
	}
	var importers []Importer
	for fpath := range bookPaths {
		log.Infof("%18s: %s", "scatter workbook", fpath)
		importer, err := New(fpath, Sheets([]string{sheetName}), Cloned())
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to create importer: %s", fpath)
		}
		importers = append(importers, importer)
	}
	return importers, nil
}

// resolveBookPaths resolve and return all related workbook paths.
// 	1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
// 	2. exclude self
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func resolveBookPaths(primaryBookPath, sheetName string, bookNameGlobs []string) (map[string]bool, error) {
	bookPaths := map[string]bool{}
	if len(bookNameGlobs) == 0 {
		return bookPaths, nil
	}

	fmt := format.Ext2Format(filepath.Ext(primaryBookPath))
	curDir := filepath.Dir(primaryBookPath)
	for _, nameGlob := range bookNameGlobs {
		pattern := filepath.Join(curDir, nameGlob)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to glob pattern: %s", pattern)
		}
		for _, match := range matches {
			path := match
			if fmt == format.CSV {
				// special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
				path, err = fs.ParseCSVBooknamePatternFrom(match)
				if err != nil {
					return nil, err
				}
			}
			if fs.IsSamePath(path, primaryBookPath) {
				// exclude self
				continue
			}
			slashPath := fs.GetCleanSlashPath(path)
			bookPaths[slashPath] = true
		}
	}
	return bookPaths, nil
}
