// Error handling model:
//  1. cause (nil = no cause) is wrapped by base, which holds the caller stack.
//  2. each error chain has exactly one caller stack.
//  3. withMessage carries a message and can be nested arbitrarily.
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
	"maps"
	"strings"

	"github.com/tableauio/tableau/internal/localizer"
)

// New returns a new error with msg and a stack trace.
func New(msg string) error {
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: msg,
		fields:  map[string]any{KeyReason: msg},
	}
}

// Newf returns a formatted error with a stack trace.
func Newf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: msg,
		fields:  map[string]any{KeyReason: msg},
	}
}

// NewKV returns an error with msg, structured key-value fields, and a stack trace.
func NewKV(msg string, keysAndValues ...any) error {
	fields := parseKV(keysAndValues...)
	if fields == nil {
		fields = make(map[string]any)
	}
	fields[KeyReason] = msg
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: msg,
		fields:  fields,
	}
}

// Wrap annotates err with a stack trace. Returns nil if err is nil.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause: withStack(1, err),
	}
}

// Wrapf annotates err with a formatted message and a stack trace. Returns nil if err is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:   withStack(1, err),
		message: fmt.Sprintf(format, args...),
	}
}

// WrapKV wraps err with structured key-value fields (visible via NewDesc, not in Error()) and a stack trace.
// Returns nil if err is nil.
func WrapKV(err error, keysAndValues ...any) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:  withStack(1, err),
		fields: parseKV(keysAndValues...),
	}
}

// fieldsCarrier is implemented by errors that carry structured key-value fields.
type fieldsCarrier interface {
	Fields() map[string]any
}

// base wraps a cause error and holds the caller stack.
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
	format(b, s, verb)
}

// renderWithFields delegates to the cause, passing outerFields through the stack wrapper.
func (b *base) renderWithFields(outerFields map[string]any) string {
	if b.cause != nil {
		if r, ok := b.cause.(fieldsRenderer); ok {
			return r.renderWithFields(outerFields)
		}
		return b.cause.Error()
	}
	return ""
}

// withMessage wraps a cause with an optional message and structured fields.
// If replacesCause is true, message fully replaces the cause text in Error();
// the cause is kept only for errors.Is/As and stack retrieval.
// Otherwise Error() returns "message: cause.Error()".
type withMessage struct {
	cause         error
	message       string
	fields        map[string]any // structured key-value metadata; never encoded into Error()
	replacesCause bool
}

// Fields implements fieldsCarrier.
func (w *withMessage) Fields() map[string]any {
	return w.fields
}

// fieldsRenderer renders an error string with outer fields merged in (inner fields win).
type fieldsRenderer interface {
	renderWithFields(outerFields map[string]any) string
}

func (w *withMessage) Error() string {
	if w.message != "" {
		// replacesCause: message is the complete text.
		if w.replacesCause {
			return w.message
		}
		if w.cause != nil {
			causeStr := w.cause.Error()
			if causeStr != "" {
				return w.message + ": " + causeStr
			}
		}
		return w.message
	}
	// No message but has fields: propagate to cause.
	if len(w.fields) > 0 && w.cause != nil {
		if r, ok := w.cause.(fieldsRenderer); ok {
			return r.renderWithFields(w.fields)
		}
	}
	// No message: delegate to cause.
	if w.cause != nil {
		return w.cause.Error()
	}
	return ""
}

// Unwrap returns the cause for error chain traversal.
func (w *withMessage) Unwrap() error { return w.cause }

// renderWithFields merges outerFields with w.fields (inner wins) and propagates down.
// If w has a message it is not a transparent wrapper, so Error() is returned directly.
func (w *withMessage) renderWithFields(outerFields map[string]any) string {
	if w.message != "" {
		return w.Error()
	}
	// Merge outerFields then overlay w.fields (inner wins).
	merged := make(map[string]any, len(outerFields)+len(w.fields))
	maps.Copy(merged, outerFields)
	maps.Copy(merged, w.fields)
	if w.cause != nil {
		if r, ok := w.cause.(fieldsRenderer); ok {
			return r.renderWithFields(merged)
		}
		return w.cause.Error()
	}
	return ""
}

func (w *withMessage) Format(s fmt.State, verb rune) {
	format(w, s, verb)
}

func format(self error, s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// %+v: Stringify(true) — summary + fields + stack trace.
			if d := NewDesc(self); d != nil {
				_, _ = io.WriteString(s, d.Stringify(true))
			} else {
				_, _ = io.WriteString(s, self.Error())
				var berr *base
				if errors.As(self, &berr) && berr.stack != nil {
					_, _ = fmt.Fprintf(s, "%+v", berr.stack)
				}
			}
		} else {
			_, _ = io.WriteString(s, self.Error())
		}
	case 's':
		_, _ = io.WriteString(s, self.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", self.Error())
	}
}

// withStack attaches a caller stack to err. Skips if a stack is already present.
// skip == 0 means the caller of withStack is the first frame shown.
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

// parseKV converts a variadic key-value slice into a map. Panics on odd length.
func parseKV(keysAndValues ...any) map[string]any {
	if len(keysAndValues) == 0 {
		return nil
	}
	if len(keysAndValues)%2 != 0 {
		panic("invalid Key-Value pairs: odd number")
	}
	m := make(map[string]any, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprint(keysAndValues[i])
		m[key] = keysAndValues[i+1]
	}
	return m
}

// joinError is a multi-error that renders children via NewDesc and supports %+v.
type joinError struct {
	errs  []error
	stack *stack
}

func (j *joinError) Unwrap() []error { return j.errs }

// Error renders all children via renderWithFields(nil).
func (j *joinError) Error() string {
	return j.renderWithFields(nil)
}

// renderWithFields renders children with outerFields merged in (inner fields win),
// propagating fields such as Module, BookName, SheetName from an enclosing WrapKV.
func (j *joinError) renderWithFields(outerFields map[string]any) string {
	if d := newDescWithOuter(j, outerFields); d != nil {
		return d.String()
	}
	// Fallback: plain join.
	var sb strings.Builder
	for i, e := range j.errs {
		if i > 0 {
			sb.WriteByte('\n')
		}
		if e != nil {
			sb.WriteString(e.Error())
		}
	}
	return sb.String()
}

func (j *joinError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			// %+v: Stringify(true) — summary + fields + stack trace.
			if d := NewDesc(j); d != nil {
				_, _ = io.WriteString(s, d.Stringify(true))
			} else {
				_, _ = io.WriteString(s, j.Error())
			}
		} else {
			_, _ = io.WriteString(s, j.Error())
		}
	case 's':
		_, _ = io.WriteString(s, j.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", j.Error())
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

func renderEcode(ec *ecode, kv map[string]any) error {
	detail := localizer.Default.RenderEcode(ec.code, kv)
	fields := make(map[string]any, len(kv)+4)
	maps.Copy(fields, kv)
	fields[KeyReason] = detail.Text
	fields[keyErrCode] = ec.code
	fields[keyErrDesc] = detail.Desc
	fields[keyHelp] = detail.Help
	return &withMessage{
		cause:         withStack(2, ec),
		message:       detail.Text,
		fields:        fields,
		replacesCause: true, // message replaces ecode.Error(); cause kept for errors.Is/stack
	}
}
