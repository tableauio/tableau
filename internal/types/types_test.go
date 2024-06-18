package types

import (
	"reflect"
	"testing"
)

func TestMatchMap(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *MapDescriptor
	}{
		{
			name: "normal-map",
			args: args{
				text: "map<int32, ValueType>",
			},
			want: &MapDescriptor{
				KeyType:   "int32",
				ValueType: "ValueType",
			},
		},
		{
			name: "normal-map-without-space",
			args: args{
				text: "map<int32,ValueType>",
			},
			want: &MapDescriptor{
				KeyType:   "int32",
				ValueType: "ValueType",
			},
		},
		{
			name: "enum-keyed-map",
			args: args{
				text: "map<enum<.EnumType>, ValueType>",
			},
			want: &MapDescriptor{
				KeyType:   "enum<.EnumType>",
				ValueType: "ValueType",
			},
		},
		{
			name: "map-with-prop",
			args: args{
				text: `map<int32, ValueType>|{range:"1,10"}`,
			},
			want: &MapDescriptor{
				KeyType:   "int32",
				ValueType: "ValueType",
				Prop:      PropDescriptor{Text: `range:"1,10"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchMap(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchMap() = %T, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchList(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *ListDescriptor
	}{
		{
			name: "scalar-list",
			args: args{
				text: "[]int32",
			},
			want: &ListDescriptor{
				ElemType:   "",
				ColumnType: "int32",
			},
		},
		{
			name: "scalar-list-with-prop",
			args: args{
				text: `[]int32|{range:"1,10"}`,
			},
			want: &ListDescriptor{
				ElemType:   "",
				ColumnType: "int32",
				Prop:       PropDescriptor{Text: `range:"1,10"`},
			},
		},
		{
			name: "enum-list-with-prop",
			args: args{
				text: `[]enum<.EnumType>|{range:"1,10"}`,
			},
			want: &ListDescriptor{
				ElemType:   "",
				ColumnType: "enum<.EnumType>",
				Prop:       PropDescriptor{Text: `range:"1,10"`},
			},
		},
		{
			name: "struct-list-without-column-type",
			args: args{
				text: "[ElemType]",
			},
			want: &ListDescriptor{
				ElemType:   "ElemType",
				ColumnType: "",
			},
		},
		{
			name: "struct-list",
			args: args{
				text: "[ElemType]int32",
			},
			want: &ListDescriptor{
				ElemType:   "ElemType",
				ColumnType: "int32",
			},
		},
		{
			name: "predefined-struct-list",
			args: args{
				text: "[.ElemType]int32",
			},
			want: &ListDescriptor{
				ElemType:   ".ElemType",
				ColumnType: "int32",
			},
		},
		{
			name: "keyed-struct-list",
			args: args{
				text: "[ElemType]<int32>",
			},
			want: &ListDescriptor{
				ElemType:   "ElemType",
				ColumnType: "<int32>",
			},
		},
		{
			name: "predefined-keyed-struct-list",
			args: args{
				text: "[.ElemType]<int32>",
			},
			want: &ListDescriptor{
				ElemType:   ".ElemType",
				ColumnType: "<int32>",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchList(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchKeyedList(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *KeyedListDescriptor
	}{
		{
			name: "keyed-struct-list-with-prop",
			args: args{
				text: `[ElemType]<int32>|{range:"1,10"}`,
			},
			want: &KeyedListDescriptor{
				ElemType:   "ElemType",
				ColumnType: "int32",
				Prop:       PropDescriptor{Text: `range:"1,10"`},
			},
		},
		{
			name: "normal-struct-list",
			args: args{
				text: "[Type]int32",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchKeyedList(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchKeyedList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchStruct(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *StructDescriptor
	}{
		{
			name: "new-defined-struct",
			args: args{
				text: "{int32 Id, int32 Value}Property",
			},
			want: &StructDescriptor{
				StructType: "int32 Id, int32 Value",
				ColumnType: "Property",
			},
		},
		{
			name: "predefined-cross-cell-struct",
			args: args{
				text: "{.Item}int32",
			},
			want: &StructDescriptor{
				StructType: ".Item",
				ColumnType: "int32",
			},
		},
		{
			name: "predefined-incell-struct",
			args: args{
				text: "{.Item}",
			},
			want: &StructDescriptor{
				StructType: ".Item",
				ColumnType: "",
			},
		},
		{
			name: "predefined-incell-struct-with-prop",
			args: args{
				text: `{.Item}|{range:"1,10"}`,
			},
			want: &StructDescriptor{
				StructType: ".Item",
				ColumnType: "",
				Prop:       PropDescriptor{Text: `range:"1,10"`},
			},
		},
		{
			name: "new-defined-cross-cell-struct-with-prop",
			args: args{
				text: `{Item}int32|{range:"1,10"}`,
			},
			want: &StructDescriptor{
				StructType: "Item",
				ColumnType: "int32",
				Prop:       PropDescriptor{Text: `range:"1,10"`},
			},
		},
		{
			name: "custom-named-struct-with-prop",
			args: args{
				text: `{Item(RewardItem)}int32|{range:"~,10"}`,
			},
			want: &StructDescriptor{
				StructType: "Item",
				CustomName: "RewardItem",
				ColumnType: "int32",
				Prop:       PropDescriptor{Text: `range:"~,10"`},
			},
		},
		{
			name: "custom-named-predefined-struct-with-prop",
			args: args{
				text: `{.Item(RewardItem)}int32|{range:"~,10"}`,
			},
			want: &StructDescriptor{
				StructType: ".Item",
				CustomName: "RewardItem",
				ColumnType: "int32",
				Prop:       PropDescriptor{Text: `range:"~,10"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchStruct(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchStruct() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchScalar(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *ScalarDescriptor
	}{
		{
			name: `scalar-type-with-prop`,
			args: args{
				text: `int32|{default:"1"}`,
			},
			want: &ScalarDescriptor{
				ScalarType: "int32",
				Prop:       PropDescriptor{Text: `default:"1"`},
			},
		},
		{
			name: `scalar-type-with-prop-and-space`,
			args: args{
				text: ` int32 |  {refer:"ItemConf.ItemId"}`,
			},
			want: &ScalarDescriptor{
				ScalarType: "int32",
				Prop:       PropDescriptor{Text: `refer:"ItemConf.ItemId"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchScalar(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchScalar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchEnum(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want *EnumDescriptor
	}{
		{
			name: `enum-type-with-prop`,
			args: args{
				text: `enum<.EnumType>|{default:"1"}`,
			},
			want: &EnumDescriptor{
				EnumType: ".EnumType",
				Prop:     PropDescriptor{Text: `default:"1"`},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchEnum(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchEnum() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchBoringInteger(t *testing.T) {
	type args struct {
		text string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "integer 0",
			args: args{
				text: "0",
			},
			want: nil,
		},
		{
			name: "integer 1",
			args: args{
				text: "1",
			},
			want: nil,
		},
		{
			name: "integer 01",
			args: args{
				text: "01",
			},
			want: nil,
		},
		{
			name: "boring integer 0.0",
			args: args{
				text: "0.0",
			},
			want: []string{"0.0", "0"},
		},
		{
			name: "boring integer 1.000",
			args: args{
				text: "1.000",
			},
			want: []string{"1.000", "1"},
		},
		{
			name: "scientific-notation integer 1.0000001e7",
			args: args{
				text: "1.0000001e7",
			},
			want: nil,
		},
		{
			name: "scientific-notation integer 1.0000001E-7",
			args: args{
				text: "1.0000001E-7",
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchBoringInteger(tt.args.text); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MatchBoringInteger() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBelongToFirstElement(t *testing.T) {
	type args struct {
		name   string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "belong to first element",
			args: args{
				name:   "BattleItem1Id",
				prefix: "BattleItem",
			},
			want: true,
		},
		{
			name: "not belong to first element as with no element fields",
			args: args{
				name:   "BattleItem1",
				prefix: "BattleItem",
			},
			want: false,
		},
		{
			name: "not belong to first element as with two next digits",
			args: args{
				name:   "BattleItem10",
				prefix: "BattleItem",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BelongToFirstElement(tt.args.name, tt.args.prefix); got != tt.want {
				t.Errorf("BelongToFirstElement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsScalarType(t *testing.T) {
	type args struct {
		t string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "int32",
			args: args{t: "int32"},
			want: true,
		},
		{
			name: WellKnownMessageTimestamp,
			args: args{t: WellKnownMessageTimestamp},
			want: true,
		},
		{
			name: WellKnownMessageDuration,
			args: args{t: WellKnownMessageDuration},
			want: true,
		},
		{
			name: "MessageType",
			args: args{t: "MessageType"},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsScalarType(tt.args.t); got != tt.want {
				t.Errorf("IsScalarType() = %v, want %v", got, tt.want)
			}
		})
	}
}
