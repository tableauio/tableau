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

// New returns an error with the supplied message.
// New also records the stack trace at the point it was called.
func New(msg string) error {
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: msg,
		fields:  map[string]any{KeyReason: msg},
	}
}

// Newf formats according to a format specifier and returns the string
// as a value that satisfies error.
// Newf also records the code and stack trace at the point it was called.
func Newf(format string, args ...any) error {
	msg := fmt.Sprintf(format, args...)
	return &withMessage{
		cause:   &base{stack: callers(1)},
		message: msg,
		fields:  map[string]any{KeyReason: msg},
	}
}

// NewKV returns an error with the supplied message and structured key-value fields.
// NewKV also records the stack trace at the point it was called.
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

// Wrap annotates err with a stack trace at the point Wrap was called.
// If err is nil, Wrap returns nil.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause: withStack(1, err),
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

// WrapKV wraps err with structured key-value metadata fields.
// The fields are accessible via NewDesc but do NOT appear in err.Error().
// WrapKV also records the stack trace at the point it was called.
func WrapKV(err error, keysAndValues ...any) error {
	if err == nil {
		return nil
	}
	return &withMessage{
		cause:  withStack(1, err),
		fields: parseKV(keysAndValues...),
	}
}

// fieldsCarrier is implemented by error types that carry structured key-value fields.
// NewDesc uses this interface to extract fields without string parsing.
type fieldsCarrier interface {
	Fields() map[string]any
}

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
	format(b, s, verb)
}

// renderWithFields implements fieldsRenderer by delegating to the cause,
// allowing outer WrapKV fields to pass through the base stack wrapper.
func (b *base) renderWithFields(outerFields map[string]any) string {
	if b.cause != nil {
		if r, ok := b.cause.(fieldsRenderer); ok {
			return r.renderWithFields(outerFields)
		}
		return b.cause.Error()
	}
	return ""
}

// withMessage is an error that has a cause error, a human-readable message,
// and optional structured key-value fields.
//
// When replacesCause is true, message fully replaces the cause's text in
// Error() (i.e. the cause is kept only for errors.Is/As traversal and stack
// retrieval, not for display). When false (the default), Error() returns
// "message: cause.Error()" — the standard wrapping behaviour.
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

// fieldsRenderer is implemented by error types that can render themselves
// with additional outer fields merged in (inner fields always win).
type fieldsRenderer interface {
	renderWithFields(outerFields map[string]any) string
}

func (w *withMessage) Error() string {
	if w.message != "" {
		// When replacesCause is set, message is the complete error text.
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
	// No message but has fields: pass them down to the cause for rendering.
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

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withMessage) Unwrap() error { return w.cause }

// renderWithFields implements fieldsRenderer for the no-message case:
// merges outerFields with w.fields (inner wins) and passes them further down.
func (w *withMessage) renderWithFields(outerFields map[string]any) string {
	if w.message != "" {
		// Has a message: not a transparent wrapper, just return Error().
		return w.Error()
	}
	// Merge: start with outerFields, then overlay w.fields (inner wins).
	merged := make(map[string]any, len(outerFields)+len(w.fields))
	for k, v := range outerFields {
		merged[k] = v
	}
	for k, v := range w.fields {
		merged[k] = v
	}
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
			// %+v: same as ErrString(true) — summary + debugging fields + stack trace.
			if d := NewDesc(self); d != nil {
				_, _ = io.WriteString(s, d.ErrString(true))
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

// parseKV converts a variadic keysAndValues slice into a map[string]any.
// Panics if the number of arguments is odd.
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

// joinError is a multi-error that renders each child via NewDesc for structured
// output and implements fmt.Formatter so that %+v appends a stack trace.
type joinError struct {
	errs  []error
	stack *stack
}

func (j *joinError) Unwrap() []error { return j.errs }

// Error renders each child as a plain join (no structured fields at this level).
func (j *joinError) Error() string {
	return j.renderWithFields(nil)
}

// renderWithFields implements fieldsRenderer: renders the joined children with
// outerFields merged in (inner fields always win). This is called by an
// enclosing WrapKV (withMessage with no message but with fields) so that
// Module, BookName, SheetName etc. are propagated into each child's output.
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
			// %+v: same as ErrString(true) — summary + debugging fields + stack trace.
			if d := NewDesc(j); d != nil {
				_, _ = io.WriteString(s, d.ErrString(true))
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
	for k, v := range kv {
		fields[k] = v
	}
	fields[KeyReason] = detail.Text
	fields[keyErrCode] = ec.code
	fields[keyErrDesc] = detail.Desc
	fields[keyHelp] = detail.Help
	return &withMessage{
		cause:         withStack(2, ec),
		message:       detail.Text,
		fields:        fields,
		replacesCause: true, // message fully replaces ecode.Error(); keep cause only for errors.Is/stack
	}
}
