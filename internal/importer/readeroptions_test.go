package importer

import (
	"reflect"
	"testing"

	"github.com/tableauio/tableau/internal/importer/metasheet"
)

func Test_wantSheet(t *testing.T) {
	type args struct {
		sheetName      string
		wantSheetNames []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "empty-wantSheetNames-and-wantSheetPattern",
			args: args{
				sheetName: "Sheet1",
			},
			want: true,
		},
		{
			name: "in-wantSheetNames",
			args: args{
				sheetName:      "Sheet1",
				wantSheetNames: []string{"Sheet1", "Sheet2"},
			},
			want: true,
		},
		{
			name: "in-wantSheetNames-pattern",
			args: args{
				sheetName:      "Sheet1",
				wantSheetNames: []string{"sheet", "Sheet*"},
			},
			want: true,
		},
		{
			name: "not-in-wantSheetNames",
			args: args{
				sheetName:      "ItemConf",
				wantSheetNames: []string{"Sheet1", "Sheet2"},
			},
			want: false,
		},
		{
			name: "not-in-wantSheetNames-pattern",
			args: args{
				sheetName:      "ItemConf",
				wantSheetNames: []string{"Sheet*"},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := wantSheet(tt.args.sheetName, tt.args.wantSheetNames); got != tt.want {
				t.Errorf("wantSheet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bookReaderOptions_GetMetasheet(t *testing.T) {
	tests := []struct {
		name string
		b    *bookReaderOptions
		want *sheetReaderOptions
	}{
		{
			name: "existed-metasheet",
			b: &bookReaderOptions{
				MetasheetName: metasheet.DefaultMetasheetName,
				Sheets: []*sheetReaderOptions{
					{Name: metasheet.DefaultMetasheetName, Filename: "testdata/Test#Item.csv"},
				},
			},
			want: &sheetReaderOptions{Name: metasheet.DefaultMetasheetName, Filename: "testdata/Test#Item.csv"},
		},
		{
			name: "not-existed-metasheet",
			b: &bookReaderOptions{
				MetasheetName: metasheet.DefaultMetasheetName,
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.GetMetasheet(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("bookReaderOptions.GetMetasheet() = %v, want %v", got, tt.want)
			}
		})
	}
}
