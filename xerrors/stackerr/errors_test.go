package stackerr

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/pkg/errors"
)

func TestNew(t *testing.T) {
	type args struct {
		code      int
		withStack bool
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "with stack",
			args: args{
				code:      -1,
				withStack: true,
			},
		},
		{
			name: "without stack",
			args: args{
				code:      -1,
				withStack: false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.args.code)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
		})
	}
}

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
			err := Errorf(tt.args.code, "add some msg %d", 111)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
		})
	}
}

func TestWithStack(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with stack in cause",
			args: args{
				err: New(-1),
			},
		},
		{
			name: "without stack in cause",
			args: args{
				err: New(-1),
			},
		},
		{
			name: "with duplicated stack",
			args: args{
				err: errors.New("stack already in error"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := withStack(tt.args.err)
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
				err: Errorf(-1, "some error %d", 111),
			},
		},
		{
			name: "without stack",
			args: args{
				err: New(-1),
			},
		},
		{
			name: "fmt.Errorf",
			args: args{
				err: Wrapf(fmt.Errorf("fmt.Errorf"), "wrapf"),
			},
		},
		{
			name: "errors.Errorf",
			args: args{
				err: Wrapf(errors.Wrapf(errors.Errorf("errors.Errorf"), "errors.Wrapf"), "wrapf"),
			},
		},
		{
			name: "with code",
			args: args{
				err: WithCode(Wrapf(Errorf(-1, "test code"), "wrap1"), -2),
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

func TestCode(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want int
	}{
		{
			name: "test code 1",
			args: args{
				err: New(-1),
			},
			want: -1,
		},
		{
			name: "test code 2",
			args: args{
				err: Wrapf(WithCode(Wrapf(New(-1), "wrap1"), -2), "wrap2"),
			},
			want: -2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Code(tt.args.err); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Code() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIs(t *testing.T) {
	type args struct {
		err  error
		code int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "test 1",
			args: args{
				err:  Wrapf(WithCode(New(-2), -1), "wrapf"),
				code: -1,
			},
			want: true,
		},
		{
			name: "test 2",
			args: args{
				err:  New(-1),
				code: -2,
			},
			want: false,
		},
		{
			name: "test 3",
			args: args{
				err:  errors.Wrap(fmt.Errorf("test 3 error"), "add some message"),
				code: -2,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Is(tt.args.err, tt.args.code); got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWithCode(t *testing.T) {
	type args struct {
		err  error
		code int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no code",
			args: args{
				err:  errors.New("no code"),
				code: -1,
			},
		},
		{
			name: "one code",
			args: args{
				err:  New(-2),
				code: -1,
			},
		},
		{
			name: "two code",
			args: args{
				err: WithCode(
					WithCode(
						Errorf(-2, "base failed"),
						-3),
					-3),
				code: -1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrapf(WithCode(tt.args.err, tt.args.code), "test failed")
			t.Logf("%+v", err)
			t.Logf("%s", err)
		})
	}
}

func TestWithCodef(t *testing.T) {
	type args struct {
		err  error
		code int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "no code",
			args: args{
				err:  errors.New("no code"),
				code: -1,
			},
		},
		{
			name: "one code",
			args: args{
				err:  New(-2),
				code: -1,
			},
		},
		{
			name: "two code",
			args: args{
				err: WithCode(
					WithCode(
						Errorf(-2, "base failed"),
						-3),
					-3),
				code: -1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WithCodef(tt.args.err, tt.args.code, "test failed")
			t.Logf("%+v", err)
			t.Logf("%s", err)
		})
	}
}

func TestCause(t *testing.T) {
	wantErr := fmt.Errorf("test cause")
	type args struct {
		err error
	}
	tests := []struct {
		name  string
		args  args
		equal bool
	}{
		{
			name: "not equal",
			args: args{
				err: WithCode(New(-2), -3),
			},
			equal: false,
		},
		{
			name: "equal",
			args: args{
				err: WithCode(wantErr, -3),
			},
			equal: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Cause(tt.args.err)
			if (errors.Is(err, wantErr)) != tt.equal {
				t.Errorf("error = %+v, wantErr %+v", err, wantErr)
			}
		})
	}
}
