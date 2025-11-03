package xerrors

import "fmt"

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
	if e.code == "" {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.code, e.desc)
}

func (e *ecode) Is(target error) bool {
	t, ok := target.(*ecode)
	return ok && e.code == t.code
}
