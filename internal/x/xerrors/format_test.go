package xerrors

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatNew(t *testing.T) {
	tests := []struct {
		error
		format string
		want   string
	}{{
		New("error"),
		"%s",
		"error",
	}, {
		New("error"),
		"%v",
		"error",
	}, {
		// %+v with no KeyModule: same as Error(), no structured rendering.
		New("error"),
		"%+v",
		"error",
	}, {
		New("error"),
		"%q",
		`"error"`,
	}}
	for i, tt := range tests {
		testFormatRegexp(t, i, tt.error, tt.format, tt.want)
	}
}

func TestFormatNewf(t *testing.T) {
	tests := []struct {
		error
		format string
		want   string
	}{{
		Newf("newf: %s", "error"),
		"%s",
		"newf: error",
	}, {
		Newf("newf: %s", "error"),
		"%v",
		"newf: error",
	}, {
		// %+v with no KeyModule: same as Error(), no structured rendering.
		Newf("newf: %s", "error"),
		"%+v",
		"newf: error",
	}}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.error, tt.format, tt.want)
	}
}

func TestFormatWrap(t *testing.T) {
	tests := []struct {
		error
		format string
		want   string
	}{
		{
			Wrapf(New("error"), "error2"),
			"%s",
			"error2: error",
		},
		{
			Wrapf(New("error"), "error2"),
			"%v",
			"error2: error",
		},
		{
			// %+v uses KeyReason from the inner New("error"), not the outer message.
			Wrap(New("error")),
			"%+v",
			"error",
		},
		{
			Wrapf(io.EOF, "error1"),
			"%s",
			"error1: EOF",
		},
		{
			Wrap(io.EOF),
			"%v",
			"EOF",
		},
		{
			// %+v with no KeyReason/KeyModule: same as Error().
			Wrapf(io.EOF, "error1"),
			"%+v",
			"error1: EOF",
		},
		{
			Wrap(Wrap(io.EOF)),
			"%+v",
			"EOF",
		},
		{
			Wrapf(New("error"), "context"),
			"%q",
			`"context: error"`,
		},
	}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.error, tt.format, tt.want)
	}
}

func TestFormatWrapf(t *testing.T) {
	tests := []struct {
		error
		format string
		want   string
	}{
		{
			Wrapf(io.EOF, "error%d", 2),
			"%s",
			"error2: EOF",
		},
		{
			Wrapf(io.EOF, "error%d", 2),
			"%v",
			"error2: EOF",
		},
		{
			// %+v with no KeyReason/KeyModule: same as Error().
			Wrapf(io.EOF, "error%d", 2),
			"%+v",
			"error2: EOF",
		},
		{
			Wrapf(New("error"), "error%d", 2),
			"%s",
			"error2: error",
		},
		{
			Wrapf(New("error"), "error%d", 2),
			"%v",
			"error2: error",
		},
		{
			// %+v uses KeyReason from the inner New("error"), not the outer message.
			// This is the key difference: %s/%v = "error2: error", %+v = "error".
			Wrapf(New("error"), "error%d", 2),
			"%+v",
			"error",
		},
	}

	for i, tt := range tests {
		testFormatRegexp(t, i, tt.error, tt.format, tt.want)
	}
}

func wrappedNew(msg string) error { // This function will be mid-stack inlined in go 1.12+
	return New(msg)
}

func TestFormatWrappedNew(t *testing.T) {
	tests := []struct {
		error
		format string
		want   []string
	}{{
		// %+v with no KeyModule: desc = Error() text, followed by stack trace.
		wrappedNew("error"),
		"%+v",
		[]string{
			"error",
			"github\\.com/tableauio/tableau/internal/x/xerrors\\.wrappedNew\n",
		},
	}, {
		// %+v with ecode: renders full structured desc (Stringify(true)) = summary + debugging fields + stack trace.
		Wrap(E2003("1", 3)),
		"%+v",
		[]string{
			"error[E2003]: illegal sequence number",
			`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
			`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
			"",
			"--- debugging ---",
			"Module: default",
			"ErrCode: E2003",
			"ErrDesc: illegal sequence number",
			`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
			`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
			"",
			"github\\.com/tableauio/tableau/internal/x/xerrors\\.TestFormatWrappedNew\n",
			"",
		},
	}}
	for i, tt := range tests {
		testFormatCompleteCompare(t, i, tt.error, tt.format, tt.want, true)
	}
}

