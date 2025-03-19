package protogen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
)

const outdir = "./testdata/_proto/default"

var testgen *Generator

func TestMain(m *testing.M) {
	testgen = NewGeneratorWithOptions("protoconf", "testdata", "testdata", &options.Options{
		LocationName: "Asia/Shanghai",
		Proto: &options.ProtoOption{
			Input: &options.ProtoInputOption{
				MetasheetName: "",
			},
			Output: &options.ProtoOutputOption{},
		},
	})
	m.Run()
}

func prepareOutput() error {
	// prepare output common dir
	err := os.MkdirAll(outdir, xfs.DefaultDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create output dir: %v", err)
	}
	outCommDir := filepath.Join(outdir, "common")
	err = os.MkdirAll(outCommDir, xfs.DefaultDirPerm)
	if err != nil {
		return fmt.Errorf("failed to create output common dir: %v", err)
	}

	srcCommDir := "../../test/functest/proto/default/common"
	dirEntries, err := os.ReadDir(srcCommDir)
	if err != nil {
		return fmt.Errorf("read dir failed: %+v", err)
	}
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			src := filepath.Join(srcCommDir, entry.Name())
			dst := filepath.Join(outCommDir, entry.Name())
			if err := xfs.CopyFile(src, dst); err != nil {
				return fmt.Errorf("copy file failed: %+v", err)
			}
		}
	}
	return nil
}

func TestGenerator_GenWorkbook(t *testing.T) {
	err := prepareOutput()
	assert.NoError(t, err)

	type args struct {
		relWorkbookPaths []string
	}
	tests := []struct {
		name    string
		gen     *Generator
		args    args
		wantErr bool
	}{
		{
			name: "test1",
			gen: NewGenerator("protoconf", "./", outdir,
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
								format.YAML,
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
			args:    args{relWorkbookPaths: []string{"./testdata/yaml/Test.yaml"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.gen.GenWorkbook(tt.args.relWorkbookPaths...); (err != nil) != tt.wantErr {
				t.Errorf("Generator.GenWorkbook() error = %+v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerator_parseSpecialSheetMode(t *testing.T) {
	type args struct {
		mode  tableaupb.Mode
		ws    *internalpb.Worksheet
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
				ws:   &internalpb.Worksheet{},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
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
			},
			wantErr: false,
		},
		{
			name: "MODE_STRUCT_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_STRUCT_TYPE,
				ws:   &internalpb.Worksheet{},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
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
			},
			wantErr: false,
		},
		{
			name: "MODE_UNION_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &internalpb.Worksheet{},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"Name", "Alias", "Field1", "Field2", "Field3"},
							{"PvpBattle", "SoloPVPBattle", "ID\nuint32", "Damage\nint64", "Mission\n{uint32 ID, int32 Level}Mission"},
							{"PveBattle", "SoloPVEBattle", "Prop\nmap<int32, string>", "Feature\n[]int32"},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.gen.parseSpecialSheetMode(tt.args.mode, tt.args.ws, tt.args.sheet, "", ""); (err != nil) != tt.wantErr {
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
					Name: "ItemType",
					Table: &book.Table{
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
					Name: "TaskReward",
					Table: &book.Table{
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
					Name: "TaskTarget",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"Name", "Alias", "Field1", "Field2", "Field3"},
							{"PvpBattle", "SoloPVPBattle", "ID\nuint32", "Damage\nint64", "Mission\n{uint32 ID, int32 Level}Mission"},
							{"PveBattle", "SoloPVEBattle", "Prop\nmap<int32, string>", "Feature\n[]int32"},
						},
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
