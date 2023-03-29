package protogen

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

func TestIsEmptyFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "nilMapFieldProp",
			args: args{
				prop: nil,
			},
			want: false,
		},
		{
			name: "emptyMapFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{},
			},
			want: true,
		},
		{
			name: "noneEmptyMapFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					JsonName: "json_name",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsEmptyFieldProp(tt.args.prop); got != tt.want {
				t.Errorf("IsEmptyFieldProp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractMapFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want *tableaupb.FieldProp
	}{
		{
			name: "emptyMapFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{},
			},
			want: nil,
		},
		{
			name: "noneEmptyMapFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique:   true,
					Sequence: proto.Int64(1),
					Size:     2,
				},
			},
			want: &tableaupb.FieldProp{
				Unique:   true,
				Sequence: proto.Int64(1),
				Size:     2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractMapFieldProp(tt.args.prop); !proto.Equal(got, tt.want) {
				t.Errorf("ExtractMapFieldProp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractListFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want *tableaupb.FieldProp
	}{
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
					Size: 2,
				},
			},
			want: &tableaupb.FieldProp{
				Size: 2,
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

func TestExtractStructFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want *tableaupb.FieldProp
	}{
		{
			name: "emptyStructFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
				},
			},
			want: nil,
		},
		{
			name: "noneEmptyStructFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
					Form:   tableaupb.Form_FORM_JSON,
				},
			},
			want: &tableaupb.FieldProp{
				Form: tableaupb.Form_FORM_JSON,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractStructFieldProp(tt.args.prop); !proto.Equal(got, tt.want) {
				t.Errorf("ExtractStructFieldProp() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractScalarFieldProp(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want *tableaupb.FieldProp
	}{
		{
			name: "emptyScalarFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
				},
			},
			want: nil,
		},
		{
			name: "noneEmptyStructFieldProp",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
					Range:  "1~10",
				},
			},
			want: &tableaupb.FieldProp{
				Range: "1~10",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractScalarFieldProp(tt.args.prop); !proto.Equal(got, tt.want) {
				t.Errorf("ExtractScalarFieldProp() = %v, want %v", got, tt.want)
			}
		})
	}
}
