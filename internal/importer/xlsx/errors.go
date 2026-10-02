package xlsx

import "errors"

// ErrUnsupported marks valid workbook features that require the compatibility
// reader. Malformed data and I/O failures are not compatibility failures.
var ErrUnsupported = errors.New("unsupported XLSX feature")
