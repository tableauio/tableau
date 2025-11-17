package fieldprop

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestHasUnique(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "has-unique1",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: proto.Bool(false),
				},
			},
			want: true,
		},
		{
			name: "has-unique2",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: proto.Bool(true),
				},
			},
			want: true,
		},
		{
			name: "has-unique2",
			args: args{
				prop: &tableaupb.FieldProp{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasUnique(tt.args.prop); got != tt.want {
				t.Errorf("HasUnique() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequireUnique(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "require unique",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: proto.Bool(true),
				},
			},
			want: true,
		},
		{
			name: "not require unique",
			args: args{
				prop: nil,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequireUnique(tt.args.prop); got != tt.want {
				t.Errorf("RequireUnique() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSize(t *testing.T) {
	type args struct {
		prop         *tableaupb.FieldProp
		detectedSize int
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "use specified size",
			args: args{
				prop: &tableaupb.FieldProp{
					Size: 3,
				},
				detectedSize: 5,
			},
			want: 3,
		},
		{
			name: "use detected size",
			args: args{
				prop: &tableaupb.FieldProp{
					Fixed: true,
				},
				detectedSize: 5,
			},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSize(tt.args.prop, tt.args.detectedSize); got != tt.want {
				t.Errorf("GetSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsFixed(t *testing.T) {
	type args struct {
		prop *tableaupb.FieldProp
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "set fixed prop as true",
			args: args{
				prop: &tableaupb.FieldProp{
					Fixed: true,
				},
			},
			want: true,
		},
		{
			name: "set size prop as 3",
			args: args{
				prop: &tableaupb.FieldProp{
					Size: 3,
				},
			},
			want: true,
		},
		{
			name: "not fixed",
			args: args{
				prop: &tableaupb.FieldProp{},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsFixed(tt.args.prop); got != tt.want {
				t.Errorf("IsFixed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPresence(t *testing.T) {
	type args struct {
		prop    *tableaupb.FieldProp
		present bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "without-prop",
			args: args{
				present: false,
			},
			wantErr: false,
		},
		{
			name: "without-prop-present-set",
			args: args{
				prop:    &tableaupb.FieldProp{},
				present: true,
			},
			wantErr: false,
		},
		{
			name: "with-prop-present-set-as-true",
			args: args{
				prop: &tableaupb.FieldProp{
					Present: true,
				},
				present: false,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckPresence(tt.args.prop, tt.args.present); (err != nil) != tt.wantErr {
				t.Errorf("CheckPresence() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckInRange(t *testing.T) {
	type args struct {
		prop      *tableaupb.FieldProp
		fieldKind protoreflect.Kind
		value     protoreflect.Value
		present   bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "without-prop",
			args: args{
				present: false,
			},
			wantErr: false,
		},
		{
			name: "without-prop-present-set",
			args: args{
				prop:    &tableaupb.FieldProp{},
				present: true,
			},
			wantErr: false,
		},
		{
			name: "in-range",
			args: args{
				prop: &tableaupb.FieldProp{
					Range: "1,~",
				},
				fieldKind: protoreflect.Int32Kind,
				value:     protoreflect.ValueOfInt32(1),
				present:   true,
			},
			wantErr: false,
		},
		{
			name: "out-of-range",
			args: args{
				prop: &tableaupb.FieldProp{
					Range: "1,2",
				},
				fieldKind: protoreflect.Int32Kind,
				value:     protoreflect.ValueOfInt32(3),
				present:   true,
			},
			wantErr: true,
		},
		{
			name: "unrecognized-range-pattern",
			args: args{
				prop: &tableaupb.FieldProp{
					Range: "1,2,3",
				},
				fieldKind: protoreflect.Int32Kind,
				value:     protoreflect.ValueOfInt32(3),
				present:   true,
			},
			wantErr: true,
		},
		{
			name: "unsupported-kind",
			args: args{
				prop: &tableaupb.FieldProp{
					Range: "a,z",
				},
				fieldKind: protoreflect.StringKind,
				value:     protoreflect.ValueOfString("b"),
				present:   true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckInRange(tt.args.prop, tt.args.fieldKind, tt.args.value, tt.args.present); (err != nil) != tt.wantErr {
				t.Errorf("CheckInRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
