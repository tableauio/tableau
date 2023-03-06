package importer

import (
	"encoding/csv"
	"os"
	"path/filepath"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
)

// CSVImporter recognizes pattern: "<BookName>#<SheetName>.csv"
type CSVImporter struct {
	*book.Book
}

func NewCSVImporter(filename string, sheetNames []string, parser book.SheetParser) (*CSVImporter, error) {
	book, err := parseCSVBook(filename, sheetNames, parser)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse csv book")
	}

	return &CSVImporter{
		Book: book,
	}, nil
}

func parseCSVBook(filename string, sheetNames []string, parser book.SheetParser) (*book.Book, error) {
	_, _, err := fs.ParseCSVFilenamePattern(filename)
	if err != nil {
		return nil, err
	}

	book, err := readCSVBook(filename, parser)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
	}

	if parser != nil {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}

	if sheetNames != nil {
		book.Squeeze(sheetNames)
	}

	return book, nil
}

func readCSVBook(filename string, parser book.SheetParser) (*book.Book, error) {
	bookName, _, err := fs.ParseCSVFilenamePattern(filename)
	if err != nil {
		return nil, errors.Errorf("cannot parse the book name from filename: %s", filename)
	}
	globFilename := fs.GenCSVBooknamePattern(filepath.Dir(filename), bookName)
	matches, err := filepath.Glob(globFilename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to glob %s", globFilename)
	}
	if len(matches) == 0 {
		return nil, errors.Errorf("no matching files found for %s", globFilename)
	}

	// NOTE: keep the order of sheets
	set := treeset.NewWithStringComparator()
	for _, filename := range matches {
		set.Add(filename)
	}

	newBook := book.NewBook(bookName, globFilename, parser)
	for _, val := range set.Values() {
		filename := val.(string)
		_, sheetName, err := fs.ParseCSVFilenamePattern(filename)
		if err != nil {
			return nil, errors.Errorf("cannot parse the sheet name from filename: %s", filename)
		}
		records, err := readCSV(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read CSV file: %s", filename)
		}
		sheet := book.NewSheet(sheetName, records)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func readCSV(filename string) ([][]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file: %s", filename)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// If FieldsPerRecord is negative, records may have a variable number of fields.
	r.FieldsPerRecord = -1
	return r.ReadAll()
}
