package importer

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/importer/metasheet"
)

func TestCSVImporter_ExportExcel(t *testing.T) {
	importer, _ := NewCSVImporter(context.Background(), "testdata/Test#Test.csv", nil, nil, 0, false)
	tests := []struct {
		name    string
		x       *CSVImporter
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "test",
			x:    importer,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.ExportExcel(); (err != nil) != tt.wantErr {
				t.Errorf("CSVImporter.Export2Excel() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parseCSVBookReaderOptions(t *testing.T) {
	type args struct {
		filename   string
		sheetNames []string
	}
	tests := []struct {
		name    string
		args    args
		want    *bookReaderOptions
		wantErr bool
	}{
		{
			name: "test",
			args: args{
				filename:   "testdata/Test#Item.csv",
				sheetNames: []string{},
			},
			want: &bookReaderOptions{
				Name:          "Test",
				Filename:      "testdata/Test#*.csv",
				MetasheetName: metasheet.DefaultMetasheetName,
				Sheets: []*sheetReaderOptions{
					{
						Name:     metasheet.DefaultMetasheetName,
						Filename: "testdata/Test#@TABLEAU.csv",
					},
					{
						Name:     "Hero",
						Filename: "testdata/Test#Hero.csv",
					},
					{
						Name:     "Item",
						Filename: "testdata/Test#Item.csv",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCSVBookReaderOptions(tt.args.filename, tt.args.sheetNames, metasheet.DefaultMetasheetName)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCSVBookReaderOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			assert.Equalf(t, tt.want, got, "bookReaderOptions should equal")
		})
	}
}

func Test_readCSVRows(t *testing.T) {
	type args struct {
		filename string
		topN     uint
	}
	tests := []struct {
		name     string
		args     args
		wantRows [][]string
		wantErr  bool
	}{
		{
			name: "read-all-rows",
			args: args{
				filename: "testdata/Test#Item.csv",
			},
			wantRows: [][]string{
				{"ID", "Name"},
				{"1", "Pike"},
				{"2", "Thompson"},
			},
		},
		{
			name: "read-top-2-rows",
			args: args{
				filename: "testdata/Test#Item.csv",
				topN:     2,
			},
			wantRows: [][]string{
				{"ID", "Name"},
				{"1", "Pike"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRows, err := readCSVRows(tt.args.filename, tt.args.topN)
			if (err != nil) != tt.wantErr {
				t.Errorf("readCSVRows() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotRows, tt.wantRows) {
				t.Errorf("readCSVRows() = %v, want %v", gotRows, tt.wantRows)
			}
		})
	}
}
