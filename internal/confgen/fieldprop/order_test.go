package fieldprop

import (
	"testing"
	"time"

	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestCheckOrder(t *testing.T) {
	timestampFD := (&unittestpb.PatchMergeConf_Time{}).ProtoReflect().Descriptor().Fields().Get(0)
	newTimestamp := func(seconds int64) protoreflect.Value {
		md := (&timestamppb.Timestamp{}).ProtoReflect().Descriptor()
		msg := dynamicpb.NewMessage(md)
		msg.Set(md.Fields().ByName("seconds"), protoreflect.ValueOfInt64(seconds))
		return protoreflect.ValueOfMessage(msg.ProtoReflect())
	}
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
			name: "asc timestamp: equal",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
		{
			name: "asc timestamp: greater",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(20000),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: true,
		},
		{
			name: "asc timestamp: less",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(20000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_ASC,
			},
			want: false,
		},
		{
			name: "strictly asc timestamp: equal",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: false,
		},
		{
			name: "strictly asc timestamp: greater",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(20000),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: true,
		},
		{
			name: "strictly asc timestamp: less",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(20000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			},
			want: false,
		},
		{
			name: "desc timestamp: equal",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: true,
		},
		{
			name: "desc timestamp: greater",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(20000),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: false,
		},
		{
			name: "desc timestamp: less",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(20000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_DESC,
			},
			want: true,
		},
		{
			name: "strictly desc timestamp: equal",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			},
			want: false,
		},
		{
			name: "strictly desc timestamp: greater",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(10000),
				newVal: newTimestamp(20000),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			},
			want: false,
		},
		{
			name: "strictly desc timestamp: less",
			args: args{
				fd:     timestampFD,
				oldVal: newTimestamp(20000),
				newVal: newTimestamp(10000),
				order:  tableaupb.Order_ORDER_STRICTLY_DESC,
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

func Test_isTimeOrdered(t *testing.T) {
	tests := []struct {
		name   string
		oldVal time.Time
		newVal time.Time
		order  tableaupb.Order
		want   bool
	}{
		{
			name:   "asc: equal",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_ASC,
			want:   true,
		},
		{
			name:   "asc: greater",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			order:  tableaupb.Order_ORDER_ASC,
			want:   true,
		},
		{
			name:   "asc: less",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_ASC,
			want:   false,
		},
		{
			name:   "strictly asc: equal",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			want:   false,
		},
		{
			name:   "strictly asc: greater",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			order:  tableaupb.Order_ORDER_STRICTLY_ASC,
			want:   true,
		},
		{
			name:   "strictly asc: less",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_ASC,
			want:   false,
		},
		{
			name:   "desc: equal",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_DESC,
			want:   true,
		},
		{
			name:   "desc: greater",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			order:  tableaupb.Order_ORDER_DESC,
			want:   false,
		},
		{
			name:   "desc: less",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_DESC,
			want:   true,
		},
		{
			name:   "strictly desc: equal",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			want:   false,
		},
		{
			name:   "strictly desc: greater",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			order:  tableaupb.Order_ORDER_STRICTLY_DESC,
			want:   false,
		},
		{
			name:   "strictly desc: less",
			oldVal: time.Date(2020, 1, 1, 0, 0, 0, 1, time.Local),
			newVal: time.Date(2020, 1, 1, 0, 0, 0, 0, time.Local),
			order:  tableaupb.Order_ORDER_DESC,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTimeOrdered(tt.oldVal, tt.newVal, tt.order)
			if tt.want != got {
				t.Errorf("isTimeOrdered() = %v, want %v", got, tt.want)
			}
		})
	}
}
