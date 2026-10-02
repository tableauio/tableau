package importer

import (
	"context"
	"errors"

	"github.com/tableauio/tableau/internal/importer/xlsx"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/log"
	"github.com/xuri/excelize/v2"
)

// excelRowReader is the workbook-reader contract used by the Excel importer.
// Backend-specific APIs and errors are adapted to this interface below.
type excelRowReader interface {
	SheetNames() []string
	ReadRows(sheetName string, limit uint) ([][]string, error)
	Close() error
}

type xlsxRowReader struct {
	reader                 *xlsx.Reader
	selectiveSharedStrings bool
}

func openXLSXRowReader(filename string, selectiveSharedStrings bool) (excelRowReader, error) {
	reader, err := xlsx.Open(filename)
	if err != nil {
		if !errors.Is(err, xlsx.ErrUnsupported) {
			return nil, xerrors.E3002(err)
		}
		return nil, err
	}
	return xlsxRowReader{reader: reader, selectiveSharedStrings: selectiveSharedStrings}, nil
}

func (r xlsxRowReader) SheetNames() []string {
	return r.reader.SheetNames()
}

func (r xlsxRowReader) ReadRows(sheetName string, limit uint) ([][]string, error) {
	var rows [][]string
	var err error
	if limit == 0 && !r.selectiveSharedStrings {
		rows, err = r.reader.ReadRows(sheetName)
	} else {
		rows, err = r.reader.ReadRowsN(sheetName, limit)
	}
	if errors.Is(err, xlsx.ErrSheetNotFound) {
		return nil, ErrSheetNotFound
	}
	return rows, err
}

func (r xlsxRowReader) Close() error {
	return r.reader.Close()
}

type excelizeRowReader struct {
	file *excelize.File
}

func openExcelizeRowReader(filename string) (excelRowReader, error) {
	file, err := excelize.OpenFile(filename)
	if err != nil {
		return nil, xerrors.E3002(err)
	}
	return excelizeRowReader{file: file}, nil
}

func (r excelizeRowReader) SheetNames() []string {
	return r.file.GetSheetList()
}

func (r excelizeRowReader) ReadRows(sheetName string, limit uint) ([][]string, error) {
	return readExcelizeRows(r.file, sheetName, limit, excelize.Options{RawCellValue: true})
}

func (r excelizeRowReader) Close() error {
	return r.file.Close()
}

// importExcel uses the focused XLSX reader for protogen and retries the whole
// import with Excelize when that reader cannot handle the workbook. Retrying
// the whole import preserves one consistent backend for every sheet.
func importExcel(ctx context.Context, filename string, opts *Options) (*ExcelImporter, error) {
	if opts.Mode == Protogen {
		importer, err := importWithXLSX(ctx, filename, opts)
		if err == nil {
			return importer, nil
		}
		if !errors.Is(err, xlsx.ErrUnsupported) {
			return nil, err
		}
		log.Debugf("raw XLSX reader unavailable for protogen %s, using excelize: %v", filename, err)
		compat, compatErr := importWithExcelize(ctx, filename, opts)
		if compatErr != nil {
			return nil, errors.Join(err, compatErr)
		}
		return compat, nil
	}
	return importWithExcelize(ctx, filename, opts)
}

func importWithXLSX(ctx context.Context, filename string, opts *Options) (*ExcelImporter, error) {
	reader, err := openXLSXRowReader(filename, true)
	if err != nil {
		return nil, err
	}
	defer closeExcelReader(reader)
	return readExcelImporter(ctx, filename, opts, reader, true)
}

func importWithExcelize(ctx context.Context, filename string, opts *Options) (*ExcelImporter, error) {
	reader, err := openExcelizeRowReader(filename)
	if err != nil {
		return nil, err
	}
	defer closeExcelReader(reader)
	return readExcelImporter(ctx, filename, opts, reader, false)
}

func closeExcelReader(reader excelRowReader) {
	if err := reader.Close(); err != nil {
		log.Error(err)
	}
}

// readExcelizeRows reads at most limit rows from a sheet. A zero limit reads
// every row. Excelize details stay in this backend adapter.
func readExcelizeRows(file *excelize.File, sheetName string, limit uint, opts ...excelize.Options) (rows [][]string, err error) {
	if idx, err := file.GetSheetIndex(sheetName); err != nil {
		return nil, xerrors.Wrapf(err, "failed to get sheet index: %s", sheetName)
	} else if idx == -1 {
		return nil, ErrSheetNotFound
	}

	if limit == 0 {
		rows, err := file.GetRows(sheetName, opts...)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to get all rows of sheet: %s#%s", file.Path, sheetName)
		}
		return rows, nil
	}

	excelRows, err := file.Rows(sheetName)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to get topN(%d) rows of sheet: %s#%s", limit, file.Path, sheetName)
	}
	defer func() {
		if closeErr := excelRows.Close(); closeErr != nil && err == nil {
			err = xerrors.Wrapf(closeErr, "failed to close row iterator: %s#%s", file.Path, sheetName)
		}
	}()
	for rowNumber := uint(1); excelRows.Next() && rowNumber <= limit; rowNumber++ {
		row, err := excelRows.Columns(opts...)
		if err != nil {
			return nil, xerrors.Wrapf(err, "read the %dth row failed: %s#%s", rowNumber, file.Path, sheetName)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
