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
	opts := parseOptions(setters...)
	fmt := format.GetFormat(filename)
	switch fmt {
	case format.Excel:
		return NewExcelImporter(ctx, filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	case format.CSV:
		return NewCSVImporter(ctx, filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	case format.XML:
		return NewXMLImporter(ctx, filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	case format.YAML:
		return NewYAMLImporter(ctx, filename, opts.Sheets, opts.Parser, opts.Mode, opts.Cloned)
	default:
		return nil, xerrors.Errorf("unsupported format: %v", fmt)
	}
}

type ImporterInfo struct {
	Importer
	SpecifiedSheetName string // Empty means no sheet specified.
}

// GetScatterImporters return all related importer infos.
//  1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//  2. exclude self
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetScatterImporters(ctx context.Context, inputDir, primaryBookName, sheetName string, scatterSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	var importerInfos []ImporterInfo
	for _, specifier := range scatterSpecifiers {
		relBookPaths, specifiedSheetName, err := ResolveSheetSpecifier(inputDir, primaryBookName, specifier, subdirRewrites)
		if err != nil {
			return nil, xerrors.WrapKV(err, xerrors.KeyPrimarySheetName, sheetName)
		}
		if specifiedSheetName == "" {
			specifiedSheetName = sheetName
		}
		for relBookPath := range relBookPaths {
			log.Infof("%15s: %s#%s", "scatter sheet", relBookPath, specifiedSheetName)
			fpath := filepath.Join(inputDir, relBookPath)
			rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)
			primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
			importer, err := New(ctx, fpath, Sheets([]string{specifiedSheetName}), Cloned(primaryBookPath))
			if err != nil {
				return nil, xerrors.Wrapf(err, "failed to create importer: %s", fpath)
			}
			importerInfos = append(importerInfos, ImporterInfo{Importer: importer, SpecifiedSheetName: specifiedSheetName})
		}
	}
	return importerInfos, nil
}

// GetMergerImporters return all related importer infos.
//  1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//  2. exclude self
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func GetMergerImporters(ctx context.Context, inputDir, primaryBookName, sheetName string, sheetSpecifiers []string, subdirRewrites map[string]string) ([]ImporterInfo, error) {
	var importerInfos []ImporterInfo
	for _, specifier := range sheetSpecifiers {
		relBookPaths, specifiedSheetName, err := ResolveSheetSpecifier(inputDir, primaryBookName, specifier, subdirRewrites)
		if err != nil {
			return nil, xerrors.WrapKV(err, xerrors.KeyPrimarySheetName, sheetName)
		}
		if specifiedSheetName == "" {
			specifiedSheetName = sheetName
		}
		for relBookPath := range relBookPaths {
			log.Infof("%15s: %s#%s", "merging sheet", relBookPath, specifiedSheetName)
			fpath := filepath.Join(inputDir, relBookPath)
			rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)
			primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
			importer, err := New(ctx, fpath, Sheets([]string{specifiedSheetName}), Cloned(primaryBookPath))
			if err != nil {
				return nil, xerrors.Wrapf(err, "failed to create importer: %s", fpath)
			}
			importerInfos = append(importerInfos, ImporterInfo{Importer: importer, SpecifiedSheetName: specifiedSheetName})
		}
	}

	return importerInfos, nil
}

// ResolveSheetSpecifier resolve and return all related workbook paths.
//  1. support Glob pattern, refer https://pkg.go.dev/path/filepath#Glob
//  2. exclude self
//  3. special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
func ResolveSheetSpecifier(inputDir, primaryBookName string, sheetSpecifier string, subdirRewrites map[string]string) (map[string]bool, string, error) {
	relBookPaths := map[string]bool{}
	bookNameGlob, specifiedSheetName := ParseSheetSpecifier(sheetSpecifier)

	// rewrite subdir
	rewrittenWorkbookName := xfs.RewriteSubdir(primaryBookName, subdirRewrites)

	primaryBookPath := filepath.Join(inputDir, rewrittenWorkbookName)
	log.Debugf("rewrittenAbsWorkbookName: %s", primaryBookPath)
	fmt := format.GetFormat(primaryBookPath)
	pattern := xfs.Join(filepath.Dir(primaryBookPath), bookNameGlob)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, "", xerrors.Wrapf(err, "failed to glob pattern: %s", pattern)
	}
	if len(matches) == 0 {
		err := xerrors.E3000(sheetSpecifier, pattern)
		return nil, "", xerrors.WrapKV(err, xerrors.KeyPrimaryBookName, primaryBookName)
	}
	for _, match := range matches {
		path := match
		if fmt == format.CSV {
			// special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
			path, err = xfs.ParseCSVBooknamePatternFrom(match)
			if err != nil {
				return nil, "", err
			}
		}
		if specifiedSheetName == "" && xfs.IsSamePath(path, primaryBookPath) {
			// sheet name not specified, so exclude self
			continue
		}
		secondaryBookName, err := xfs.Rel(inputDir, path)
		if err != nil {
			return nil, "", err
		}
		relBookPaths[secondaryBookName] = true
	}
	return relBookPaths, specifiedSheetName, nil
}

// ParseSheetSpecifier parses the sheet specifier pattern like: "<BookNameGlob>[#SheetName]".
//  1. The delimiter between BookNameGlob and SheetName is "#".
//  2. The "SheetName" is optional, default is same as sheet name in the primary workbook.
func ParseSheetSpecifier(specifier string) (bookNameGlob string, specifiedSheetName string) {
	// NOTE: "Activity#Item.csv" will be parsed as bookNameGlob: "Activity" and specifiedSheetName: "Item.csv".
	// TODO: This is a problem, need to be solved.
	lastIndex := strings.LastIndex(specifier, "#")
	if lastIndex != -1 {
		bookNameGlob = specifier[:lastIndex]
		specifiedSheetName = specifier[lastIndex+1:]
	} else {
		bookNameGlob = specifier
	}
	return
}
