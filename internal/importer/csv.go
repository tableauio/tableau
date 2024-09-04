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

	if mode == Protogen {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, errors.WithMessage(err, "failed to parse metasheet")
		}
	}
	return &CSVImporter{
		Book: book,
	}, nil
}

func adjustCSVTopN(brOpts *bookReaderOptions, parser book.SheetParser, cloned bool) error {
	if parser != nil && !cloned {
		// parse metasheet, and change topN to 0 if any sheet is transpose or not default mode.
		metasheetReaderOpts := brOpts.GetMetasheet()
		if metasheetReaderOpts == nil {
			log.Debugf("metasheet not found, use default TopN: %d", defaultTopN)
			for _, srOpts := range brOpts.Sheets {
				srOpts.TopN = defaultTopN
			}
			return nil
		}
		metasheet, err := readCSVSheet(brOpts.GetMetasheet().Filename, book.MetasheetName, 0)
		if err != nil {
			return err
		}
		meta, err := metasheet.ParseMetasheet(parser)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse metasheet: %s", book.MetasheetName)
		}

		for _, srOpts := range brOpts.Sheets {
			if srOpts.Name == book.MetasheetName {
				// for metasheet, read all rows
				srOpts.TopN = 0
				continue
			}
			metasheet := meta.MetasheetMap[srOpts.Name]
			if metasheet == nil || (metasheet.Mode == tableaupb.Mode_MODE_DEFAULT && !metasheet.Transpose) {
				log.Debugf("sheet %s is in default mode and not transpose, so topN is reset to defaultTopN: %d", srOpts.Name, defaultTopN)
				srOpts.TopN = defaultTopN
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
		sheet := book.NewTableSheet(srOpts.Name, rows)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func readCSVSheet(filename, sheetName string, topN uint) (*book.Sheet, error) {
	rows, err := readCSVRows(filename, topN)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read CSV file: %s", filename)
	}
	return book.NewTableSheet(sheetName, rows), nil
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
		set.Add(fs.CleanSlashPath(filename))
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
		if NeedSheet(sheetName, sheetNames) {
			shReaderOpt := &sheetReaderOptions{
				Filename: filename,
				Name:     sheetName,
			}
			brOpts.Sheets = append(brOpts.Sheets, shReaderOpt)
		}
	}
	return brOpts, nil
}
