package protogen

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/printer"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	_ "github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genFieldOptionsString(tt.args.opts); got != tt.want {
				t.Errorf("genFieldOptionsString() = %v, want %v", got, tt.want)
			}
		})
	}
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
		m protoreflect.ProtoMessage
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
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := marshalToText(tt.args.m); got != tt.want {
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
			name: "field-number-compatibility",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "Item",
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
					wb: &internalpb.Workbook{
						Name: "tableau/protobuf/unittest/common",
					},
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						GeneratedProtoRegistryFiles: protoregistry.GlobalFiles,
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
						ctx: context.Background(),
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
			name: "field-number-compatibility",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "YamlScalarConf",
					Options: &tableaupb.WorksheetOptions{
						Name: "YamlScalarConf",
					},
					Fields: []*internalpb.Field{
						{Name: "id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "ID"}},
						{Name: "num", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "Num"}},
						{Name: "value", Type: "uint64", FullType: "uint64", Options: &tableaupb.FieldOptions{Name: "Value"}},
						{Name: "inserted_field", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "InsertedField"}},
						{Name: "weight", Type: "int64", FullType: "int64", Options: &tableaupb.FieldOptions{Name: "Weight"}},
						{Name: "percentage", Type: "float", FullType: "float", Options: &tableaupb.FieldOptions{Name: "Percentage"}},
						{Name: "ratio", Type: "double", FullType: "double", Options: &tableaupb.FieldOptions{Name: "Ratio"}},
						{Name: "another_inserted_field", Type: "int32", FullType: "int32", Options: &tableaupb.FieldOptions{Name: "AnotherInsertedField"}},
						{Name: "name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "Name"}},
						{Name: "blob", Type: "bytes", FullType: "bytes", Options: &tableaupb.FieldOptions{Name: "Blob"}},
						{Name: "ok", Type: "bool", FullType: "bool", Options: &tableaupb.FieldOptions{Name: "OK"}},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					wb: &internalpb.Workbook{
						Name: "tableau/protobuf/unittest/unittest",
					},
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						GeneratedProtoRegistryFiles: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message YamlScalarConf {
  option (tableau.worksheet) = {name:"YamlScalarConf"};

  uint32 id = 1 [(tableau.field) = {name:"ID"}];
  int32 num = 2 [(tableau.field) = {name:"Num"}];
  uint64 value = 3 [(tableau.field) = {name:"Value"}];
  int32 inserted_field = 10 [(tableau.field) = {name:"InsertedField"}];
  int64 weight = 4 [(tableau.field) = {name:"Weight"}];
  float percentage = 5 [(tableau.field) = {name:"Percentage"}];
  double ratio = 6 [(tableau.field) = {name:"Ratio"}];
  int32 another_inserted_field = 11 [(tableau.field) = {name:"AnotherInsertedField"}];
  string name = 7 [(tableau.field) = {name:"Name"}];
  bytes blob = 8 [(tableau.field) = {name:"Blob"}];
  bool ok = 9 [(tableau.field) = {name:"OK"}];
}

`,
			wantErr: false,
		},
		{
			name: "field-number-compatibility-in-sub-structs",
			x: &sheetExporter{
				ws: &internalpb.Worksheet{
					Name: "RewardConf",
					Options: &tableaupb.WorksheetOptions{
						Name: "RewardConf",
					},
					Fields: []*internalpb.Field{
						{
							Name:     "reward_map",
							Type:     "map<uint32, Reward>",
							FullType: "map<uint32, Reward>",
							MapEntry: &internalpb.Field_MapEntry{
								KeyType:       "uint32",
								ValueType:     "Reward",
								ValueFullType: "Reward",
							},
							Options: &tableaupb.FieldOptions{
								Key:    "RewardID",
								Layout: tableaupb.Layout_LAYOUT_VERTICAL,
							},
							Fields: []*internalpb.Field{
								{Name: "reward_id", Type: "uint32", FullType: "uint32", Options: &tableaupb.FieldOptions{Name: "RewardID"}},
								{Name: "reward_name", Type: "string", FullType: "string", Options: &tableaupb.FieldOptions{Name: "RewardName"}},
								{
									Name: "item_map", Type: "map<uint32, Item>", FullType: "map<uint32, unittest.Item>", Predefined: true,
									MapEntry: &internalpb.Field_MapEntry{KeyType: "uint32", ValueType: "Item", ValueFullType: "unittest.Item"},
									Options:  &tableaupb.FieldOptions{Name: "Item", Key: "ID", Layout: tableaupb.Layout_LAYOUT_HORIZONTAL},
								},
							},
						},
					},
				},
				p: printer.New(),
				be: &bookExporter{
					wb: &internalpb.Workbook{
						Name: "tableau/protobuf/unittest/unittest",
					},
					gen: &Generator{
						OutputOpt: &options.ProtoOutputOption{
							PreserveFieldNumbers: true,
						},
						GeneratedProtoRegistryFiles: protoregistry.GlobalFiles,
					},
				},
				typeInfos:      &xproto.TypeInfos{},
				nestedMessages: make(map[string]*internalpb.Field),
			},
			want: `message RewardConf {
  option (tableau.worksheet) = {name:"RewardConf"};

  map<uint32, Reward> reward_map = 1 [(tableau.field) = {key:"RewardID" layout:LAYOUT_VERTICAL}];
  message Reward {
    uint32 reward_id = 1 [(tableau.field) = {name:"RewardID"}];
    string reward_name = 3 [(tableau.field) = {name:"RewardName"}];
    map<uint32, unittest.Item> item_map = 2 [(tableau.field) = {name:"Item" key:"ID" layout:LAYOUT_HORIZONTAL}];
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
