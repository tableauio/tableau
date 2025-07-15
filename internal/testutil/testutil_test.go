package testutil

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
)

func TestAssertProtoJSONEq(t *testing.T) {
	type args struct {
		t        *testing.T
		expected proto.Message
		actual   proto.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "equal",
			args: args{
				t: t,
				expected: &unittestpb.ItemConf{
					ItemMap: map[uint32]*unittestpb.Item{
						1: {
							Id:  1,
							Num: 10,
						},
						2: {
							Id:  2,
							Num: 20,
						},
					},
				},
				actual: &unittestpb.ItemConf{
					ItemMap: map[uint32]*unittestpb.Item{
						2: {
							Id:  2,
							Num: 20,
						},
						1: {
							Id:  1,
							Num: 10,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AssertProtoJSONEq(tt.args.t, tt.args.expected, tt.args.actual)
			AssertProtoJSONEqf(t, tt.args.expected, tt.args.actual, "")
		})
	}
}
