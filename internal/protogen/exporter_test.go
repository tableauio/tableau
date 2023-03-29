package protogen

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := genFieldOptionsString(tt.args.opts); got != tt.want {
				t.Errorf("genFieldOptionsString() = %v, want %v", got, tt.want)
			}
		})
	}
}
