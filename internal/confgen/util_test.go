package confgen

import (
	"reflect"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
)

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

func Test_storeMessage(t *testing.T) {
	itemConf := &unittestpb.ItemConf{
		ItemMap: map[uint32]*unittestpb.Item{
			1: {Id: 1, Num: 10},
			2: {Id: 2, Num: 20},
			3: {Id: 3, Num: 30},
		},
	}
	type args struct {
		msg       proto.Message
		name      string
		outputDir string
		opt       *options.ConfOutputOption
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "export-item-conf",
			args: args{
				msg:       itemConf,
				name:      "ItemConfAlias",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Subdir:          "",
					Formats:         []format.Format{"json"},
					Pretty:          true,
					EmitUnpopulated: true,
				},
			},
			wantErr: false,
		},
		{
			name: "export-item-conf-subdir",
			args: args{
				msg:       itemConf,
				name:      "",
				outputDir: "_out/",
				opt: &options.ConfOutputOption{
					Subdir:          "subdir",
					Formats:         nil,
					Pretty:          true,
					EmitUnpopulated: true,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := storeMessage(tt.args.msg, tt.args.name, "UTC", tt.args.outputDir, tt.args.opt); (err != nil) != tt.wantErr {
				t.Errorf("storeMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parseOutputFormats(t *testing.T) {
	type args struct {
		msg proto.Message
		opt *options.ConfOutputOption
	}
	tests := []struct {
		name string
		args args
		want []format.Format
	}{
		{
			name: "default",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{},
			},
			want: []format.Format{format.JSON, format.Bin, format.Text},
		},
		{
			name: "global",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{
					Formats: []format.Format{format.Bin},
					MessagerFormats: map[string][]format.Format{
						"TaskConf": {format.JSON},
					},
				},
			},
			want: []format.Format{format.Bin},
		},
		{
			name: "messager-level",
			args: args{
				msg: &unittestpb.ItemConf{},
				opt: &options.ConfOutputOption{
					Formats: []format.Format{format.Bin},
					MessagerFormats: map[string][]format.Format{
						"TaskConf": {format.JSON},
						"ItemConf": {format.Text},
					},
				},
			},
			want: []format.Format{format.Text},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseOutputFormats(tt.args.msg, tt.args.opt); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseOutputFormats() = %v, want %v", got, tt.want)
			}
		})
	}
}
