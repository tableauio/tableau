package protogen

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protoreflect"
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
						Unique: true,
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
						Unique:   true,
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
		left  *tableaupb.Field
		right *tableaupb.Field
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
			want: false,
		},
		{
			name: "one-is-nil",
			args: args{
				left:  &tableaupb.Field{},
				right: nil,
			},
			want: false,
		},
		{
			name: "one-sub-fields-nil",
			args: args{
				left: &tableaupb.Field{
					Fields: nil,
				},
				right: &tableaupb.Field{
					Fields: []*tableaupb.Field{
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
				left: &tableaupb.Field{
					Fields: []*tableaupb.Field{
						{
							Number: 1,
						},
						{
							Number: 2,
						},
					},
				},
				right: &tableaupb.Field{
					Fields: []*tableaupb.Field{
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
				left: &tableaupb.Field{
					Type: "Item",
				},
				right: &tableaupb.Field{
					Type: "Drop",
				},
			},
			want: false,
		},
		{
			name: "same-sub-fields",
			args: args{
				left: &tableaupb.Field{
					Fields: []*tableaupb.Field{
						{
							Number: 1,
							Name:   "Item",
							Alias:  "RewardItem",
						},
					},
				},
				right: &tableaupb.Field{
					Fields: []*tableaupb.Field{
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
						Unique:  true,
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
				wb: &tableaupb.Workbook{
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
