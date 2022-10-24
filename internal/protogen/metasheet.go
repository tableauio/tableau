package protogen

import "strings"

func parseIndexes(str string) []string {
	if strings.TrimSpace(str) == "" {
		return nil
	}

	var indexes []string
	var hasGroupLeft, hasGroupRight bool
	start := 0
	for i := 0; i <= len(str); i++ {
		if i == len(str) {
			indexes = appendIndex(indexes, str, start, i)
			break
		}

		switch str[i] {
		case '(':
			hasGroupLeft = true
		case ')':
			hasGroupRight = true
		case ',':
			if (!hasGroupLeft && !hasGroupRight) || (hasGroupLeft && hasGroupRight) {
				indexes = appendIndex(indexes, str, start, i)

				start = i + 1 // skip ',' to next rune
				hasGroupLeft, hasGroupRight = false, false
			}
		}
	}
	return indexes
}

func appendIndex(indexes []string, str string, start, end int) []string {
	index := strings.TrimSpace(str[start:end])
	if index != "" {
		indexes = append(indexes, index)
	}
	return indexes
}
