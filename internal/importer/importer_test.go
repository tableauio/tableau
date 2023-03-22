package importer

import (
	"log"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
)

func init() {
	err := fs.RangeFilesByFormat("./testdata", format.CSV, func(bookPath string) error {
		// log.Printf("path: %s", bookPath)
		imp, err := NewCSVImporter(bookPath, nil, nil)
		if err != nil {
			return err
		}
		return imp.ExportExcel()
	})
	if err != nil {
		log.Panicf("%+v", err)
	}
}

func Test_resolveBookPaths(t *testing.T) {
	type args struct {
		primaryBookPath string
		sheetName       string
		bookNameGlobs   []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "xlsx",
			args: args{
				primaryBookPath: "testdata/Test.xlsx",
				sheetName:       "Item",
				bookNameGlobs:   []string{"Test_*.xlsx"},
			},
			want: map[string]bool{
				"testdata/Test_Second.xlsx": true,
			},
		},
		{
			name: "csv",
			args: args{
				primaryBookPath: "testdata/Test#*.csv",
				sheetName:       "Item",
				bookNameGlobs:   []string{"Test_*.csv"},
			},
			want: map[string]bool{
				"testdata/Test_Second#*.csv": true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveBookPaths(tt.args.primaryBookPath, tt.args.sheetName, tt.args.bookNameGlobs)
			if (err != nil) != tt.wantErr {
				t.Errorf("resolveBookPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("resolveBookPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMergerImporters(t *testing.T) {
	type args struct {
		primaryBookPath string
		sheetName       string
		bookNameGlobs   []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string // book filenames
		wantErr bool
	}{
		{
			name: "xlsx",
			args: args{
				primaryBookPath: "testdata/Test.xlsx",
				sheetName:       "Item",
				bookNameGlobs:   []string{"Test_*.xlsx"},
			},
			want: []string{"testdata/Test_Second.xlsx"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMergerImporters(tt.args.primaryBookPath, tt.args.sheetName, tt.args.bookNameGlobs)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMergerImporters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			filenames := []string{}
			for _, imp := range got {
				filenames = append(filenames, fs.GetCleanSlashPath(imp.Filename()))
			}
			assert.ElementsMatch(t, tt.want, filenames, "got book filenames not match")
		})
	}
}

func TestGetScatterImporters(t *testing.T) {
	type args struct {
		primaryBookPath string
		sheetName       string
		bookNameGlobs   []string
	}
	tests := []struct {
		name    string
		args    args
		want    []string // book filenames
		wantErr bool
	}{
		{
			name: "csv",
			args: args{
				primaryBookPath: "testdata/Test#*.csv",
				sheetName:       "Item",
				bookNameGlobs:   []string{"Test_*.csv"},
			},
			want: []string{"testdata/Test_Second#*.csv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetScatterImporters(tt.args.primaryBookPath, tt.args.sheetName, tt.args.bookNameGlobs)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetScatterImporters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			filenames := []string{}
			for _, imp := range got {
				filenames = append(filenames, fs.GetCleanSlashPath(imp.Filename()))
			}
			assert.ElementsMatch(t, tt.want, filenames, "got book filenames not match")
		})
	}
}
