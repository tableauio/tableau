package xerrors

import (
	"fmt"
	"io"
	"runtime"
	"strconv"
	"strings"
)

const unknown = "unknown"

// Frame represents a program counter inside a stack frame.
// For historical reasons if Frame is interpreted as a uintptr
// its value represents the program counter + 1.
type Frame uintptr

// pc returns the program counter for this frame;
// multiple frames may have the same PC value.
func (f Frame) pc() uintptr { return uintptr(f) - 1 }

// file returns the full path to the file that contains the
// function for this Frame's pc.
func (f Frame) file() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return unknown
	}
	file, _ := fn.FileLine(f.pc())
	return file
}

// trimmedFile returns a package/file description of the caller,
// preserving only the leaf directory name and file name.
//
// Refer to https://github.com/uber-go/zap/blob/v1.27.1/zapcore/entry.go#L100
func (f Frame) trimmedFile() string {
	// nb. To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	//
	// Find the last separator.
	//
	file := f.file()
	idx := strings.LastIndexByte(file, '/')
	if idx == -1 {
		return file
	}
	// Find the penultimate separator.
	idx = strings.LastIndexByte(file[:idx], '/')
	if idx == -1 {
		return file
	}
	// Keep everything after the penultimate separator,
	// and prepend it with '@'.
	return "@" + file[idx+1:]
}

// line returns the line number of source code of the
// function for this Frame's pc.
func (f Frame) line() int {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return 0
	}
	_, line := fn.FileLine(f.pc())
	return line
}

// name returns the name of this function, if known.
func (f Frame) name() string {
	fn := runtime.FuncForPC(f.pc())
	if fn == nil {
		return unknown
	}
	return fn.Name()
}

// Format formats the frame according to the fmt.Formatter interface.
//
//	%s    source file
//	%d    source line
//	%n    function name
//	%v    equivalent to %s:%d
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%#s   function name and path of source file relative to the compile time
//	      GOPATH separated by \n\t (<funcname>\n\t<path>)
//	%#v   equivalent to %#s:%d
func (f Frame) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		switch {
		case s.Flag('#'):
			_, _ = io.WriteString(s, f.name())
			_, _ = io.WriteString(s, "\n\t")
			_, _ = io.WriteString(s, f.file())
		default:
			_, _ = io.WriteString(s, f.trimmedFile())
		}
	case 'd':
		_, _ = io.WriteString(s, strconv.Itoa(f.line()))
	case 'n':
		_, _ = io.WriteString(s, funcname(f.name()))
	case 'v':
		f.Format(s, 's')
		_, _ = io.WriteString(s, ":")
		f.Format(s, 'd')
	}
}

// StackTrace is stack of Frames from innermost (newest) to outermost (oldest).
type StackTrace []Frame

// Format formats the stack of Frames according to the fmt.Formatter interface.
//
//	%s	lists source files for each Frame in the stack
//	%v	lists the source file and line number for each Frame in the stack
//
// Format accepts flags that alter the printing of some verbs, as follows:
//
//	%#v   Prints filename, function, and line number for each Frame in the stack.
func (st StackTrace) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case s.Flag('#'):
			for _, f := range st {
				_, _ = io.WriteString(s, "\n")
				f.Format(s, verb)
			}
		default:
			st.formatSlice(s, verb)
		}
	case 's':
		st.formatSlice(s, verb)
	}
}

// formatSlice will format this StackTrace into the given buffer as a slice of
// Frame, only valid when called with '%s' or '%v'.
func (st StackTrace) formatSlice(s fmt.State, verb rune) {
	_, _ = io.WriteString(s, "[")
	for i, f := range st {
		if i > 0 {
			_, _ = io.WriteString(s, " ")
		}
		f.Format(s, verb)
	}
	_, _ = io.WriteString(s, "]")
}

// stack represents a stack of program counters.
type stack []uintptr

func (s *stack) Format(st fmt.State, verb rune) {
	switch verb {
	case 'v':
		switch {
		case st.Flag('#'):
			for _, pc := range *s {
				f := Frame(pc)
				_, _ = fmt.Fprintf(st, "\n%#v", f)
			}
		case st.Flag('+'):
			for i, pc := range *s {
				if i >= 3 { // truncation after 3 frames
					break
				}
				f := Frame(pc)
				_, _ = fmt.Fprintf(st, " %+v", f)
			}
		default:
		}
	default:
	}
}

func (s *stack) StackTrace() StackTrace {
	f := make([]Frame, len(*s))
	for i := range f {
		f[i] = Frame((*s)[i])
	}
	return f
}

// The argument skip is the number of stack frames to skip before recording in
// pc, skip == 0 means the caller of callers is the first frame shown.
func callers(skip int) *stack {
	const depth = 32
	var pcs [depth]uintptr
	// skip runtime.Callers and this function itself
	skip += 2
	n := runtime.Callers(skip, pcs[:])
	var st stack = pcs[0:n]
	return &st
}

// funcname removes the path prefix component of a function's name reported by func.Name().
func funcname(name string) string {
	i := strings.LastIndex(name, "/")
	name = name[i+1:]
	i = strings.Index(name, ".")
	return name[i+1:]
}
