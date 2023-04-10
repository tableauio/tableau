package protogen

import (
	"testing"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

var testgen *Generator

func init() {
	testgen = NewGeneratorWithOptions("protoconf", "testdata", "testdata", &options.Options{
		LocationName: "Asia/Shanghai",
		Proto: &options.ProtoOption{
			Input: &options.ProtoInputOption{
				MetasheetName: "",
			},
		},
	})
}

func TestGenerator_parseSpecialSheetMode(t *testing.T) {
	type args struct {
		mode           tableaupb.Mode
		ws             *tableaupb.Worksheet
		sheet          *book.Sheet
		parentFilename string
	}
	tests := []struct {
		name    string
		gen     *Generator
		args    args
		wantErr bool
	}{
		{
			name: "MODE_ENUM_TYPE",
			gen: testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &tableaupb.Worksheet{},
				sheet: &book.Sheet{
					Name:   "ItemType",
					MaxRow: 3,
					MaxCol: 4,
					Rows: [][]string{
						{"Number", "Name", "Alias"},
						{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
						{"1", "ITEM_TYPE_FRUIT", "Fruit"},
						{"2", "ITEM_TYPE_EQUIP", "Equip"},
						{"3", "ITEM_TYPE_BOX", "Box"},
					},
				},
				parentFilename: "ItemConf.proto",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.parseSpecialSheetMode(tt.args.mode, tt.args.ws, tt.args.sheet, tt.args.parentFilename); (err != nil) != tt.wantErr {
				t.Errorf("Generator.parseSpecialSheetMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
