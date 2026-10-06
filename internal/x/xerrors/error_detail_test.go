package xerrors

func detailFields(value any) map[string]any {
	switch e := value.(type) {
	case *Error:
		return e.Details[0].fields()
	case *ErrorDetail:
		return e.fields()
	default:
		panic("unexpected error test value")
	}
}