func TestFormatWrapKV(t *testing.T) {
	tests := []struct {
		error
		format string
		want   []string
	}{
		// %s / %v: always the plain Error() text.
		{
			WrapKV(io.EOF, "k1", "v1", "k2", "v2"),
			"%s",
			[]string{"EOF"},
		},
		{
			WrapKV(io.EOF, "k1", "v1", "k2", "v2"),
			"%v",
			[]string{"EOF"},
		},
		{
			WrapKV(New("error"), "k1", "v1", "k2", "v2"),
			"%s",
			[]string{"error"},
		},
		{
			WrapKV(New("error"), "k1", "v1", "k2", "v2"),
			"%v",
			[]string{"error"},
		},
		// %+v with a plain error (no KeyModule): desc = Error() text, followed by stack trace.
		{
			WrapKV(io.EOF, "k1", "v1", "k2", "v2"),
			"%+v",
			[]string{
				"EOF",
				"github\\.com/tableauio/tableau/internal/x/xerrors\\.TestFormatWrapKV\n",
			},
		},
		// %+v with ecode error: renders full structured desc (ErrCode + Reason + Help),
		// which is different from Error() that only returns the reason text.
		{
			E2003("1", 3),
			"%s",
			[]string{`value "1" does not meet sequence requirement: "sequence:3"`},
		},
		{
			E2003("1", 3),
			"%v",
			[]string{`value "1" does not meet sequence requirement: "sequence:3"`},
		},
		{
			// %+v renders Stringify(true): summary + debugging fields + stack trace.
			E2003("1", 3),
			"%+v",
			[]string{
				"error[E2003]: illegal sequence number",
				`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
				`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
				"",
				"--- debugging ---",
				"Module: default",
				"ErrCode: E2003",
				"ErrDesc: illegal sequence number",
				`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
				`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
				"",
				"github\\.com/tableauio/tableau/internal/x/xerrors\\.TestFormatWrapKV\n",
				"",
			},
		},
		{
			// WrapKV adds outer fields (BookName, SheetName) to an ecode error.
			// %+v renders Stringify(true): summary + debugging fields (including outer BookName/SheetName) + stack trace.
			WrapKV(E2003("1", 3), KeyModule, ModuleDefault, KeyBookName, "Test.xlsx", KeySheetName, "Sheet1"),
			"%+v",
			[]string{
				"error[E2003]: illegal sequence number",
				`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
				`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
				"",
				"--- debugging ---",
				"Module: default",
				"BookName: Test.xlsx",
				"SheetName: Sheet1",
				"ErrCode: E2003",
				"ErrDesc: illegal sequence number",
				`Reason: value "1" does not meet sequence requirement: "sequence:3"`,
				`Help: prop "sequence:3" requires value starts from "3" and increases monotonically`,
				"",
				"github\\.com/tableauio/tableau/internal/x/xerrors\\.TestFormatWrapKV\n",
				"",
			},
		},
	}

	for i, tt := range tests {
		testFormatCompleteCompare(t, i, tt.error, tt.format, tt.want, true)
	}
}

func testFormatRegexp(t *testing.T, n int, arg any, format, want string) {
	t.Helper()
	got := fmt.Sprintf(format, arg)
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")

	require.GreaterOrEqualf(t, len(gotLines), len(wantLines),
		"test %d: wantLines(%d) > gotLines(%d):\n got: %q\nwant: %q", n+1, len(wantLines), len(gotLines), got, want)

	for i, w := range wantLines {
		match, err := regexp.MatchString(w, gotLines[i])
		require.NoErrorf(t, err, "test %d: line %d: invalid regexp %q", n+1, i+1, w)
		require.Truef(t, match,
			"test %d: line %d: fmt.Sprintf(%q, err):\n got: %q\nwant: %q", n+1, i+1, format, got, want)
	}
}

var stackLineR = regexp.MustCompile(`\.`)

// parseBlocks parses input into a slice, where:
//   - incase entry contains a newline, its a stacktrace
//   - incase entry contains no newline, its a solo line.
//
// Detecting stack boundaries only works incase the WithStack-calls are
// to be found on the same line, thats why it is optionally here.
//
// Example use:
//
//	for _, e := range blocks {
//	  if strings.ContainsAny(e, "\n") {
//	    // Match as stack
//	  } else {
//	    // Match as line
//	  }
//	}
func parseBlocks(input string, detectStackboundaries bool) ([]string, error) {
	var blocks []string

	stack := ""
	wasStack := false
	lines := map[string]bool{} // already found lines

	for _, l := range strings.Split(input, "\n") {
		isStackLine := stackLineR.MatchString(l)

		switch {
		case !isStackLine && wasStack:
			blocks = append(blocks, stack, l)
			stack = ""
			lines = map[string]bool{}
		case isStackLine:
			if wasStack {
				// Detecting two stacks after another, possible cause lines match in
				// our tests due to WithStack(WithStack(io.EOF)) on same line.
				if detectStackboundaries {
					if lines[l] {
						if len(stack) == 0 {
							return nil, errors.New("len of block must not be zero here")
						}

						blocks = append(blocks, stack)
						stack = l
						lines = map[string]bool{l: true}
						continue
					}
				}

				stack = stack + "\n" + l
			} else {
				stack = l
			}
			lines[l] = true
		case !isStackLine && !wasStack:
			blocks = append(blocks, l)
		default:
			return nil, errors.New("must not happen")
		}

		wasStack = isStackLine
	}

	// Use up stack
	if stack != "" {
		blocks = append(blocks, stack)
	}
	return blocks, nil
}

func testFormatCompleteCompare(t *testing.T, n int, arg any, format string, want []string, detectStackBoundaries bool) {
	t.Helper()
	gotStr := fmt.Sprintf(format, arg)

	got, err := parseBlocks(gotStr, detectStackBoundaries)
	require.NoError(t, err)
	require.Lenf(t, got, len(want),
		"test %d: fmt.Sprintf(%s, err) -> wrong number of blocks:\n got: %s\nwant: %s\ngotStr: %q",
		n+1, format, prettyBlocks(got), prettyBlocks(want), gotStr)

	for i := range got {
		if strings.ContainsAny(want[i], "\n") {
			// Match as stack
			match, err := regexp.MatchString(want[i], got[i])
			require.NoErrorf(t, err, "test %d: block %d: invalid regexp %q", n+1, i+1, want[i])
			require.Truef(t, match,
				"test %d: block %d: fmt.Sprintf(%q, err):\ngot:\n%q\nwant:\n%q\nall-got:\n%s\nall-want:\n%s\n",
				n+1, i+1, format, got[i], want[i], prettyBlocks(got), prettyBlocks(want))
		} else {
			// Match as exact string.
			require.Equalf(t, want[i], got[i],
				"test %d: fmt.Sprintf(%s, err) at block %d", n+1, format, i+1)
		}
	}
}

func prettyBlocks(blocks []string) string {
	var out []string

	for _, b := range blocks {
		out = append(out, fmt.Sprintf("%v", b))
	}

	return "   " + strings.Join(out, "\n   ")
}
