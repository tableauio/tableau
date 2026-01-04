package xerrors

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
)

var initpc = caller()

type X struct{}

// val returns a Frame pointing to itself.
func (x X) val() Frame {
	return caller()
}

// ptr returns a Frame pointing to itself.
func (x *X) ptr() Frame {
	return caller()
}

func TestFrameFormat(t *testing.T) {
	var tests = []struct {
		Frame
		format string
		want   string
	}{{
		initpc,
		"%s",
		"@xerrors/stack_test.go",
	}, {
		initpc,
		"%+s",
		"github.com/tableauio/tableau/xerrors.init\n" +
			"\t.+/xerrors/stack_test.go",
	}, {
		0,
		"%s",
		"unknown",
	}, {
		0,
		"%+s",
		"unknown",
	}, {
		initpc,
		"%d",
		"10",
	}, {
		0,
		"%d",
		"0",
	}, {
		initpc,
		"%n",
		"init",
	}, {
		func() Frame {
			var x X
			return x.ptr()
		}(),
		"%n",
		`\(\*X\).ptr`,
	}, {
		func() Frame {
			var x X
			return x.val()
		}(),
		"%n",
		"X.val",
	}, {
		0,
		"%n",
		"",
	}, {
		initpc,
		"%v",
		"stack_test.go:10",
	}, {
		initpc,
		"%+v",
		"github.com/tableauio/tableau/xerrors.init\n" +
			"\t.+/xerrors/stack_test.go:10",
	}, {
		0,
		"%v",
		"unknown:0",
	}}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.Frame, tt.format, tt.want)
	}
}

func TestFuncname(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"", ""},
		{"runtime.main", "main"},
		{"github.com/tableauio/tableau/xerrors.funcname", "funcname"},
		{"funcname", "funcname"},
		{"io.copyBuffer", "copyBuffer"},
		{"main.(*R).Write", "(*R).Write"},
	}

	for _, tt := range tests {
		got := funcname(tt.name)
		want := tt.want
		if got != want {
			t.Errorf("funcname(%q): want: %q, got %q", tt.name, want, got)
		}
	}
}

func TestStackTrace(t *testing.T) {
	tests := []struct {
		err  error
		want []string
	}{
		{
			New("error"), []string{
				"github.com/tableauio/tableau/xerrors.TestStackTrace\n" +
					"\t.+/xerrors/stack_test.go:123",
			},
		},
		{
			Wrapf(New("error"), "ahh"), []string{
				"github.com/tableauio/tableau/xerrors.TestStackTrace\n" +
					"\t.+/xerrors/stack_test.go:129", // this is the stack of Wrap, not New
			},
		},
		{
			errors.Unwrap(Wrapf(New("error"), "ahh")), []string{
				"github.com/tableauio/tableau/xerrors.TestStackTrace\n" +
					"\t.+/xerrors/stack_test.go:135", // this is the stack of New
			},
		},
		{
			func() error { return New("error") }(), []string{
				`github.com/tableauio/tableau/xerrors.TestStackTrace.func1` +
					"\n\t.+/xerrors/stack_test.go:141", // this is the stack of New
				"github.com/tableauio/tableau/xerrors.TestStackTrace\n" +
					"\t.+/xerrors/stack_test.go:141", // this is the stack of New's caller
			},
		},
		{
			errors.Unwrap(func() error {
				return func() error {
					return Wrap(Newf("newf: hello %s", fmt.Sprintf("world: %s", "ooh")))
				}()
			}()), []string{
				`github.com/tableauio/tableau/xerrors.TestStackTrace.TestStackTrace.func2.func3` +
					"\n\t.+/xerrors/stack_test.go:151", // this is the stack of Newf
				`github.com/tableauio/tableau/xerrors.TestStackTrace.func2` +
					"\n\t.+/xerrors/stack_test.go:152", // this is the stack of Newf's caller
				"github.com/tableauio/tableau/xerrors.TestStackTrace\n" +
					"\t.+/xerrors/stack_test.go:153", // this is the stack of Newf's caller's caller
			},
		},
	}
	for i, tt := range tests {
		var base *base
		if !errors.As(tt.err, &base) {
			t.Errorf("expected %+v to match the base error", tt.err)
			continue
		}
		st := base.StackTrace()
		for j, want := range tt.want {
			testFormatRegexp(t, i, st[j], "%+v", want)
		}
	}
}

func stackTrace() StackTrace {
	const depth = 8
	var pcs [depth]uintptr
	n := runtime.Callers(1, pcs[:])
	var st stack = pcs[0:n]
	return st.StackTrace()
}

func TestStackTraceFormat(t *testing.T) {
	tests := []struct {
		StackTrace
		format string
		want   string
	}{
		{
			nil,
			"%s",
			`\[\]`,
		},
		{
			nil,
			"%v",
			`\[\]`,
		},
		{
			nil,
			"%+v",
			"",
		},
		{
			make(StackTrace, 0),
			"%s",
			`\[\]`,
		},
		{
			make(StackTrace, 0),
			"%v",
			`\[\]`,
		},
		{
			make(StackTrace, 0),
			"%+v",
			"",
		},
		{
			stackTrace()[:2],
			"%s",
			`\[@xerrors/stack_test.go @xerrors/stack_test.go\]`,
		},
		{
			stackTrace()[:2],
			"%#v",
			`\[@xerrors/stack_test.go:179 @xerrors/stack_test.go:226\]`,
		},
		{
			stackTrace()[:2],
			"%+v",
			"\n" +
				"github.com/tableauio/tableau/xerrors.stackTrace\n" +
				"\t.+/xerrors/stack_test.go:179\n" +
				"github.com/tableauio/tableau/xerrors.TestStackTraceFormat\n" +
				"\t.+/xerrors/stack_test.go:231",
		},
		{
			stackTrace()[:2],
			"%#v",
			`[@xerrors/stack_test.go:179, @xerrors/stack_test.go:240]`,
		},
	}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.StackTrace, tt.format, tt.want)
	}
}

// a version of runtime.Caller that returns a Frame, not a uintptr.
func caller() Frame {
	var pcs [3]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	frame, _ := frames.Next()
	return Frame(frame.PC)
}

func Test_withStack(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "without stack in cause",
			args: args{
				err: func() error {
					err := errors.New("error")
					return withStack(0, err)
				}(),
			},
			want: []string{
				"github.com/tableauio/tableau/xerrors.Test_withStack\n" +
					"\t.+/xerrors/stack_test.go:274",
			},
		},
		{
			name: "with stack in cause",
			args: args{
				err: func() error {
					err := New("error")
					return withStack(0, err)
				}(),
			},
			want: []string{
				"github.com/tableauio/tableau/xerrors.Test_withStack\n" +
					"\t.+/xerrors/stack_test.go:286", // this is the stack of new
				`github.com/tableauio/tableau/xerrors.Test_withStack` +
					"\n\t.+/xerrors/stack_test.go:288", // this is the stack of withStack
			},
		},
		{
			name: "nil error",
			args: args{
				err: withStack(0, nil),
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.want == nil {
				if tt.args.err != nil {
					t.Errorf("expected nil, got %+v", tt.args.err)
				}
				return
			}
			var base *base
			if !errors.As(tt.args.err, &base) {
				t.Errorf("expected %+v to match the base error", tt.args.err)
			}
			st := base.StackTrace()
			for j, want := range tt.want {
				testFormatRegexp(t, 0, st[j], "%+v", want)
			}
		})
	}
}
