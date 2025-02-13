package xproto

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestGetFieldTypeName(t *testing.T) {
	incellMap := &unittestpb.IncellMap{}
	incellList := &unittestpb.IncellList{}
	incellItem := &unittestpb.IncellMap_Item{}
	item := &unittestpb.Item{}
	pve := &unittestpb.Target_Pve{}
	type args struct {
		fd protoreflect.FieldDescriptor
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "map with value as message type",
			args: args{fd: incellMap.ProtoReflect().Descriptor().Fields().ByNumber(1)},
			want: "map<int32, unittest.IncellMap.Fruit>",
		},
		{
			name: "map with value as enum type",
			args: args{fd: incellMap.ProtoReflect().Descriptor().Fields().ByNumber(2)},
			want: "map<int64, unittest.FruitFlavor>",
		},
		{
			name: "map with value as scalar type",
			args: args{fd: pve.ProtoReflect().Descriptor().Fields().ByNumber(3)},
			want: "map<int32, int64>",
		},
		{
			name: "list with element as message type",
			args: args{fd: incellList.ProtoReflect().Descriptor().Fields().ByNumber(3)},
			want: "repeated unittest.Item",
		},
		{
			name: "list with element as enum type",
			args: args{fd: incellList.ProtoReflect().Descriptor().Fields().ByNumber(2)},
			want: "repeated unittest.FruitFlavor",
		},
		{
			name: "list with element as scalar type",
			args: args{fd: incellList.ProtoReflect().Descriptor().Fields().ByNumber(1)},
			want: "repeated int32",
		},
		{
			name: "message",
			args: args{fd: pve.ProtoReflect().Descriptor().Fields().ByNumber(1)},
			want: "unittest.Target.Pve.Mission",
		},
		{
			name: "enum",
			args: args{fd: incellItem.ProtoReflect().Descriptor().Fields().ByNumber(1)},
			want: "unittest.FruitType",
		},
		{
			name: "scalar",
			args: args{fd: item.ProtoReflect().Descriptor().Fields().ByNumber(1)},
			want: "uint32",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetFieldTypeName(tt.args.fd); got != tt.want {
				t.Errorf("GetFieldTypeName() = %v, want %v", got, tt.want)
			}
		})
	}
}
