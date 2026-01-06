package importer

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
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
	// Format returns workboot format.
	Format() format.Format
	// GetBookOptions creates a new tableaupb.WorkbookOptions
	// based on this special sheet(#)'s info.
	GetBookOptions() *tableaupb.WorkbookOptions
	// GetSheets returns all sheets in order of the book.
	GetSheets() []*book.Sheet
	// GetSheet returns a Sheet of the specified sheet name.
	GetSheet(name string) *book.Sheet
}

// New creates a new importer.
func New(ctx context.Context, filename string, setters ...Option) (Importer, error) {
	fmt := format.GetFormat(filename)
	switch fmt {
	case format.Excel:
		return NewExcelImporter(ctx, filename, setters...)
	case format.CSV:
		return NewCSVImporter(ctx, filename, setters...)
	case format.XML:
		return NewXMLImporter(ctx, filename, setters...)
	case format.YAML:
		return NewYAMLImporter(ctx, filename, setters...)
	default:
		return nil, xerrors.Newf("unsupported format: %v", fmt)
	}
}

type ImporterInfo struct {
	Importer
	SpecifiedSheetName string // Empty means no sheet specified.
}

// GetScatterImporters parses and returns all related importer infos.
//  1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//  2. exclude primary sheet, and auto filter out duplicate importers
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetScatterImporters(ctx context.Context, inputDir, primaryBookName, primarySheetName string, sheetSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	var importerInfos []ImporterInfo
	books := map[string][]string{} // relative book path -> sheet name patterns
	for _, specifier := range sheetSpecifiers {
		relBookPaths, sheetNamePattern, err := ResolveSheetSpecifier(inputDir, primaryBookName, specifier, subdirRewrites)
		if err != nil {
			return nil, xerrors.WrapKV(err, xerrors.KeyPrimarySheetName, primarySheetName)
		}
		if sheetNamePattern == "" {
			sheetNamePattern = primarySheetName
		}
		for relBookPath := range relBookPaths {
			books[relBookPath] = append(books[relBookPath], sheetNamePattern)
		}
	}

	for relBookPath, sheetNamePatterns := range books {
		log.Infof("%15s: %s#%s", "scatter sheet", relBookPath, sheetNamePatterns)
		path := filepath.Join(inputDir, relBookPath)
		rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)
		primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
		importer, err := New(ctx, path, Sheets(sheetNamePatterns), Cloned(primaryBookPath))
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to create importer: %s", path)
		}
		for _, sheet := range importer.GetSheets() {
			if xfs.IsSamePath(path, primaryBookPath) && sheet.Name == primarySheetName {
				continue // skip primary sheet
			}
			importerInfos = append(importerInfos, ImporterInfo{Importer: importer, SpecifiedSheetName: sheet.Name})
		}
	}
	return importerInfos, nil
}

// GetMergerImporters parses and returns all related importer infos.
//  1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//  2. support filepath.Match pattern for worksheet name, see https://pkg.go.dev/path/filepath#Match
//  3. exclude primary sheet, and auto filter out duplicate importers
//  4. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetMergerImporters(ctx context.Context, inputDir, primaryBookName, primarySheetName string, sheetSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	var importerInfos []ImporterInfo
	books := map[string][]string{} // relative book path -> sheet name patterns
	for _, specifier := range sheetSpecifiers {
		relBookPaths, sheetNamePattern, err := ResolveSheetSpecifier(inputDir, primaryBookName, specifier, subdirRewrites)
		if err != nil {
			return nil, xerrors.WrapKV(err, xerrors.KeyPrimarySheetName, primarySheetName)
		}
		if sheetNamePattern == "" {
			sheetNamePattern = primarySheetName
		}
		for relBookPath := range relBookPaths {
			books[relBookPath] = append(books[relBookPath], sheetNamePattern)
		}
	}

	for relBookPath, sheetNamePatterns := range books {
		log.Infof("%15s: %s#%s", "merge sheet", relBookPath, sheetNamePatterns)
		path := filepath.Join(inputDir, relBookPath)
		rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)
		primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
		importer, err := New(ctx, path, Sheets(sheetNamePatterns), Cloned(primaryBookPath))
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to create importer: %s", path)
		}
		for _, sheet := range importer.GetSheets() {
			if xfs.IsSamePath(path, primaryBookPath) && sheet.Name == primarySheetName {
				continue // skip primary sheet
			}
			importerInfos = append(importerInfos, ImporterInfo{Importer: importer, SpecifiedSheetName: sheet.Name})
		}
	}

	return importerInfos, nil
}

// ResolveSheetSpecifier resolves and returns all related workbook paths and sheet names.
//  1. support filepath.Glob pattern for workbook file, see https://pkg.go.dev/path/filepath#Glob
//  2. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func ResolveSheetSpecifier(inputDir, primaryBookName string, sheetSpecifier string, subdirRewrites map[string]string) (relBookPaths map[string]bool, sheetNamePattern string, err error) {
	relBookPaths = map[string]bool{}
	bookNamePattern, sheetNamePattern := parseSheetSpecifier(sheetSpecifier)

	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)

	primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
	log.Debugf("rewrittenAbsWorkbookName: %s", primaryBookPath)
	fmt := format.GetFormat(primaryBookPath)
	filePattern := xfs.Join(filepath.Dir(primaryBookPath), bookNamePattern)
	fileMatches, err := filepath.Glob(filePattern)
	if err != nil {
		err = xerrors.Wrapf(err, "failed to glob pattern: %s", filePattern)
		return
	}
	if len(fileMatches) == 0 {
		err = xerrors.WrapKV(xerrors.E3000(sheetSpecifier, filePattern), xerrors.KeyPrimaryBookName, primaryBookName)
		return
	}
	for _, fileMatch := range fileMatches {
		path := fileMatch
		if fmt == format.CSV {
			// special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
			path, err = xfs.ParseCSVBooknamePatternFrom(fileMatch)
			if err != nil {
				return
			}
		}
		// if sheetNamePattern == "" && xfs.IsSamePath(path, primaryBookPath) {
		// 	// sheet name not specified, so exclude self
		// 	continue
		// }
		var secondaryBookPath string
		secondaryBookPath, err = xfs.Rel(inputDir, path)
		if err != nil {
			return
		}
		relBookPaths[secondaryBookPath] = true
	}
	return relBookPaths, sheetNamePattern, nil
}

// parseSheetSpecifier parses the sheet specifier pattern like: "<BookNamePattern>[#SheetNamePattern]".
//  1. The delimiter between BookNamePattern and SheetNamePattern is "#".
//  2. The "SheetNamePattern" is optional, default is same as sheet name in the primary workbook.
func parseSheetSpecifier(specifier string) (bookNamePattern string, sheetNamePattern string) {
	// NOTE: "Activity#Item.csv" will be parsed as BookNamePattern: "Activity" and SheetNamePattern: "Item.csv".
	// TODO: This is a problem, need to be solved.
	lastIndex := strings.LastIndex(specifier, "#")
	if lastIndex != -1 {
		bookNamePattern = specifier[:lastIndex]
		sheetNamePattern = specifier[lastIndex+1:]
	} else {
		bookNamePattern = specifier
	}
	return
}
