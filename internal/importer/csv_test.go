package importer

import (
	"testing"
)

func TestCSVImporter_ExportExcel(t *testing.T) {
	importer, _ := NewCSVImporter("testdata/Test#Test.csv", nil, nil)
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

func TestParseCSVFilenamePattern(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name          string
		args          args
		wantBookName  string
		wantSheetName string
		wantErr       bool
	}{
		{
			name: "case1",
			args: args{
				filename: "BookName#SheetName.csv",
			},
			wantBookName:  "BookName",
			wantSheetName: "SheetName",
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBookName, gotSheetName, err := ParseCSVFilenamePattern(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCSVFilenamePattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBookName != tt.wantBookName {
				t.Errorf("ParseCSVFilenamePattern() gotBookName = %v, want %v", gotBookName, tt.wantBookName)
			}
			if gotSheetName != tt.wantSheetName {
				t.Errorf("ParseCSVFilenamePattern() gotSheetName = %v, want %v", gotSheetName, tt.wantSheetName)
			}
		})
	}
}

func TestParseCSVBooknamePatternFrom(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "case1",
			args: args{
				filename: "BookName#SheetName.csv",
			},
			want:    "BookName#*.csv",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCSVBooknamePatternFrom(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCSVBooknamePatternFrom() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseCSVBooknamePatternFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}
