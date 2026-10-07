package tableau

import "github.com/tableauio/tableau/internal/x/xerrors"

// Source metadata keys for WrapKV and GetValue.
// Inspect maps these fields to ErrorDetail.Source.
const (
	KeyBookName  = xerrors.KeyBookName  // workbook name
	KeySheetName = xerrors.KeySheetName // worksheet name
)
