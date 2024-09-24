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
			name: "Empty string field",
			args: args{
				dst: &unittestpb.PatchMergeConf{
					Name:  "apple",
					Name2: "apple2",
				},
				src: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name2: "",
				},
				result: &unittestpb.PatchMergeConf{
					Name:  "orange",
					Name2: "apple2",
				},
			},
		},
		{
			name: "Empty optional string field",
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
		{
			name: "Recursively patch",
			args: args{
				dst: &unittestpb.RecursivePatchConf{
					ShopMap: map[uint32]*unittestpb.RecursivePatchConf_Shop{
						1: {
							ShopId: 1,
							GoodsMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods{
								1001: {
									GoodsId: 1001,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000001: {
											Type:      10000001,
											PriceList: []int32{1, 2, 3},
										},
										10000002: {
											Type:      10000002,
											PriceList: []int32{4, 5, 6},
										},
									},
									TagList: []string{
										"new",
										"discount",
									},
								},
								1002: {
									GoodsId: 1002,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000002: {
											Type:      10000002,
											PriceList: []int32{7, 8, 9},
										},
									},
									TagList: []string{
										"new",
									},
								},
							},
						},
					},
				},
				src: &unittestpb.RecursivePatchConf{
					ShopMap: map[uint32]*unittestpb.RecursivePatchConf_Shop{
						1: {
							ShopId: 1,
							GoodsMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods{
								1001: {
									GoodsId: 1001,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000002: {
											Type:      10000002,
											PriceList: []int32{31, 32, 33},
										},
										10000003: {
											Type:      10000003,
											PriceList: []int32{44, 45, 46},
										},
									},
									TagList: []string{
										"offshelf soon",
										"discount",
									},
								},
								1003: {
									GoodsId: 1003,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000002: {
											Type:      10000002,
											PriceList: []int32{37, 38, 39},
										},
									},
									TagList: []string{
										"new",
									},
								},
							},
						},
						2: {
							ShopId: 2,
							GoodsMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods{
								2001: {
									GoodsId: 2001,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										20000001: {
											Type:      20000001,
											PriceList: []int32{2001, 2002, 2003},
										},
									},
								},
							},
						},
					},
				},
				result: &unittestpb.RecursivePatchConf{
					ShopMap: map[uint32]*unittestpb.RecursivePatchConf_Shop{
						1: {
							ShopId: 1,
							GoodsMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods{
								1001: {
									GoodsId: 1001,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000001: {
											Type:      10000001,
											PriceList: []int32{1, 2, 3},
										},
										10000002: {
											Type:      10000002,
											PriceList: []int32{31, 32, 33},
										},
										10000003: {
											Type:      10000003,
											PriceList: []int32{44, 45, 46},
										},
									},
									TagList: []string{
										"offshelf soon",
										"discount",
									},
								},
								1002: {
									GoodsId: 1002,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000002: {
											Type:      10000002,
											PriceList: []int32{7, 8, 9},
										},
									},
									TagList: []string{
										"new",
									},
								},
								1003: {
									GoodsId: 1003,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										10000002: {
											Type:      10000002,
											PriceList: []int32{37, 38, 39},
										},
									},
									TagList: []string{
										"new",
									},
								},
							},
						},
						2: {
							ShopId: 2,
							GoodsMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods{
								2001: {
									GoodsId: 2001,
									CurrencyMap: map[uint32]*unittestpb.RecursivePatchConf_Shop_Goods_Currency{
										20000001: {
											Type:      20000001,
											PriceList: []int32{2001, 2002, 2003},
										},
									},
								},
							},
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
