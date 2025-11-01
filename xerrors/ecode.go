package xerrors

import "fmt"

type Error struct {
	Code int
	Desc string
}

func (e *Error) Error() string {
	if e.Code == 0 {
		return ""
	}
	return fmt.Sprintf("E%4d: %s", e.Code, e.Desc)
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}
