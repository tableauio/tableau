package fieldprop

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCheckOrder(t *testing.T) {
	type args struct {
		fd     protoreflect.FieldDescriptor
		oldVal protoreflect.Value
		newVal protoreflect.Value
		order  tableaupb.Order
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "asc int32 equal",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(0),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
		{
			name: "asc int32 less",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(-1),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: false,
		},
		{
			name: "asc int32 greater",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(1),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
		{
			name: "desc int32 equal",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(0),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: true,
		},
		{
			name: "desc int32 less",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(-1),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: true,
		},
		{
			name: "desc int32 greater",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(1),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: false,
		},
		{
			name: "strictly asc int32 equal",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(0),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: false,
		},
		{
			name: "strictly asc int32 less",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(-1),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: false,
		},
		{
			name: "strictly asc int32 greater",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(1),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: true,
		},
		{
			name: "strictly desc int32 equal",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(0),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			},
			want: false,
		},
		{
			name: "strictly desc int32 less",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(-1),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			},
			want: true,
		},
		{
			name: "strictly desc int32 greater",
			args: args{
				fd:     wrapperspb.Int32(0).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfInt32(0),
				newVal: protoreflect.ValueOfInt32(1),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			},
			want: false,
		},
		{
			name: "asc string",
			args: args{
				fd:     wrapperspb.String("").ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfString("Alice"),
				newVal: protoreflect.ValueOfString("Bob"),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
		{
			name: "asc time",
			args: args{
				fd: (&unittestpb.PatchMergeConf_Time{}).ProtoReflect().Descriptor().Fields().Get(0),
				oldVal: protoreflect.ValueOfMessage(func() protoreflect.Message {
					md := (&timestamppb.Timestamp{}).ProtoReflect().Descriptor()
					msg := dynamicpb.NewMessage(md)
					msg.Set(md.Fields().ByName("seconds"), protoreflect.ValueOfInt64(10000))
					return msg.ProtoReflect()
				}()),
				newVal: protoreflect.ValueOfMessage(func() protoreflect.Message {
					md := (&timestamppb.Timestamp{}).ProtoReflect().Descriptor()
					msg := dynamicpb.NewMessage(md)
					msg.Set(md.Fields().ByName("seconds"), protoreflect.ValueOfInt64(20000))
					return msg.ProtoReflect()
				}()),
				order: tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, got := CheckOrder(tt.args.fd, tt.args.oldVal, tt.args.newVal, tt.args.order); got != tt.want {
				t.Errorf("CheckOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}
