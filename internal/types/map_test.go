package types

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestCheckMessageWithOnlyKVFields(t *testing.T) {
	type args struct {
		msg protoreflect.Message
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Fruit",
			args: args{
				msg: (&tableaupb.TestIncellMap_Fruit{}).ProtoReflect(),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckMessageWithOnlyKVFields(tt.args.msg); got != tt.want {
				t.Errorf("CheckMessageWithOnlyKVFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_expectFieldOptName(t *testing.T) {
	msg := &tableaupb.TestIncellMap_Fruit{}
	type args struct {
		fd   protoreflect.FieldDescriptor
		name string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Key",
			args: args{
				fd:   msg.ProtoReflect().Descriptor().Fields().ByNumber(1),
				name: DefaultMapKeyOptName,
			},
			want: true,
		},
		{
			name: "Value",
			args: args{
				fd:   msg.ProtoReflect().Descriptor().Fields().ByNumber(2),
				name: DefaultMapValueOptName,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectFieldOptName(tt.args.fd, tt.args.name); got != tt.want {
				t.Errorf("expectFieldOptName() = %v, want %v", got, tt.want)
			}
		})
	}
}
