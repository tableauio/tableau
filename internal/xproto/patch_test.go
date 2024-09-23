package xproto

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
)

func TestLoadWithPatch(t *testing.T) {
	type args struct {
		dst, src, result proto.Message
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Empty proto3 string field",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name:  "apple",
					Name2: "apple2",
				},
				src: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name2: "apple2",
				},
				result: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name2: "apple2",
				},
			},
		},
		{
			name: "Empty proto2 string field",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name:  "apple",
					Name3: proto.String("apple3"),
				},
				src: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name3: proto.String(""),
				},
				result: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name3: proto.String(""),
				},
			},
		},
		{
			name: "Merge list",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name:      "apple",
					PriceList: []int32{10, 100},
				},
				src: &unittestpb.PatchMergeConf{
					Name:      "orange",
					PriceList: []int32{20, 200},
				},
				result: &unittestpb.PatchMergeConf{
					Name:      "orange",
					PriceList: []int32{10, 100, 20, 200},
				},
			},
		},
		{
			name: "Replace list",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name:             "apple",
					ReplacePriceList: []int32{10, 100},
				},
				src: &unittestpb.PatchMergeConf{
					Name:             "orange",
					ReplacePriceList: []int32{20, 200},
				},
				result: &unittestpb.PatchMergeConf{
					Name:             "orange",
					ReplacePriceList: []int32{20, 200},
				},
			},
		},
		{
			name: "Merge map",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name: "apple",
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
				src: &unittestpb.PatchMergeConf{
					Name: "orange",
					ItemMap: map[uint32]*unittestpb.Item{
						1: {
							Id:  1,
							Num: 99,
						},
						999: {
							Id:  999,
							Num: 99900,
						},
					},
				},
				result: &unittestpb.PatchMergeConf{
					Name: "orange",
					ItemMap: map[uint32]*unittestpb.Item{
						1: {
							Id:  1,
							Num: 99,
						},
						2: {
							Id:  2,
							Num: 20,
						},
						999: {
							Id:  999,
							Num: 99900,
						},
					},
				},
			},
		},
		{
			name: "Replace map",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name: "apple",
					ReplaceItemMap: map[uint32]*unittestpb.Item{
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
				src: &unittestpb.PatchMergeConf{
					Name: "orange",
					ReplaceItemMap: map[uint32]*unittestpb.Item{
						1: {
							Id:  1,
							Num: 99,
						},
						999: {
							Id:  999,
							Num: 99900,
						},
					},
				},
				result: &unittestpb.PatchMergeConf{
					Name: "orange",
					ReplaceItemMap: map[uint32]*unittestpb.Item{
						1: {
							Id:  1,
							Num: 99,
						},
						999: {
							Id:  999,
							Num: 99900,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			PatchMessage(tt.args.dst, tt.args.src)
			require.Equal(t, proto.Equal(tt.args.result, tt.args.dst), true)
		})
	}
}
