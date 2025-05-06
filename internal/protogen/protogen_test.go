package protogen

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
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
			name: "test1-FirstPassModeDefault",
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
								format.YAML, format.CSV,
							},
							FirstPassMode: "",
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
			args:    args{relWorkbookPaths: []string{"./testdata/yaml/Test.yaml", "./testdata/csv/Unittest#*.csv"}},
			wantErr: false,
		},
		{
			name: "test2-FirstPassModeNormal",
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
								format.YAML, format.CSV,
							},
							FirstPassMode: options.FirstPassModeNormal,
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
			args:    args{relWorkbookPaths: []string{"./testdata/yaml/Test.yaml", "./testdata/csv/Unittest#*.csv"}},
			wantErr: false,
		},
		{
			name: "test3-FirstPassModeAdvanced",
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
								format.YAML, format.CSV,
							},
							FirstPassMode: options.FirstPassModeAdvanced,
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
			args:    args{relWorkbookPaths: []string{"./testdata/yaml/Test.yaml", "./testdata/csv/Unittest#*.csv"}},
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
		want    []*internalpb.Worksheet
		wantErr bool
		errcode string
	}{
		{
			name: "MODE_ENUM_TYPE",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 5,
						MaxCol: 3,
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
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Fields: []*internalpb.Field{
						{
							Number: 0,
							Name:   "ITEM_TYPE_UNKNOWN",
							Alias:  "Unknown",
						},
						{
							Number: 1,
							Name:   "ITEM_TYPE_FRUIT",
							Alias:  "Fruit",
						},
						{
							Number: 2,
							Name:   "ITEM_TYPE_EQUIP",
							Alias:  "Equip",
						},
						{
							Number: 3,
							Name:   "ITEM_TYPE_BOX",
							Alias:  "Box",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_ENUM_TYPE no number",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 2,
						Rows: [][]string{
							{"Name", "Alias"},
							{"ITEM_TYPE_FRUIT", "Fruit"},
							{"ITEM_TYPE_EQUIP", "Equip"},
							{"ITEM_TYPE_BOX", "Box"},
						},
					},
				},
			},
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "ITEM_TYPE_FRUIT",
							Alias:  "Fruit",
						},
						{
							Number: 2,
							Name:   "ITEM_TYPE_EQUIP",
							Alias:  "Equip",
						},
						{
							Number: 3,
							Name:   "ITEM_TYPE_BOX",
							Alias:  "Box",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_ENUM_TYPE dup number",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 5,
						MaxCol: 3,
						Rows: [][]string{
							{"Number", "Name", "Alias"},
							{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
							{"1", "ITEM_TYPE_FRUIT", "Fruit"},
							{"2", "ITEM_TYPE_EQUIP", "Equip"},
							{"2", "ITEM_TYPE_BOX", "Box"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_ENUM_TYPE dup zero number",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 5,
						MaxCol: 3,
						Rows: [][]string{
							{"Number", "Name", "Alias"},
							{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
							{"0", "ITEM_TYPE_FRUIT", "Fruit"}, // duplicate
							{"1", "ITEM_TYPE_EQUIP", "Equip"},
							{"2", "ITEM_TYPE_BOX", "Box"},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_ENUM_TYPE dup name",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 5,
						MaxCol: 3,
						Rows: [][]string{
							{"Number", "Name", "Alias"},
							{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
							{"1", "ITEM_TYPE_FRUIT", "Fruit"},
							{"2", "ITEM_TYPE_EQUIP", "Equip"},
							{"3", "ITEM_TYPE_EQUIP", "Box"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_ENUM_TYPE dup alias",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 5,
						MaxCol: 3,
						Rows: [][]string{
							{"Number", "Name", "Alias"},
							{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
							{"1", "ITEM_TYPE_FRUIT", "Fruit"},
							{"2", "ITEM_TYPE_EQUIP", "Equip"},
							{"3", "ITEM_TYPE_BOX", "Equip"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_ENUM_TYPE_MULTI",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_ENUM_TYPE_MULTI,
				ws:   &internalpb.Worksheet{Name: "EnumDefault"},
				sheet: &book.Sheet{
					Name: "EnumDefault",
					Table: &book.Table{
						MaxRow: 11,
						MaxCol: 3,
						Rows: [][]string{
							{"ItemType", "Item's Type", ""},
							{"Number", "Name", "Alias"},
							{"0", "ITEM_TYPE_UNKNOWN", "Unknown"},
							{"1", "ITEM_TYPE_FRUIT", "Fruit"},
							{"2", "ITEM_TYPE_EQUIP", "Equip"},
							{"3", "ITEM_TYPE_BOX", "Box"},
							{"", "", ""},
							{"ModeType", "Mode's Type", ""},
							{"Alias", "Name", ""},
							{"Pvp", "MODE_TYPE_PVP", ""},
							{"Pve", "MODE_TYPE_PVE", ""},
						},
					},
				},
			},
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Note: "Item's Type",
					Fields: []*internalpb.Field{
						{
							Number: 0,
							Name:   "ITEM_TYPE_UNKNOWN",
							Alias:  "Unknown",
						},
						{
							Number: 1,
							Name:   "ITEM_TYPE_FRUIT",
							Alias:  "Fruit",
						},
						{
							Number: 2,
							Name:   "ITEM_TYPE_EQUIP",
							Alias:  "Equip",
						},
						{
							Number: 3,
							Name:   "ITEM_TYPE_BOX",
							Alias:  "Box",
						},
					},
				},
				{
					Name: "ModeType",
					Note: "Mode's Type",
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "MODE_TYPE_PVP",
							Alias:  "Pvp",
						},
						{
							Number: 2,
							Name:   "MODE_TYPE_PVE",
							Alias:  "Pve",
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
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 4,
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
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Fields: []*internalpb.Field{
						{
							// Note that field numbers are not set explicitly when parsing sheets.
							// They are auto generated when exporting to proto files finally.
							//
							// Number:   1,
							Name:     "id",
							Type:     "uint32",
							FullType: "uint32",
							Options: &tableaupb.FieldOptions{
								Name: "ID",
							},
						},
						{
							// Number:   2,
							Name:     "prop_map",
							Type:     "map<int32, string>",
							FullType: "map<int32, string>",
							MapEntry: &internalpb.Field_MapEntry{
								KeyType:       "int32",
								ValueType:     "string",
								ValueFullType: "string",
							},
							Options: &tableaupb.FieldOptions{
								Name:   "Prop",
								Layout: tableaupb.Layout_LAYOUT_INCELL,
							},
						},
						{
							// Number:   3,
							Name:     "feature_list",
							Type:     "repeated int32",
							FullType: "repeated int32",
							ListEntry: &internalpb.Field_ListEntry{
								ElemType:     "int32",
								ElemFullType: "int32",
							},
							Options: &tableaupb.FieldOptions{
								Name:   "Feature",
								Layout: tableaupb.Layout_LAYOUT_INCELL,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_STRUCT_TYPE dup name",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_STRUCT_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 2,
						Rows: [][]string{
							{"Name", "Type"},
							{"ID", "uint32"},
							{"Prop", "map<int32, string>"},
							{"Prop", "[]int32"}, // dupliacte
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_STRUCT_TYPE_MULTI",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_STRUCT_TYPE_MULTI,
				ws:   &internalpb.Worksheet{Name: "StuuctDefault"},
				sheet: &book.Sheet{
					Name: "StuuctDefault",
					Table: &book.Table{
						MaxRow: 11,
						MaxCol: 2,
						Rows: [][]string{
							{"ItemType", "Item's Type"},
							{"Name", "Type"},
							{"ID", "uint32"},
							{"Prop", "map<int32, string>"},
							{"Feature", "[]int32"},
							{"", ""},
							{"ModeType", "Mode's Type"},
							{"Type", "Name"},
							{"uint32", "ID"},
							{"string", "Name"},
							{"bool", "Valid"},
						},
					},
				},
			},
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Note: "Item's Type",
					Fields: []*internalpb.Field{
						{
							// Number:   1,
							Name:     "id",
							Type:     "uint32",
							FullType: "uint32",
							Options: &tableaupb.FieldOptions{
								Name: "ID",
							},
						},
						{
							// Number:   2,
							Name:     "prop_map",
							Type:     "map<int32, string>",
							FullType: "map<int32, string>",
							MapEntry: &internalpb.Field_MapEntry{
								KeyType:       "int32",
								ValueType:     "string",
								ValueFullType: "string",
							},
							Options: &tableaupb.FieldOptions{
								Name:   "Prop",
								Layout: tableaupb.Layout_LAYOUT_INCELL,
							},
						},
						{
							// Number:   3,
							Name:     "feature_list",
							Type:     "repeated int32",
							FullType: "repeated int32",
							ListEntry: &internalpb.Field_ListEntry{
								ElemType:     "int32",
								ElemFullType: "int32",
							},
							Options: &tableaupb.FieldOptions{
								Name:   "Feature",
								Layout: tableaupb.Layout_LAYOUT_INCELL,
							},
						},
					},
				},
				{
					Name: "ModeType",
					Note: "Mode's Type",
					Fields: []*internalpb.Field{
						{
							// Number:   1,
							Name:     "id",
							Type:     "uint32",
							FullType: "uint32",
							Options: &tableaupb.FieldOptions{
								Name: "ID",
							},
						},
						{
							// Number:   2,
							Name:     "name",
							Type:     "string",
							FullType: "string",
							Options: &tableaupb.FieldOptions{
								Name: "Name",
							},
						},
						{
							// Number:   3,
							Name:     "valid",
							Type:     "bool",
							FullType: "bool",
							Options: &tableaupb.FieldOptions{
								Name: "Valid",
							},
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
				ws:   &internalpb.Worksheet{Name: "ItemType"},
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
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "PvpBattle",
							Alias:  "SoloPVPBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "id",
									Type:     "uint32",
									FullType: "uint32",
									Options: &tableaupb.FieldOptions{
										Name: "ID",
									},
								},
								{
									// Number:   2,
									Name:     "damage",
									Type:     "int64",
									FullType: "int64",
									Options: &tableaupb.FieldOptions{
										Name: "Damage",
									},
								},
								{
									// Number:   3,
									Name:     "mission",
									Type:     "Mission",
									FullType: "Mission",
									Options: &tableaupb.FieldOptions{
										Name: "Mission",
										Span: tableaupb.Span_SPAN_INNER_CELL,
									},
									Fields: []*internalpb.Field{
										{
											// Number:   1,
											Name:     "id",
											Type:     "uint32",
											FullType: "uint32",
											Options: &tableaupb.FieldOptions{
												Name: "ID",
											},
										},
										{
											// Number:   2,
											Name:     "level",
											Type:     "int32",
											FullType: "int32",
											Options: &tableaupb.FieldOptions{
												Name: "Level",
											},
										},
									},
								},
							},
						},
						{
							Number: 2,
							Name:   "PveBattle",
							Alias:  "SoloPVEBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "prop_map",
									Type:     "map<int32, string>",
									FullType: "map<int32, string>",
									MapEntry: &internalpb.Field_MapEntry{
										KeyType:       "int32",
										ValueType:     "string",
										ValueFullType: "string",
									},
									Options: &tableaupb.FieldOptions{
										Name:   "Prop",
										Layout: tableaupb.Layout_LAYOUT_INCELL,
									},
								},
								{
									// Number:   2,
									Name:     "feature_list",
									Type:     "repeated int32",
									FullType: "repeated int32",
									ListEntry: &internalpb.Field_ListEntry{
										ElemType:     "int32",
										ElemFullType: "int32",
									},
									Options: &tableaupb.FieldOptions{
										Name:   "Feature",
										Layout: tableaupb.Layout_LAYOUT_INCELL,
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_UNION_TYPE with number",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 4,
						Rows: [][]string{
							{"Number", "Name", "Alias", "Field1"},
							{"2", "PvpBattle", "SoloPVPBattle", "ID\nuint32"},
							{"5", "PveBattle", "SoloPVEBattle", "Name\nstring"},
						},
					},
				},
			},
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Fields: []*internalpb.Field{
						{
							Number: 2,
							Name:   "PvpBattle",
							Alias:  "SoloPVPBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "id",
									Type:     "uint32",
									FullType: "uint32",
									Options: &tableaupb.FieldOptions{
										Name: "ID",
									},
								},
							},
						},
						{
							Number: 5,
							Name:   "PveBattle",
							Alias:  "SoloPVEBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "name",
									Type:     "string",
									FullType: "string",
									Options: &tableaupb.FieldOptions{
										Name: "Name",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "MODE_UNION_TYPE dup number",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 4,
						Rows: [][]string{
							{"Number", "Name", "Alias", "Field1"},
							{"1", "PvpBattle", "SoloPVPBattle", "ID\nuint32"},
							{"1", "PveBattle", "SoloPVEBattle", "Name\nstring"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_UNION_TYPE dup name",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 3,
						Rows: [][]string{
							{"Name", "Alias", "Field1"},
							{"Battle", "SoloPvpBattle", "ID\nuint32"},
							{"Battle", "SoloPveBattle", "Name\nstring"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_UNION_TYPE dup alias",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE,
				ws:   &internalpb.Worksheet{Name: "ItemType"},
				sheet: &book.Sheet{
					Name: "ItemType",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 3,
						Rows: [][]string{
							{"Name", "Alias", "Field1"},
							{"PvpBattle", "SoloBattle", "ID\nuint32"},
							{"PveBattle", "SoloBattle", "Name\nstring"}, // duplicate
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name: "MODE_UNION_TYPE_MULTI",
			gen:  testgen,
			args: args{
				mode: tableaupb.Mode_MODE_UNION_TYPE_MULTI,
				ws:   &internalpb.Worksheet{Name: "UnionDefault"},
				sheet: &book.Sheet{
					Name: "UnionDefault",
					Table: &book.Table{
						MaxRow: 9,
						MaxCol: 5,
						Rows: [][]string{
							{"ItemType", "Item's Type", "", "", ""},
							{"Name", "Alias", "Field1", "Field2", "Field3"},
							{"PvpBattle", "SoloPVPBattle", "ID\nuint32", "Damage\nint64", "Mission\n{uint32 ID, int32 Level}Mission"},
							{"PveBattle", "SoloPVEBattle", "Prop\nmap<int32, string>", "Feature\n[]int32"},
							{"", "", "", "", ""},
							{"ModeType", "Mode's Type", "", "", ""},
							{"Field3", "Name", "Field1", "Alias", "Field2"},
							{"", "PVP", "ID\nuint32", "PvpMode", "Difficulty\nint32"},
							{"", "PVE", "Name\nstring", "PveMode", "Score\nint32"},
						},
					},
				},
			},
			want: []*internalpb.Worksheet{
				{
					Name: "ItemType",
					Note: "Item's Type",
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "PvpBattle",
							Alias:  "SoloPVPBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "id",
									Type:     "uint32",
									FullType: "uint32",
									Options: &tableaupb.FieldOptions{
										Name: "ID",
									},
								},
								{
									// Number:   2,
									Name:     "damage",
									Type:     "int64",
									FullType: "int64",
									Options: &tableaupb.FieldOptions{
										Name: "Damage",
									},
								},
								{
									// Number:   3,
									Name:     "mission",
									Type:     "Mission",
									FullType: "Mission",
									Options: &tableaupb.FieldOptions{
										Name: "Mission",
										Span: tableaupb.Span_SPAN_INNER_CELL,
									},
									Fields: []*internalpb.Field{
										{
											// Number:   1,
											Name:     "id",
											Type:     "uint32",
											FullType: "uint32",
											Options: &tableaupb.FieldOptions{
												Name: "ID",
											},
										},
										{
											// Number:   2,
											Name:     "level",
											Type:     "int32",
											FullType: "int32",
											Options: &tableaupb.FieldOptions{
												Name: "Level",
											},
										},
									},
								},
							},
						},
						{
							Number: 2,
							Name:   "PveBattle",
							Alias:  "SoloPVEBattle",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "prop_map",
									Type:     "map<int32, string>",
									FullType: "map<int32, string>",
									MapEntry: &internalpb.Field_MapEntry{
										KeyType:       "int32",
										ValueType:     "string",
										ValueFullType: "string",
									},
									Options: &tableaupb.FieldOptions{
										Name:   "Prop",
										Layout: tableaupb.Layout_LAYOUT_INCELL,
									},
								},
								{
									// Number:   2,
									Name:     "feature_list",
									Type:     "repeated int32",
									FullType: "repeated int32",
									ListEntry: &internalpb.Field_ListEntry{
										ElemType:     "int32",
										ElemFullType: "int32",
									},
									Options: &tableaupb.FieldOptions{
										Name:   "Feature",
										Layout: tableaupb.Layout_LAYOUT_INCELL,
									},
								},
							},
						},
					},
				},
				{
					Name: "ModeType",
					Note: "Mode's Type",
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "PVP",
							Alias:  "PvpMode",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "id",
									Type:     "uint32",
									FullType: "uint32",
									Options: &tableaupb.FieldOptions{
										Name: "ID",
									},
								},
								{
									// Number:   2,
									Name:     "difficulty",
									Type:     "int32",
									FullType: "int32",
									Options: &tableaupb.FieldOptions{
										Name: "Difficulty",
									},
								},
							},
						},
						{
							Number: 2,
							Name:   "PVE",
							Alias:  "PveMode",
							Fields: []*internalpb.Field{
								{
									// Number:   1,
									Name:     "name",
									Type:     "string",
									FullType: "string",
									Options: &tableaupb.FieldOptions{
										Name: "Name",
									},
								},
								{
									// Number:   2,
									Name:     "score",
									Type:     "int32",
									FullType: "int32",
									Options: &tableaupb.FieldOptions{
										Name: "Score",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.gen.parseSpecialSheetMode(tt.args.mode, tt.args.ws, tt.args.sheet, "", "")
			if (err != nil) != tt.wantErr {
				t.Errorf("Generator.parseSpecialSheetMode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			} else if len(got) != len(tt.want) {
				t.Errorf("Generator.parseSpecialSheetMode() size = %v, want size %v", len(got), len(tt.want))
			} else {
				for i := range got {
					if !proto.Equal(got[i], tt.want[i]) {
						t.Errorf("Generator.parseSpecialSheetMode()[%d] = %v\n want[%d] %v", i, got[i], i, tt.want[i])
					}
				}
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
