package xerrors

import (
	"fmt"
	"testing"
)

func TestErrorf(t *testing.T) {
	type args struct {
		code int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "with stack",
			args: args{
				code: -1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// err := Errorf(tt.args.code, "add some msg %d", 111)
			err := Errorf("add some msg %d", 111)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
		})
	}
}

func TestWrapf(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with stack",
			args: args{
				err: Errorf("some error %d", 111),
			},
		},
		{
			name: "fmt.Errorf",
			args: args{
				err: Wrapf(fmt.Errorf("fmt.Errorf"), "wrapf"),
			},
		},
		{
			name: "Errorf",
			args: args{
				err: Wrapf(Wrapf(Errorf("Errorf"), "Wrapf"), "wrapf"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrapf(tt.args.err, "add some msg %d", 111)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
		})
	}
}
