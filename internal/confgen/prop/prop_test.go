package prop

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
)

func TestCheckKeyUnique(t *testing.T) {
	type args struct {
		prop    *tableaupb.FieldProp
		key     string
		existed bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "unique",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
				},
				key:     "100",
				existed: false,
			},
			wantErr: false,
		},
		{
			name: "not unique",
			args: args{
				prop: &tableaupb.FieldProp{
					Unique: true,
				},
				key:     "100",
				existed: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckKeyUnique(tt.args.prop, tt.args.key, tt.args.existed); (err != nil) != tt.wantErr {
				t.Errorf("CheckKeyUnique() error = %v, wantErr %v", err, tt.wantErr)
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
