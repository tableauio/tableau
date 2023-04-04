package importer

import (
	"encoding/csv"
	"io"
	"os"
	"path/filepath"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
)

// CSVImporter recognizes pattern: "<BookName>#<SheetName>.csv"
type CSVImporter struct {
	*book.Book
}

func NewCSVImporter(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*CSVImporter, error) {
	book, err := parseCSVBook(filename, sheetNames, parser, mode, cloned)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse csv book")
	}

	return &CSVImporter{
		Book: book,
	}, nil
}

func parseCSVBook(filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*book.Book, error) {
	_, _, err := fs.ParseCSVFilenamePattern(filename)
	if err != nil {
		return nil, err
	}

	brOpts, err := parseCSVBookReaderOptions(filename, sheetNames)
	if err != nil {
		return nil, err
	}

	if mode == Protogen {
		err := adjustCSVTopN(brOpts, parser, cloned)
		if err != nil {
			return nil, errors.WithMessagef(err, "failed to read book: %s", filename)
		}
	}

	book, err := readCSVBook(brOpts, parser)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
	}

	if parser != nil {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}
	return book, nil
}

func adjustCSVTopN(brOpts *bookReaderOptions, parser book.SheetParser, cloned bool) error {
	if parser != nil && !cloned {
		metasheetReaderOpts := brOpts.GetMetasheet()
		if metasheetReaderOpts == nil {
			log.Debugf("metasheet not found, use default TopN: %d", defaultTopN)
			for _, shReaderOpts := range brOpts.Sheets {
				shReaderOpts.TopN = defaultTopN
			}
			return nil
		}
		// parse metasheet, and change topN to 0 if any sheet is transpose or not default mode.
		metasheet, err := readCSVSheet(brOpts.GetMetasheet().Filename, book.MetasheetName, 0)
		if err != nil {
			return err
		}
		meta, err := book.ParseMetasheet(metasheet, parser)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse metasheet: %s", book.MetasheetName)
		}

		for _, shReaderOpts := range brOpts.Sheets {
			metasheet := meta.MetasheetMap[shReaderOpts.Name]
			if metasheet == nil || (metasheet.Mode == tableaupb.Mode_MODE_DEFAULT && !metasheet.Transpose) {
				log.Debugf("sheet %s is in default mode and not transpose, so topN is reset to defaultTopN: %d", defaultTopN)
				shReaderOpts.TopN = defaultTopN
			}
		}
	}
	return nil
}

func readCSVBook(brOpts *bookReaderOptions, parser book.SheetParser) (*book.Book, error) {
	newBook := book.NewBook(brOpts.Name, brOpts.Filename, parser)
	for _, srOpts := range brOpts.Sheets {
		rows, err := readCSVRows(srOpts.Filename, srOpts.TopN)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read CSV file: %s", srOpts.Filename)
		}
		sheet := book.NewSheet(srOpts.Name, rows)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func readCSVSheet(filename, sheetName string, topN uint) (*book.Sheet, error) {
	rows, err := readCSVRows(filename, topN)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read CSV file: %s", filename)
	}
	return book.NewSheet(sheetName, rows), nil
}

func readCSVRows(filename string, topN uint) (rows [][]string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to open file: %s", filename)
	}
	defer f.Close()

	r := csv.NewReader(f)

	// topN: 0 means read all rows
	if topN == 0 {
		// If FieldsPerRecord is negative, records may have a variable number of fields.
		r.FieldsPerRecord = -1
		return r.ReadAll()
	}

	// read topN rows
	var nrow uint
	for {
		nrow++
		if nrow > topN {
			break
		}
		row, err := r.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Wrapf(err, "read one CSV row failed")
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseCSVBookReaderOptions(filename string, sheetNames []string) (*bookReaderOptions, error) {
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

	brOpts := &bookReaderOptions{
		Name:     bookName,
		Filename: globFilename,
	}
	for _, val := range set.Values() {
		filename := val.(string)
		_, sheetName, err := fs.ParseCSVFilenamePattern(filename)
		if err != nil {
			return nil, errors.Errorf("cannot parse the book name from filename: %s", filename)
		}
		var needed bool
		if len(sheetNames) == 0 {
			// read all sheets if sheetNames not set.
			needed = true
		} else {
			for _, name := range sheetNames {
				if name == sheetName {
					needed = true
					break
				}
			}
		}
		if !needed {
			continue
		}
		shReaderOpt := &sheetReaderOptions{
			Filename: filename,
			Name:     sheetName,
		}
		brOpts.Sheets = append(brOpts.Sheets, shReaderOpt)
	}
	return brOpts, nil
}
