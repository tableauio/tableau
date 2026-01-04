//   Error handling model:
// 			1. cause error(nil means no cause) is wrapped by base error with caller stack
//          2. all errors contain only one caller stack
//          3. withMessage is an error with a message, which could be infinitely nested with each other
//
//                                 +---------+
//                                 |  cause  |
//                                 +----+----+
//                                      ^
//                                      |
//                                 +----------+
//                                 |   base   |
//                                 |  (stack) |
//                                 +----+-----+
//                                      ^
//                                      |
//                 +----------------------------------------+
//                 |                                        |
//          +------+------+                          +------+------+
//          | withMessage |                          | withMessage |
//          +------+------+                          +------+------+
//                 |                                        |
//          +------+------+                          +------+------+
//          | withMessage |                          | withMessage |
//          +------+------+                          +------+------+
//                 |                                        |
//          +------+------+                          +------+------+
//          | withMessage |                          | withMessage |
//          +------+------+                          +------+------+
//                 |                                        |

package xerrors

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tableauio/tableau/internal/localizer"
)

const sep = "|" // separator for error messages and key-value pairs

// base is an error which has a cause error and caller stack
type base struct {
	cause error
	*stack
}

func (b *base) Unwrap() error {
	return b.cause
}

func (b *base) Error() string {
	if b.cause == nil {
		return ""
	}
	return b.cause.Error()
}

func (b *base) Format(s fmt.State, verb rune) {
	format(b, b, s, verb)
}

// withMessage is an error that has a cause error and message.
type withMessage struct {
	cause   error
	message string
}

func (w *withMessage) Error() string {
	content := w.message
	if w.cause != nil {
		cause := w.cause.Error()
		if cause != "" && !strings.HasPrefix(cause, sep) {
			content += sep
		}
		content += cause
	}
	return content
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withMessage) Unwrap() error { return w.cause }

func (w *withMessage) Format(s fmt.State, verb rune) {
	format(w, w.cause, s, verb)
}

func format(self, cause error, s fmt.State, verb rune) {
	content := self.Error()
	switch verb {
	case 'v':
		_, _ = io.WriteString(s, content)
		// try to find base error in cause, then print the stack trace.
		var berr *base
		if errors.As(cause, &berr) {
			if berr.stack != nil {
				berr.stack.Format(s, verb)
			}
		}
	case 's':
		_, _ = io.WriteString(s, content)
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", content)
	}
}

// withStack add a caller stack to given error, but directly return if stack
// already wrapped.
//
// NOTE: skip == 0 means the caller of withStack is the first frame shown.
func withStack(skip int, err error) error { // nolint:unparam
	if err == nil {
		return nil
	}
	var berr *base
	if errors.As(err, &berr) {
		if berr.stack == nil {
			berr.stack = callers(1 + skip)
		}
		return err
	}
	return &base{cause: err, stack: callers(1 + skip)}
}

func combineKV(keysAndValues ...any) string {
	var msg strings.Builder
	for i := 0; i < len(keysAndValues); i += 2 {
		if i == len(keysAndValues)-1 {
			panic("invalid Key-Value pairs: odd number")
		}
		key, val := keysAndValues[i], keysAndValues[i+1]
		msg.WriteString(fmt.Sprintf("|%v: %v", key, val))
	}
	return msg.String()
}

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(msg string) error {
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: combineKV(KeyReason, msg),
	}
}

// Newf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Newf also records the code and stack trace at the point it was called.
func Newf(format string, args ...any) error {
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: combineKV(KeyReason, fmt.Sprintf(format, args...)),
	}
}

// NewKV returns an error with the supplied message and the key-value pairs
// as `[|key: value]...` string.
// NewKV also records the stack trace at the point it was called.
func NewKV(msg string, keysAndValues ...any) error {
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: combineKV(keysAndValues...) + combineKV(KeyReason, msg),
	}
}

// Wrap annotates err with a stack trace at the point Wrap was called.
// If err is nil, Wrap returns nil.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:   withStack(1, err),
		message: "",
	}
}

// Wrapf returns an error annotating err with a stack trace
// at the point Wrapf is called, and the format specifier.
// If err is nil, Wrapf returns nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:   withStack(1, err),
		message: fmt.Sprintf(format, args...),
	}
}

// WrapKV formats the key-value pairs as `[|key: value]...` string and
// returns the string as a value that satisfies error.
// WrapKV also records the stack trace at the point it was called.
func WrapKV(err error, keysAndValues ...any) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:   withStack(1, err),
		message: combineKV(keysAndValues...),
	}
}

type ecode struct {
	code string
	desc string
}

func newEcode(code, desc string) *ecode {
	return &ecode{
		code: code,
		desc: desc,
	}
}

func (e *ecode) Error() string {
	if e == nil || e.code == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.code, e.desc)
}

func (e *ecode) Is(target error) bool {
	t, ok := target.(*ecode)
	return ok && e.code == t.code
}

func renderSummary(module string, kv map[string]any) string {
	return localizer.Default.RenderMessage(module, kv)
}

func renderEcode(ecode *ecode, kv map[string]any) error {
	detail := localizer.Default.RenderEcode(ecode.code, kv)
	err := withStack(2, ecode)
	return &withMessage{
		cause: err,
		message: combineKV(
			KeyReason, detail.Text,
			keyErrCode, ecode.code,
			keyErrDesc, detail.Desc,
			keyHelp, detail.Help,
		),
	}
}
