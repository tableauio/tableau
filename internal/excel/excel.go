package excel

// LetterAxis generate the corresponding column name.
// index: 0-based.
func LetterAxis(index int) string {
	var (
		colCode = ""
		key     = 'A'
		loop    = index / 26
	)
	if loop > 0 {
		colCode += LetterAxis(loop - 1)
	}
	return colCode + string(key+int32(index)%26)
}
