package protogen

import (
	"reflect"
	"testing"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
)

func Test_parseScalarOrEnumField(t *testing.T) {
	type args struct {
		typeInfos xproto.TypeInfoMap
		name      string
		typ       string
	}
	tests := []struct {
		name    string
		args    args
		want    *tableaupb.Field
		wantErr bool
	}{
		{
			name: "int32 ID",
			args: args{
				typeInfos: xproto.TypeInfoMap{},
				name:      "ID",
				typ:       "int32",
			},
			want: &tableaupb.Field{
				Type:     "int32",
				FullType: "int32",
				Name:     "id",
				Options: &tableaupb.FieldOptions{
					Name: "ID",
				},
			},
		},
		{
			name: "predefined enum type: ItemType",
			args: args{
				typeInfos: xproto.TypeInfoMap{
					"ItemType": &xproto.TypeInfo{
						Fullname:       "protoconf.ItemType",
						ParentFilename: "common.proto",
						Kind:           types.EnumKind,
					},
				},
				name: "Type",
				typ:  "enum<.ItemType>",
			},
			want: &tableaupb.Field{
				Type:       "ItemType",
				FullType:   "protoconf.ItemType",
				Name:       "type",
				Predefined: true,
				Options: &tableaupb.FieldOptions{
					Name: "Type",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseScalarOrEnumField(tt.args.typeInfos, tt.args.name, tt.args.typ)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseScalarOrEnumField() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseScalarOrEnumField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseTypeDescriptor(t *testing.T) {
	type args struct {
		typeInfos xproto.TypeInfoMap
		rawType   string
	}
	tests := []struct {
		name    string
		args    args
		want    *types.Descriptor
		wantErr bool
	}{
		{
			name: "scalar: int32",
			args: args{
				typeInfos: xproto.TypeInfoMap{},
				rawType:   "int32",
			},
			want: &types.Descriptor{
				Name:     "int32",
				FullName: "int32",
			},
		},
		{
			name: "message: Item",
			args: args{
				typeInfos: xproto.TypeInfoMap{},
				rawType:   "Item",
			},
			want: &types.Descriptor{
				Name:     "Item",
				FullName: "Item",
				Kind:     types.MessageKind,
			},
		},
		{
			name: "predefined enum: ItemType",
			args: args{
				typeInfos: xproto.TypeInfoMap{
					"ItemType": &xproto.TypeInfo{
						Fullname:       "protoconf.ItemType",
						ParentFilename: "common.proto",
						Kind:           types.EnumKind,
					},
				},
				rawType: "enum<.ItemType>",
			},
			want: &types.Descriptor{
				Name:       "ItemType",
				FullName:   "protoconf.ItemType",
				Predefined: true,
				Kind:       types.EnumKind,
			},
		},
		{
			name: "predefined message: Item",
			args: args{
				typeInfos: xproto.TypeInfoMap{
					"Item": &xproto.TypeInfo{
						Fullname:       "protoconf.Item",
						ParentFilename: "common.proto",
						Kind:           types.MessageKind,
					},
				},
				rawType: ".Item",
			},
			want: &types.Descriptor{
				Name:       "Item",
				FullName:   "protoconf.Item",
				Predefined: true,
				Kind:       types.MessageKind,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTypeDescriptor(tt.args.typeInfos, tt.args.rawType)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTypeDescriptor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("name: %s, parseTypeDescriptor() = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func Test_parseIncellStruct(t *testing.T) {
	type args struct {
		structType string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "one field",
			args: args{

				structType: "int32 ID",
			},
			want: []string{"int32", "ID"},
		},
		{
			name: "one field with space",
			args: args{

				structType: " int32 ID ",
			},
			want: []string{"int32", "ID"},
		},
		{
			name: "two fields with space",
			args: args{

				structType: "int32 ID, string Name",
			},
			want: []string{"int32", "ID", "string", "Name"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIncellStruct(tt.args.structType)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIncellStruct() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseIncellStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}
