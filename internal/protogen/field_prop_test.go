package protogen

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

func TestExtractListFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want *tableaupb.FieldProp
	}{
		// TODO: Add test cases.
		{
			name: "emptyListFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{},
			},
			want: nil,
		},
		{
			name: "noneEmptyListFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Length: 2,
				},
			},
			want: &tableaupb.FieldProp{
				Length: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractListFieldProp(tt.args.prop); !proto.Equal(got, tt.want) {
				t.Errorf("ExtractListFieldProp() = %v, want %v", got, tt.want)
			}
		})
	}
}
