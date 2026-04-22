package protogen

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"buf.build/gen/go/bufbuild/protovalidate/protocolbuffers/go/buf/validate"
	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/printer"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/internal/x/xproto/protoc"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

func Test_genFieldOptionsString(t *testing.T) {
	type args struct {
		opts *tableaupb.FieldOptions
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "only-name",
			args: args{
				opts: &tableaupb.FieldOptions{
					Name: "ItemID",
				},
			},
			want: `[(tableau.field) = {name:"ItemID"}]`,
		},
		{
			name: "name-and-prop",
			args: args{
				opts: &tableaupb.FieldOptions{
					Name: "ItemID",
					Prop: &tableaupb.FieldProp{
						Unique: proto.Bool(true),
					},
				},
			},
			want: `[(tableau.field) = {name:"ItemID" prop:{unique:true}}]`,
		},
		{
			name: "name-prop-and-json-name",
			args: args{
				opts: &tableaupb.FieldOptions{
					Name: "ItemID",
					Prop: &tableaupb.FieldProp{
						Unique:   proto.Bool(true),
						JsonName: "item_id_1",
					},
				},
			},
			want: `[(tableau.field) = {name:"ItemID" prop:{unique:true}}, json_name="item_id_1"]`,
		},
		{
			name: "name-and-prop-json_name",
			args: args{
				opts: &tableaupb.FieldOptions{
					Name: "ItemID",
					Prop: &tableaupb.FieldProp{
						JsonName: "item_id_1",
					},
				},
			},
			want: `[(tableau.field) = {name:"ItemID"}, json_name="item_id_1"]`,
		},
	}
	be := &bookExporter{gen: &Generator{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := be.genFieldOptionsString(tt.args.opts, nil); got != tt.want {
				t.Errorf("genFieldOptionsString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_genFieldOptionsString_predefinedValidateRule(t *testing.T) {
	// Parse the custom_rules.proto which extends buf.validate.Int32Rules
	// with a custom "is_zero" field.
	registryFiles, err := protoc.NewFiles(
		[]string{"testdata"},
		[]string{"testdata/custom_rules.proto"},
	)
	assert.NoError(t, err)
	registryTypes := dynamicpb.NewTypes(registryFiles)

	be := &bookExporter{gen: &Generator{ProtoRegistryTypes: registryTypes}}

	fieldValidate := `int32:{[testpkg.is_zero]:true}`
	opts := &tableaupb.FieldOptions{Name: "FieldX"}
	// Unmarshal validate into FieldRules first, then pass to genFieldOptionsString.
	fieldRules := &validate.FieldRules{}
	err = be.unmarshalFromText(fieldRules, fieldValidate)
	assert.NoError(t, err)
	got := be.genFieldOptionsString(opts, fieldRules)
	want := `[(tableau.field) = {name:"FieldX"}, (buf.validate.field) = {int32:{[testpkg.is_zero]:true}}]`
	assert.Equal(t, want, got)
}

func Test_isSameFieldMessageType(t *testing.T) {
	type args struct {
		left  *internalpb.Field
		right *internalpb.Field
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "both-are-nil",
			args: args{
				left:  nil,
				right: nil,
			},
			want: true,
		},
		{
			name: "one-is-nil",
			args: args{
				left:  &internalpb.Field{},
				right: nil,
			},
			want: true,
		},
		{
			name: "one-sub-fields-nil",
			args: args{
				left: &internalpb.Field{
					Fields: nil,
				},
				right: &internalpb.Field{
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "Item",
							Alias:  "RewardItem",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "not-equal-length-of-sub-fields",
			args: args{
				left: &internalpb.Field{
					Fields: []*internalpb.Field{
						{
							Number: 1,
						},
						{
							Number: 2,
						},
					},
				},
				right: &internalpb.Field{
					Fields: []*internalpb.Field{
						{
							Number: 1,
						},
					},
				},
			},
			want: false,
		},
		{
			name: "not-same-type",
			args: args{
				left: &internalpb.Field{
					Type: "Item",
				},
				right: &internalpb.Field{
					Type: "Drop",
				},
			},
			want: false,
		},
		{
			name: "same-sub-fields",
			args: args{
				left: &internalpb.Field{
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "Item",
							Alias:  "RewardItem",
						},
					},
				},
				right: &internalpb.Field{
					Fields: []*internalpb.Field{
						{
							Number: 1,
							Name:   "Item",
							Alias:  "RewardItem",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSameFieldMessageType(tt.args.left, tt.args.right); got != tt.want {
				t.Errorf("isSameFieldMessageType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_marshalToText(t *testing.T) {
	type args struct {
		m proto.Message
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "FieldOptions",
			args: args{
				m: &tableaupb.FieldOptions{
					Key:    "ID",
					Layout: tableaupb.Layout_LAYOUT_VERTICAL,
					Prop: &tableaupb.FieldProp{
						Unique:  proto.Bool(true),
						Refer:   "ItemConf.ID",
						Present: true,
					},
				},
			},
			want: `key:"ID" layout:LAYOUT_VERTICAL prop:{unique:true refer:"ItemConf.ID" present:true}`,
		},
	}
	be := &bookExporter{gen: &Generator{}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := be.marshalToText(tt.args.m); got != tt.want {
				t.Errorf("marshalToText() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bookExporter_GetProtoFilePath(t *testing.T) {
	tests := []struct {
		name string
		x    *bookExporter
		want string
	}{
		{
			name: "name-and-prop",
			x: &bookExporter{
				wb: &internalpb.Workbook{
					Name: "name",
				},
				FilenameSuffix: "_conf",
			},
			want: `name_conf.proto`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.x.GetProtoFilePath(); got != tt.want {
				t.Errorf("bookExporter.GetProtoFilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_bookExporter_export(t *testing.T) {
	tests := []struct {
		name             string
		edition          string
		protoFileOptions map[string]string
		wantContains     []string
		wantNotContains  []string
		// wantOrderedLines asserts the given lines appear in the generated
		// proto file in the specified relative order.
		wantOrderedLines []string
	}{
		{
			name: "proto3",
			protoFileOptions: map[string]string{
				"go_package": `"github.com/example/protoconf"`,
			},
			wantContains: []string{
				`syntax = "proto3";`,
				`option go_package = "github.com/example/protoconf";`,
			},
		},
		{
			name:    "edition-2023-with-features-utf8-validation",
			edition: "2023",
			protoFileOptions: map[string]string{
				"go_package":               `"github.com/example/protoconf"`,
				"features.utf8_validation": "NONE",
			},
			wantContains: []string{
				`edition = "2023";`,
				`option go_package = "github.com/example/protoconf";`,
				`option features.utf8_validation = NONE;`,
			},
			wantNotContains: []string{
				`syntax = "proto3";`,
			},
		},
		{
			name:    "edition-2024-with-features-strip-enum-prefix",
			edition: "2024",
			protoFileOptions: map[string]string{
				"go_package":                         `"github.com/example/protoconf"`,
				"features.(pb.go).strip_enum_prefix": "STRIP_ENUM_PREFIX_STRIP",
			},
			wantContains: []string{
				`edition = "2024";`,
				`option go_package = "github.com/example/protoconf";`,
				`option features.(pb.go).strip_enum_prefix = STRIP_ENUM_PREFIX_STRIP;`,
				// go features option implies importing go_features.proto.
				`import "google/protobuf/go_features.proto";`,
			},
			wantNotContains: []string{
				`syntax = "proto3";`,
				// must NOT import unrelated features proto files.
				`import "google/protobuf/cpp_features.proto";`,
				`import "google/protobuf/java_features.proto";`,
			},
		},
		{
			name:    "edition-2024-with-pb-cpp-features-infer-cpp-features-import",
			edition: "2024",
			protoFileOptions: map[string]string{
				"go_package":                    `"github.com/example/protoconf"`,
				"features.(pb.cpp).string_type": "VIEW",
			},
			wantContains: []string{
				`edition = "2024";`,
				`option features.(pb.cpp).string_type = VIEW;`,
				`import "google/protobuf/cpp_features.proto";`,
			},
			wantNotContains: []string{
				`import "google/protobuf/go_features.proto";`,
				`import "google/protobuf/java_features.proto";`,
			},
		},
		{
			name:    "edition-2024-with-pb-java-features-infer-java-features-import",
			edition: "2024",
			protoFileOptions: map[string]string{
				"go_package":                         `"github.com/example/protoconf"`,
				"features.(pb.java).utf8_validation": "VERIFY",
			},
			wantContains: []string{
				`edition = "2024";`,
				`option features.(pb.java).utf8_validation = VERIFY;`,
				`import "google/protobuf/java_features.proto";`,
			},
			wantNotContains: []string{
				`import "google/protobuf/go_features.proto";`,
				`import "google/protobuf/cpp_features.proto";`,
			},
		},
		{
			name:    "edition-2024-with-all-language-features-infer-all-imports",
			edition: "2024",
			protoFileOptions: map[string]string{
				"go_package":                            `"github.com/example/protoconf"`,
				"features.(pb.go).api_level":            "API_OPAQUE",
				"features.(pb.cpp).legacy_closed_enum":  "true",
				"features.(pb.java).legacy_closed_enum": "true",
			},
			wantContains: []string{
				`import "google/protobuf/go_features.proto";`,
				`import "google/protobuf/cpp_features.proto";`,
				`import "google/protobuf/java_features.proto";`,
				`option features.(pb.cpp).legacy_closed_enum = true;`,
				`option features.(pb.go).api_level = API_OPAQUE;`,
				`option features.(pb.java).legacy_closed_enum = true;`,
			},
		},
		{
			name: "options-sorted-alphabetically-by-key",
			protoFileOptions: map[string]string{
				// intentionally define keys in a non-alphabetical order
				// to verify the output is sorted by key.
				"java_package":     `"com.example.protoconf"`,
				"go_package":       `"github.com/example/protoconf"`,
				"csharp_namespace": `"Example.Protoconf"`,
				"cc_enable_arenas": "true",
				"optimize_for":     "SPEED",
			},
			wantContains: []string{
				`option cc_enable_arenas = true;`,
				`option csharp_namespace = "Example.Protoconf";`,
				`option go_package = "github.com/example/protoconf";`,
				`option java_package = "com.example.protoconf";`,
				`option optimize_for = SPEED;`,
			},
			// verify the output order strictly matches alphabetical order
			// of the keys: cc_enable_arenas < csharp_namespace < go_package
			// < java_package < optimize_for
			wantOrderedLines: []string{
				`option cc_enable_arenas = true;`,
				`option csharp_namespace = "Example.Protoconf";`,
				`option go_package = "github.com/example/protoconf";`,
				`option java_package = "com.example.protoconf";`,
				`option optimize_for = SPEED;`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			gen := &Generator{
				ctx: context.Background(),
				InputOpt: &options.ProtoInputOption{
					MessagerPattern: `Conf$`,
				},
				OutputOpt: &options.ProtoOutputOption{},
			}
			wb := &internalpb.Workbook{
				Name: "item",
				Options: &tableaupb.WorkbookOptions{
					Name: "item.xlsx",
				},
				Worksheets: []*internalpb.Worksheet{
					{
						Name: "ItemConf",
						Options: &tableaupb.WorksheetOptions{
							Name: "ItemConf",
						},
						Fields: []*internalpb.Field{
							{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
							{Name: "name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "Name"}},
						},
					},
				},
			}
			be := newBookExporter("protoconf", tt.edition, tt.protoFileOptions, tmpDir, "", wb, gen)
			err := be.export(false)
			assert.NoError(t, err)

			// read the generated file and verify
			content, err := os.ReadFile(filepath.Join(tmpDir, "item.proto"))
			assert.NoError(t, err)
			got := string(content)
			for _, want := range tt.wantContains {
				assert.Contains(t, got, want, "expected proto file to contain: %s", want)
			}
			for _, notWant := range tt.wantNotContains {
				assert.NotContains(t, got, notWant, "expected proto file NOT to contain: %s", notWant)
			}
			// verify that the expected lines appear in the given relative order.
			if len(tt.wantOrderedLines) >= 2 {
				prevIdx := -1
				var prevLine string
				for _, line := range tt.wantOrderedLines {
					idx := strings.Index(got, line)
					assert.GreaterOrEqual(t, idx, 0, "expected proto file to contain: %s", line)
					assert.Greater(t, idx, prevIdx,
						"expected line %q to appear after %q in generated proto file", line, prevLine)
					prevIdx = idx
					prevLine = line
				}
			}
		})
	}
}

func Test_sheetExporter_exportEnum(t *testing.T) {
	tests := []struct {
		name    string
		x       *sheetExporter
		want    string
		wantErr bool
	}{
		{
			name: "auto add zero enum value",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemType",
					Options: &tableaupb.WorksheetOptions{
						Name: "ItemType",
					},
					Fields: []*internalpb.Field{
						{Number: 1, Name: "ITEM_TYPE_FRUIT", Alias: "Fruit"},
						{Number: 2, Name: "ITEM_TYPE_EQUIP", Alias: "Equip"},
						{Number: 3, Name: "ITEM_TYPE_BOX", Alias: "Box"},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						ctx: context.Background(),
					},
				},
			},
			want: `enum ItemType {
  option (tableau.etype) = {name:"ItemType"};

  ITEM_TYPE_INVALID = 0;
  ITEM_TYPE_FRUIT = 1 [(tableau.evalue).name = "Fruit"]; // Fruit
  ITEM_TYPE_EQUIP = 2 [(tableau.evalue).name = "Equip"]; // Equip
  ITEM_TYPE_BOX = 3 [(tableau.evalue).name = "Box"]; // Box
}

`,
			wantErr: false,
		},
		{
			name: "zero enum value not the first one",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemType",
					Options: &tableaupb.WorksheetOptions{
						Name: "ItemType",
					},
					Fields: []*internalpb.Field{
						{Number: -1, Name: "ITEM_TYPE_FRUIT", Alias: "Fruit"},
						{Number: 0, Name: "ITEM_TYPE_EQUIP", Alias: "Equip"},
						{Number: 1, Name: "ITEM_TYPE_BOX", Alias: "Box"},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						ctx: context.Background(),
					},
				},
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.x.exportEnum()
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetExporter.exportEnum() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				assert.Equal(t, tt.want, tt.x.p.String())
			}
		})
	}
}

func Test_sheetExporter_exportStruct(t *testing.T) {
	tests := []struct {
		name    string
		x       *sheetExporter
		want    string
		wantErr bool
	}{
		{
			name: "export-struct",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "TaskReward",
					Options: &tableaupb.WorksheetOptions{
						Name: "StructTaskReward",
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
						{Name: "num", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Num"}},
						{Name: "fruit_type", Type: "FruitType", FullType: "protoconf.FruitType", Predefined: true, Options: &tableaupb.FieldOptions{Name: "FruitType"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{},
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message TaskReward {
  option (tableau.struct) = {name:"StructTaskReward"};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
  int32 num = 2 [(tableau.field) = {name:"Num"}];
  protoconf.FruitType fruit_type = 3 [(tableau.field) = {name:"FruitType"}];
}

`,
			wantErr: false,
		},
		{
			name: "field-number-compatibility-add-new-field-in-the-middle",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "Item", // use message unittest.Item to test
					Options: &tableaupb.WorksheetOptions{
						Name: "StructItem",
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
						{Name: "fruit_type", Type: "FruitType", FullType: "protoconf.FruitType", Predefined: true, Options: &tableaupb.FieldOptions{Name: "FruitType"}},
						{Name: "num", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Num"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					ProtoPackage: "unittest",
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						ProtoRegistryFilesWithGenerated: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message Item {
  option (tableau.struct) = {name:"StructItem"};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
  protoconf.FruitType fruit_type = 3 [(tableau.field) = {name:"FruitType"}];
  int32 num = 2 [(tableau.field) = {name:"Num"}];
}

`,
			wantErr: false,
		},
		{
			name: "field-number-compatibility-delete-old-field-and-add-new-field",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "Item", // use message unittest.Item to test
					Options: &tableaupb.WorksheetOptions{
						Name: "StructItem",
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
						{Name: "fruit_type", Type: "FruitType", FullType: "protoconf.FruitType", Predefined: true, Options: &tableaupb.FieldOptions{Name: "FruitType"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					ProtoPackage: "unittest",
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						ProtoRegistryFilesWithGenerated: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message Item {
  option (tableau.struct) = {name:"StructItem"};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
  protoconf.FruitType fruit_type = 3 [(tableau.field) = {name:"FruitType"}];
}

`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.exportStruct(); (err != nil) != tt.wantErr {
				t.Errorf("sheetExporter.exportStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, tt.x.p.String())
		})
	}
}

func Test_sheetExporter_exportUnion(t *testing.T) {
	tests := []struct {
		name    string
		x       *sheetExporter
		want    string
		wantErr bool
	}{
		{
			name: "export-union",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "TaskTarget",
					Options: &tableaupb.WorksheetOptions{
						Name: "UnionTaskTarget",
					},
					Fields: []*internalpb.Field{
						{Number: 1, Name: "PvpBattle", Alias: "SoloPVPBattle",
							Fields: []*internalpb.Field{
								{Number: 1, Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
								{Number: 2, Name: "damage", Type: "int64", FullType: "int64", Options: &tableaupb.FieldOptions{Name: "Damage"}},
								{Number: 3, Name: "type_list", Type: "repeated", FullType: "repeated protoconf.FruitType",
									ListEntry:  &internalpb.Field_ListEntry{ElemType: "FruitType", ElemFullType: "protoconf.FruitType"},
									Predefined: true,
									Options:    &tableaupb.FieldOptions{Name: "Type", Layout: tableaupb.Layout_LAYOUT_INCELL}},
							},
						},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						ctx:       context.Background(),
						OutputOpt: &options.ProtoOutputOption{},
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message TaskTarget {
  option (tableau.union) = {name:"UnionTaskTarget"};

  Type type = 9999 [(tableau.field) = {name:"Type"}];
  oneof value {
    option (tableau.oneof) = {field:"Field"};

    PvpBattle pvp_battle = 1; // Bound to enum value: TYPE_PVP_BATTLE.
  }

  enum Type {
    TYPE_INVALID = 0;
    TYPE_PVP_BATTLE = 1 [(tableau.evalue).name = "SoloPVPBattle"]; // SoloPVPBattle
  }

  message PvpBattle {
    uint32 id = 1 [(tableau.field) = {name:"ID"}];
    int64 damage = 2 [(tableau.field) = {name:"Damage"}];
    repeated protoconf.FruitType type_list = 3 [(tableau.field) = {name:"Type" layout:LAYOUT_INCELL}];
  }
}

`,
			wantErr: false,
		},
		{
			name: "export-union-preserve-field-numbers",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "Target", // use message unittest.Target to test
					Options: &tableaupb.WorksheetOptions{
						Name: "UnionTarget",
					},
					Fields: []*internalpb.Field{
						{Number: 1, Name: "Pvp", Alias: "PVP",
							Fields: []*internalpb.Field{
								// keep existing field "type"
								{Name: "type", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Type"}},
								// delete field "health" (field number 2)
								// add new field "armor"
								{Name: "armor", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "Armor"}},
								// keep existing field "damage"
								{Name: "damage", Type: "int64", FullType: "int64", Options: &tableaupb.FieldOptions{Name: "Damage"}},
							},
						},
						{Number: 2, Name: "Pve", Alias: "PVE",
							Fields: []*internalpb.Field{
								{Name: "mission", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Mission"}},
								{Name: "heros", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Heros"}},
								{Name: "dungeons", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Dungeons"}},
							},
						},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					ProtoPackage: "unittest",
					gen: &Generator{
						ctx: context.Background(),
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						ProtoRegistryFilesWithGenerated: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message Target {
  option (tableau.union) = {name:"UnionTarget"};

  Type type = 9999 [(tableau.field) = {name:"Type"}];
  oneof value {
    option (tableau.oneof) = {field:"Field"};

    Pvp pvp = 1; // Bound to enum value: TYPE_PVP.
    Pve pve = 2; // Bound to enum value: TYPE_PVE.
  }

  enum Type {
    TYPE_INVALID = 0;
    TYPE_PVP = 1 [(tableau.evalue).name = "PVP"]; // PVP
    TYPE_PVE = 2 [(tableau.evalue).name = "PVE"]; // PVE
  }

  message Pvp {
    int32 type = 1 [(tableau.field) = {name:"Type"}];
    uint32 armor = 5 [(tableau.field) = {name:"Armor"}];
    int64 damage = 3 [(tableau.field) = {name:"Damage"}];
  }
  message Pve {
    int32 mission = 1 [(tableau.field) = {name:"Mission"}];
    int32 heros = 2 [(tableau.field) = {name:"Heros"}];
    int32 dungeons = 3 [(tableau.field) = {name:"Dungeons"}];
  }
}

`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.exportUnion(); (err != nil) != tt.wantErr {
				t.Errorf("sheetExporter.exportUnion() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, tt.x.p.String())
		})
	}
}

func Test_sheetExporter_exportMessager(t *testing.T) {
	tests := []struct {
		name    string
		x       *sheetExporter
		want    string
		wantErr bool
	}{
		{
			name: "export-messager",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemConf",
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{},
					},
					messagerPatternRegexp: regexp.MustCompile(`Conf$`),
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message ItemConf {
  option (tableau.worksheet) = {};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
}

`,
			wantErr: false,
		},
		{
			name: "export-messager-pattern-not-match",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemConf",
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{},
					},
					messagerPatternRegexp: regexp.MustCompile(`Data$`),
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			wantErr: true,
		},
		{
			name: "export-messager-with-validate",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemConf",
					Options: &tableaupb.WorksheetOptions{
						Name:     "ItemConf",
						Validate: `cel:{id:"item.id" message:"id must be positive" expression:"this.id > 0"}`,
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{},
					},
					messagerPatternRegexp: regexp.MustCompile(`Conf$`),
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
				Imports:        make(map[string]bool),
			},
			want: `message ItemConf {
  option (tableau.worksheet) = {name:"ItemConf"};
  option (buf.validate.message) = {cel:{id:"item.id" message:"id must be positive" expression:"this.id > 0"}};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
}

`,
			wantErr: false,
		},
		{
			name: "export-messager-with-message-validate-on-nested-message",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ItemConf",
					Options: &tableaupb.WorksheetOptions{
						Name: "ItemConf",
					},
					Fields: []*internalpb.Field{
						{
							Name: "item_map", Type: "map<uint32, Item>", FullType: "map<uint32, Item>",
							MapEntry: &internalpb.Field_MapEntry{KeyType: "uint32", ValueType: "Item", ValueFullType: "Item"},
							Options: &tableaupb.FieldOptions{
								Key:    "ID",
								Layout: tableaupb.Layout_LAYOUT_VERTICAL,
								Prop: &tableaupb.FieldProp{
									ValidateMessage: `cel:{id:"item.id_name" message:"id must be positive when name is non-empty" expression:"this.id == 0u || this.name != ''"}`,
								},
							},
							Fields: []*internalpb.Field{
								{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
								{Name: "name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "Name"}},
							},
						},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{},
					},
					messagerPatternRegexp: regexp.MustCompile(`Conf$`),
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
				Imports:        make(map[string]bool),
			},
			want: `message ItemConf {
  option (tableau.worksheet) = {name:"ItemConf"};

  map<uint32, Item> item_map = 1 [(tableau.field) = {key:"ID" layout:LAYOUT_VERTICAL}];
  message Item {
    option (buf.validate.message) = {cel:{id:"item.id_name" message:"id must be positive when name is non-empty" expression:"this.id == 0u || this.name != ''"}};
    uint32 id = 1 [(tableau.field) = {name:"ID"}];
    string name = 2 [(tableau.field) = {name:"Name"}];
  }
}

`,
			wantErr: false,
		},
		{
			name: "field-number-compatibility-delete-fields-and-add-new-fields",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "YamlScalarConf",
					Options: &tableaupb.WorksheetOptions{
						Name: "YamlScalarConf",
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
						// delete field "num"
						// {Name: "num", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Num"}},
						{Name: "value", Type: "uint64", FullType: "uint64", Options: &tableaupb.FieldOptions{Name: "Value"}},
						{Name: "inserted_field", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "InsertedField"}},
						{Name: "weight", Type: "int64", FullType: "int64", Options: &tableaupb.FieldOptions{Name: "Weight"}},
						{Name: "percentage", Type: "float", FullType: "float", Options: &tableaupb.FieldOptions{Name: "Percentage"}},
						{Name: "ratio", Type: "double", FullType: "double", Options: &tableaupb.FieldOptions{Name: "Ratio"}},
						{Name: "another_inserted_field", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "AnotherInsertedField"}},
						{Name: "name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "Name"}},
						{Name: "blob", Type: "bytes", FullType: "bytes", Options: &tableaupb.FieldOptions{Name: "Blob"}},
						// delete field "ok" which has max field number
						// {Name: "ok", Type: "bool", FullType: "bool", Options: &tableaupb.FieldOptions{Name: "OK"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					ProtoPackage: "unittest",
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						ProtoRegistryFilesWithGenerated: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message YamlScalarConf {
  option (tableau.worksheet) = {name:"YamlScalarConf"};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
  uint64 value = 3 [(tableau.field) = {name:"Value"}];
  int32 inserted_field = 10 [(tableau.field) = {name:"InsertedField"}];
  int64 weight = 4 [(tableau.field) = {name:"Weight"}];
  float percentage = 5 [(tableau.field) = {name:"Percentage"}];
  double ratio = 6 [(tableau.field) = {name:"Ratio"}];
  int32 another_inserted_field = 11 [(tableau.field) = {name:"AnotherInsertedField"}];
  string name = 7 [(tableau.field) = {name:"Name"}];
  bytes blob = 8 [(tableau.field) = {name:"Blob"}];
}

`,
			wantErr: false,
		},
		{
			name: "field-number-compatibility-sub-message",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "ActivityConf",
					Options: &tableaupb.WorksheetOptions{
						Name: "ActivityConf",
					},
					Fields: []*internalpb.Field{
						{
							Name: "activity_map", Type: "map<uint32, Activity>", FullType: "map<uint32, Activity>",
							MapEntry: &internalpb.Field_MapEntry{KeyType: "uint32", ValueType: "Activity", ValueFullType: "Activity"},
							Options:  &tableaupb.FieldOptions{Key: "ActivityID", Layout: tableaupb.Layout_LAYOUT_VERTICAL},
							Fields: []*internalpb.Field{
								{Name: "activity_id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ActivityID"}},
								// delete field "activity_name"
								// {Name: "activity_name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "ActivityName"}},
								// add new field "activity_desc"
								{Name: "activity_desc", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "ActivityDesc"}},
								{
									Name: "chapter_map", Type: "map<uint32, Chapter>", FullType: "map<uint32, Chapter>",
									MapEntry: &internalpb.Field_MapEntry{KeyType: "uint32", ValueType: "Chapter", ValueFullType: "Chapter"},
									Options:  &tableaupb.FieldOptions{Key: "ChapterID", Layout: tableaupb.Layout_LAYOUT_VERTICAL},
									Fields: []*internalpb.Field{
										{Name: "chapter_id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ChapterID"}},
										{Name: "chapter_name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "ChapterName"}},
										{
											Name: "section_list", Type: "repeated", FullType: "repeated Section",
											ListEntry: &internalpb.Field_ListEntry{ElemType: "Section", ElemFullType: "Section"},
											Options:   &tableaupb.FieldOptions{Layout: tableaupb.Layout_LAYOUT_VERTICAL},
											Fields: []*internalpb.Field{
												{Name: "section_id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "SectionID"}},
												// delete field "section_name"
												// {Name: "section_name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "SectionName"}},
												{Name: "section_desc", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "SectionDesc"}},
												{
													Name: "reward_map", Type: "map<uint32, Reward>", FullType: "map<uint32, Reward>",
													MapEntry: &internalpb.Field_MapEntry{KeyType: "uint32", ValueType: "Reward", ValueFullType: "Reward"},
													Options:  &tableaupb.FieldOptions{Name: "Reward", Key: "ID", Layout: tableaupb.Layout_LAYOUT_HORIZONTAL},
													Fields: []*internalpb.Field{
														{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
														{Name: "num", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Num"}},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					ProtoPackage: "unittest",
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						ProtoRegistryFilesWithGenerated: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message ActivityConf {
  option (tableau.worksheet) = {name:"ActivityConf"};

  map<uint32, Activity> activity_map = 1 [(tableau.field) = {key:"ActivityID" layout:LAYOUT_VERTICAL}];
  message Activity {
    uint32 activity_id = 1 [(tableau.field) = {name:"ActivityID"}];
    string activity_desc = 4 [(tableau.field) = {name:"ActivityDesc"}];
    map<uint32, Chapter> chapter_map = 3 [(tableau.field) = {key:"ChapterID" layout:LAYOUT_VERTICAL}];
    message Chapter {
      uint32 chapter_id = 1 [(tableau.field) = {name:"ChapterID"}];
      string chapter_name = 2 [(tableau.field) = {name:"ChapterName"}];
      repeated Section section_list = 3 [(tableau.field) = {layout:LAYOUT_VERTICAL}];
      message Section {
        uint32 section_id = 1 [(tableau.field) = {name:"SectionID"}];
        string section_desc = 4 [(tableau.field) = {name:"SectionDesc"}];
        map<uint32, Reward> reward_map = 3 [(tableau.field) = {name:"Reward" key:"ID" layout:LAYOUT_HORIZONTAL}];
        message Reward {
          uint32 id = 1 [(tableau.field) = {name:"ID"}];
          int32 num = 2 [(tableau.field) = {name:"Num"}];
        }
      }
    }
  }
}

`,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.x.exportMessager(); (err != nil) != tt.wantErr {
				t.Errorf("sheetExporter.exportMessager() error = %v, wantErr %v", err, tt.wantErr)
			}
			assert.Equal(t, tt.want, tt.x.p.String())
		})
	}
}
