package load

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/tableauio/tableau/format"
)

func extractLinesOnUnmarshalError(err error, f format.Format, data []byte) string {
	// only JSON and text formats are supported.
	if f != format.JSON && f != format.Text {
		return ""
	}
	const deltaLines = 5
	ln, _ := extractLineAndColumn(err.Error())
	minLine := ln - deltaLines // negative is ok
	maxLine := ln + deltaLines // over max file line number is ok

	scanner := bufio.NewScanner(bytes.NewReader(data))
	lines := "\n\n"
	line := 0
	for scanner.Scan() {
		line++
		if line >= minLine && line <= maxLine {
			lines += fmt.Sprintf("%6d\t%s\n", line, scanner.Text())
		}
	}
	return lines
}

func extractLineAndColumn(errmsg string) (int, int) {
	re := regexp.MustCompile(`\(line (\d+):(\d+)\)`) // e.g.: (line 11:9)
	matches := re.FindStringSubmatch(errmsg)
	if len(matches) != 3 {
		return 0, 0
	}
	line, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, 0
	}
	column, err := strconv.Atoi(matches[2])
	if err != nil {
		return 0, 0
	}
	return line, column
}
