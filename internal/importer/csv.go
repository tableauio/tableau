package importer

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
)

// CSVImporter recognizes pattern: "<BookName>#<SheetName>.csv"
type CSVImporter struct {
	*book.Book
}

func NewCSVImporter(ctx context.Context, filename string, sheetNames []string, parser book.SheetParser, mode ImporterMode, cloned bool) (*CSVImporter, error) {
	brOpts, err := parseCSVBookReaderOptions(filename, sheetNames, metasheet.FromContext(ctx).Name)
	if err != nil {
		return nil, err
	}

	if mode == Protogen {
		err := adjustCSVTopN(ctx, brOpts, parser, cloned)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read book: %s", filename)
		}
	}

	book, err := readCSVBook(ctx, brOpts, parser)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read csv book: %s", filename)
	}

	if mode == Protogen {
		if err := book.ParseMetaAndPurge(); err != nil {
			return nil, xerrors.Wrapf(err, "failed to parse metasheet")
		}
	}
	return &CSVImporter{
		Book: book,
	}, nil
}

func adjustCSVTopN(ctx context.Context, brOpts *bookReaderOptions, parser book.SheetParser, cloned bool) error {
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
		ms, err := readCSVSheet(brOpts.GetMetasheet().Filename, metasheet.FromContext(ctx).Name, 0)
		if err != nil {
			return err
		}
		meta, err := ms.ParseMetasheet(parser)
		if err != nil {
			return xerrors.Wrapf(err, "failed to parse metasheet: %s", metasheet.FromContext(ctx).Name)
		}

		for _, srOpts := range brOpts.Sheets {
			if srOpts.Name == metasheet.FromContext(ctx).Name {
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

func readCSVBook(ctx context.Context, brOpts *bookReaderOptions, parser book.SheetParser) (*book.Book, error) {
	newBook := book.NewBook(ctx, brOpts.Name, brOpts.Filename, parser)
	for _, srOpts := range brOpts.Sheets {
		rows, err := readCSVRows(srOpts.Filename, srOpts.TopN)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to read CSV file: %s", srOpts.Filename)
		}
		sheet := book.NewTableSheet(srOpts.Name, rows)
		newBook.AddSheet(sheet)
	}

	return newBook, nil
}

func readCSVSheet(filename, sheetName string, topN uint) (*book.Sheet, error) {
	rows, err := readCSVRows(filename, topN)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to read CSV file: %s", filename)
	}
	return book.NewTableSheet(sheetName, rows), nil
}

func readCSVRows(filename string, topN uint) (rows [][]string, err error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, xerrors.E3002(err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Panicf("failed to close file: %s", filename)
		}
	}()

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
			return nil, xerrors.Wrapf(err, "read one CSV row failed")
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseCSVBookReaderOptions(filename string, sheetNames []string, metasheetName string) (*bookReaderOptions, error) {
	bookName, _, err := xfs.ParseCSVFilenamePattern(filename)
	if err != nil {
		return nil, xerrors.Errorf("cannot parse the book name from filename: %s", filename)
	}
	globFilename := xfs.GenCSVBooknamePattern(filepath.Dir(filename), bookName)
	matches, err := filepath.Glob(globFilename)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to glob %s", globFilename)
	}
	if len(matches) == 0 {
		return nil, xerrors.Errorf("no matching files found for %s", globFilename)
	}

	// NOTE: keep the order of sheets
	set := treeset.NewWithStringComparator()
	for _, filename := range matches {
		set.Add(xfs.CleanSlashPath(filename))
	}

	brOpts := &bookReaderOptions{
		Name:          bookName,
		Filename:      globFilename,
		MetasheetName: metasheetName,
	}
	for _, val := range set.Values() {
		filename := val.(string)
		_, sheetName, err := xfs.ParseCSVFilenamePattern(filename)
		if err != nil {
			return nil, xerrors.Errorf("cannot parse the book name from filename: %s", filename)
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
