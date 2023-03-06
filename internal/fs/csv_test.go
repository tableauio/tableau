package fs

import "testing"

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
