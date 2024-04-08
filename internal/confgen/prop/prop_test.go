package prop

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
)

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
					Unique: true,
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
