package importer

import (
	"context"
	"log"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/x/xfs"
)

func init() {
	err := xfs.RangeFilesByFormat("./testdata", format.CSV, func(bookPath string) error {
		// log.Printf("path: %s", bookPath)
		imp, err := NewCSVImporter(context.Background(), bookPath)
		if err != nil {
			return err
		}
		return imp.ExportExcel()
	})
	if err != nil {
		log.Panicf("%+v", err)
	}
}

func Test_ResolveSheetSpecifier(t *testing.T) {
	type args struct {
		inputDir        string
		primaryBookName string
		sheetName       string
		sheetSpecifier  string
		subdirRewrites  map[string]string
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
				inputDir:        ".",
				primaryBookName: "testdata/Test.xlsx",
				sheetName:       "Item",
				sheetSpecifier:  "Test_*.xlsx",
				subdirRewrites:  nil,
			},
			want: map[string]bool{
				"testdata/Test_Second.xlsx": true,
			},
		},
		{
			name: "csv",
			args: args{
				inputDir:        ".",
				primaryBookName: "testdata/Test#*.csv",
				sheetName:       "Item",
				sheetSpecifier:  "Test_*.csv",
				subdirRewrites:  nil,
			},
			want: map[string]bool{
				"testdata/Test_Second#*.csv": true,
			},
		},
		{
			name: "xml",
			args: args{
				inputDir:        ".",
				primaryBookName: "testdata/Test.xml",
				sheetName:       "Item",
				sheetSpecifier:  "Test_*.xml",
				subdirRewrites:  nil,
			},
			want: map[string]bool{
				"testdata/Test_Second.xml": true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := ResolveSheetSpecifier(tt.args.inputDir, tt.args.primaryBookName, tt.args.sheetSpecifier, tt.args.subdirRewrites)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveSheetSpecifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ResolveSheetSpecifier() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMergerImporters(t *testing.T) {
	type args struct {
		primaryBookName string
		sheetName       string
		sheetSpecifiers []string
		subdirRewrites  map[string]string
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
				primaryBookName: "testdata/Test.xlsx",
				sheetName:       "Item",
				sheetSpecifiers: []string{"Test_*.xlsx"},
				subdirRewrites:  nil,
			},
			want: []string{"testdata/Test_Second.xlsx"},
		},
		{
			name: "xml",
			args: args{
				primaryBookName: "testdata/Test.xml",
				sheetName:       "Item",
				sheetSpecifiers: []string{"Test_*.xml"},
				subdirRewrites:  nil,
			},
			want: []string{"testdata/Test_Second.xml"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetMergerImporters(context.Background(), ".", tt.args.primaryBookName, tt.args.sheetName, tt.args.sheetSpecifiers, tt.args.subdirRewrites)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetMergerImporters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			filenames := []string{}
			for _, imp := range got {
				filenames = append(filenames, xfs.CleanSlashPath(imp.Filename()))
			}
			assert.ElementsMatch(t, tt.want, filenames, "got book filenames not match")
		})
	}
}

func TestGetScatterImporters(t *testing.T) {
	type args struct {
		primaryBookName string
		sheetName       string
		sheetSpecifiers []string
		subdirRewrites  map[string]string
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
				primaryBookName: "testdata/Test#*.csv",
				sheetName:       "Item",
				sheetSpecifiers: []string{"Test_*.csv"},
				subdirRewrites:  map[string]string{},
			},
			want: []string{"testdata/Test_Second#*.csv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetScatterImporters(context.Background(), ".", tt.args.primaryBookName, tt.args.sheetName, tt.args.sheetSpecifiers, tt.args.subdirRewrites)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetScatterImporters() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			filenames := []string{}
			for _, imp := range got {
				filenames = append(filenames, xfs.CleanSlashPath(imp.Filename()))
			}
			assert.ElementsMatch(t, tt.want, filenames, "got book filenames not match")
		})
	}
}
