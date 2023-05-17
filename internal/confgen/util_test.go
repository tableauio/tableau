package confgen

import "testing"

func Test_parseBookSpecifier(t *testing.T) {
	type args struct {
		bookSpecifier string
	}
	tests := []struct {
		name          string
		args          args
		wantBookName  string
		wantSheetName string
		wantErr       bool
	}{
		{
			name: "xlsx-only-workbook",
			args: args{
				bookSpecifier: "testdata/excel/Item.xlsx",
			},
			wantBookName:  "testdata/excel/Item.xlsx",
			wantSheetName: "",
			wantErr:       false,
		},
		{
			name: "xlsx-with-sheet",
			args: args{
				bookSpecifier: "testdata/excel/Item.xlsx#Item",
			},
			wantBookName:  "testdata/excel/Item.xlsx",
			wantSheetName: "Item",
			wantErr:       false,
		},
		{
			name: "dir-path-with-special-char-#",
			args: args{
				bookSpecifier: "testdata/excel#dir/Item.xlsx#Item",
			},
			wantBookName:  "testdata/excel#dir/Item.xlsx",
			wantSheetName: "Item",
			wantErr:       false,
		},
		{
			name: "csv-only-workbook",
			args: args{
				bookSpecifier: "testdata/csv/Item#Item.csv",
			},
			wantBookName:  "testdata/csv/Item#*.csv",
			wantSheetName: "",
			wantErr:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotBookName, gotSheetName, err := parseBookSpecifier(tt.args.bookSpecifier)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBookSpecifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotBookName != tt.wantBookName {
				t.Errorf("parseBookSpecifier() gotBookName = %v, want %v", gotBookName, tt.wantBookName)
			}
			if gotSheetName != tt.wantSheetName {
				t.Errorf("parseBookSpecifier() gotSheetName = %v, want %v", gotSheetName, tt.wantSheetName)
			}
		})
	}
}
