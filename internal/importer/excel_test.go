package importer

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/excel"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/xerrors"
	"github.com/xuri/excelize/v2"
)

func TestNewExcelImporter(t *testing.T) {
	type args struct {
		ctx        context.Context
		filename   string
		sheetNames []string
		parser     book.SheetParser
		mode       ImporterMode
		cloned     bool
	}
	tests := []struct {
		name       string
		args       args
		wantSheets []*book.Sheet
		wantErr    bool
		err        error
	}{
		{
			name: "normal",
			args: args{
				ctx:        context.Background(),
				filename:   "testdata/Test.xlsx",
				sheetNames: []string{"Item"},
			},
			wantSheets: []*book.Sheet{
				book.NewTableSheet("Item", [][]string{
					{"ID", "Name"},
					{"1", "Pike"},
					{"2", "Thompson"},
				}),
			},
			wantErr: false,
		},
		{
			name: "E3002",
			args: args{
				ctx:      context.Background(),
				filename: "testdata/Test_NotFound.xlsx",
			},
			wantSheets: nil,
			wantErr:    true,
			err:        xerrors.ErrE3002,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExcelImporter(tt.args.ctx, tt.args.filename, tt.args.sheetNames, tt.args.parser, tt.args.mode, tt.args.cloned)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExcelImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got.GetSheets(), tt.wantSheets) {
					t.Errorf("NewExcelImporter() = %v, want %v", got.GetSheets(), tt.wantSheets)
				}
			} else {
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}

func Test_readExcelSheetRows(t *testing.T) {
	sheetName := "Sheet1"
	f, err := excel.Open("testdata/RawCellValue.xlsx", sheetName)
	if err != nil {
		panic(err)
	}

	SetCellValue := func(cell string, value any) {
		err := f.SetCellValue(sheetName, cell, value)
		assert.NoError(t, err)
	}

	SetStyle := func(topLeftCell string, bottomRightCell string, style excelize.Style) {
		s, err := f.NewStyle(&style)
		assert.NoError(t, err)
		err = f.SetCellStyle(sheetName, topLeftCell, bottomRightCell, s)
		assert.NoError(t, err)
	}

	// Refer to https://xuri.me/excelize/en/style.html#number_format
	// number format with one thousand separator (,)
	SetStyle("A1", "A4", excelize.Style{NumFmt: 3}) // #,##0
	SetCellValue("A1", int32(10001000))
	SetCellValue("A2", uint32(10001000))
	SetCellValue("A3", int64(10001000))
	SetCellValue("A4", uint64(10001000))

	// float number with two decimal places
	SetStyle("A5", "A6", excelize.Style{NumFmt: 4}) // #,##0.00
	SetCellValue("A5", float32(100.12345))
	SetCellValue("A6", float64(10001000.12345))

	SetCellValue("A7", "string")
	SetCellValue("A8", []byte("bytes"))
	SetCellValue("A9", true)
	SetCellValue("A10", false)

	// datetime with custom format
	customFormat := "m/d/yyyy hh:mm"
	SetStyle("A11", "A11", excelize.Style{CustomNumFmt: &customFormat})
	SetCellValue("A11", "2025-12-01 05:59:59")

	type args struct {
		f         *excelize.File
		sheetName string
		topN      uint
		opts      []excelize.Options
	}
	tests := []struct {
		name     string
		args     args
		wantRows [][]string
		wantErr  bool
	}{
		{
			name: "all rows in formatted value",
			args: args{
				f:         f,
				sheetName: sheetName,
				topN:      0,
				opts:      nil,
			},
			wantRows: [][]string{
				{"10,001,000"},
				{"10,001,000"},
				{"10,001,000"},
				{"10,001,000"},
				{"100.12"},
				{"10,001,000.12"},
				{"string"},
				{"bytes"},
				{"TRUE"},
				{"FALSE"},
				{"2025-12-01 05:59:59"}, // Why not "12/1/2025 05:59" ?
			},
			wantErr: false,
		},
		{
			name: "all rows in raw cell value",
			args: args{
				f:         f,
				sheetName: sheetName,
				topN:      0,
				opts: []excelize.Options{
					{RawCellValue: true},
				},
			},
			wantRows: [][]string{
				{"10001000"},
				{"10001000"},
				{"10001000"},
				{"10001000"},
				{"100.12345"},
				{"10001000.12345"},
				{"string"},
				{"bytes"},
				{"1"},
				{"0"},
				{"2025-12-01 05:59:59"},
			},
			wantErr: false,
		},
		{
			name: "top 3 rows in raw cell value",
			args: args{
				f:         f,
				sheetName: sheetName,
				topN:      3,
				opts: []excelize.Options{
					{RawCellValue: true},
				},
			},
			wantRows: [][]string{
				{"10001000"},
				{"10001000"},
				{"10001000"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRows, err := readExcelSheetRows(tt.args.f, tt.args.sheetName, tt.args.topN, tt.args.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("readExcelSheetRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equal(t, tt.wantRows, gotRows)
		})
	}
}
