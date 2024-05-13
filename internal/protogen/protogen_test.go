package protogen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
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
			Output: &options.ProtoOutputOption{},
		},
	})
}

func TestGenerator_parseSpecialSheetMode(t *testing.T) {
	type args struct {
		mode  tableaupb.Mode
		ws    *tableaupb.Worksheet
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		gen     *Generator
		args    args
		wantErr bool
	}{
		{
			name: "MODE_ENUM_TYPE",
			gen:  testgen,
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
			},
			wantErr: false,
		},
		{
			name: "MODE_STRUCT_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_STRUCT_TYPE,
				ws:   &tableaupb.Worksheet{},
				sheet: &book.Sheet{
					Name:   "ItemType",
					MaxRow: 3,
					MaxCol: 2,
					Rows: [][]string{
						{"Name", "Type"},
						{"ID", "uint32"},
						{"Prop", "map<int32, string>"},
						{"Feature", "[]int32"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_UNION_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &tableaupb.Worksheet{},
				sheet: &book.Sheet{
					Name:   "ItemType",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"Name", "Alias", "Field1", "Field2", "Field3"},
						{"PvpBattle", "SoloPVPBattle", "ID\nuint32", "Damage\nint64", "Mission\n{uint32 ID, int32 Level}Mission"},
						{"PveBattle", "SoloPVEBattle", "Prop\nmap<int32, string>", "Feature\n[]int32"},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.parseSpecialSheetMode(tt.args.mode, tt.args.ws, tt.args.sheet, "", ""); (err != nil) != tt.wantErr {
				t.Errorf("Generator.parseSpecialSheetMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_extractTypeInfoFromSpecialSheetMode(t *testing.T) {
	type args struct {
		mode           tableaupb.Mode
		sheet          *book.Sheet
		typeName       string
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
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
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
				typeName:       "ItemType",
				parentFilename: "test.proto",
			},
			wantErr: false,
		},
		{
			name: "MODE_STRUCT_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_STRUCT_TYPE,
				sheet: &book.Sheet{
					Name:   "TaskReward",
					MaxRow: 3,
					MaxCol: 2,
					Rows: [][]string{
						{"Name", "Type"},
						{"ID", "uint32"},
						{"Prop", "map<int32, string>"},
						{"Feature", "[]int32"},
					},
				},
				typeName:       "TaskReward",
				parentFilename: "test.proto",
			},
			wantErr: false,
		},
		{
			name: "MODE_UNION_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				sheet: &book.Sheet{
					Name:   "TaskTarget",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"Name", "Alias", "Field1", "Field2", "Field3"},
						{"PvpBattle", "SoloPVPBattle", "ID\nuint32", "Damage\nint64", "Mission\n{uint32 ID, int32 Level}Mission"},
						{"PveBattle", "SoloPVEBattle", "Prop\nmap<int32, string>", "Feature\n[]int32"},
					},
				},
				typeName:       "TaskTarget",
				parentFilename: "test.proto",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.extractTypeInfoFromSpecialSheetMode(tt.args.mode, tt.args.sheet, tt.args.typeName, tt.args.parentFilename); (err != nil) != tt.wantErr {
				t.Errorf("Generator.extractTypeInfoFromSpecialSheetMode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func prepareOutput() error {
	// prepare output common dir
	outdir := "./testdata/_proto"
	err := os.MkdirAll(outdir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	outCommDir := filepath.Join(outdir, "common")
	err = os.MkdirAll(outCommDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create output common dir: %v", err)
	}

	srcCommDir := "../../test/functest/proto/common"
	dirEntries, err := os.ReadDir(srcCommDir)
	if err != nil {
		return fmt.Errorf("read dir failed: %+v", err)
	}
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			src := filepath.Join(srcCommDir, entry.Name())
			dst := filepath.Join(outCommDir, entry.Name())
			if err := fs.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy file failed: %+v", err)
			}
		}
	}
	return nil
}

func TestGenerator_GenAll(t *testing.T) {
	if err := prepareOutput(); err != nil {
		t.Fatalf("failed to create output common dir: %v", err)
	}
	outdir := "./testdata/_proto"
	tests := []struct {
		name    string
		gen     *Generator
		wantErr bool
	}{
		{
			name: "test1",
			gen: NewGenerator("protoconf", "../../test/functest/testdata/", outdir,
				options.Proto(
					&options.ProtoOption{
						Input: &options.ProtoInputOption{
							ProtoPaths: []string{outdir},
							ProtoFiles: []string{
								"common/base.proto",
								"common/common.proto",
								"common/union.proto",
							},
							Formats: []format.Format{
								// format.Excel,
								format.CSV,
								format.XML,
							},
							Header: &options.HeaderOption{
								Namerow: 1,
								Typerow: 2,
								Noterow: 3,
								Datarow: 4,
							},
						},
						Output: &options.ProtoOutputOption{
							FilenameWithSubdirPrefix: true,
							FileOptions: map[string]string{
								"go_package": "github.com/tableauio/tableau/test/functest/protoconf",
							},
						},
					},
				),
			),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.GenAll(); (err != nil) != tt.wantErr {
				t.Errorf("Generator.GenAll() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
