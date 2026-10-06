package tableau

import "github.com/tableauio/tableau/internal/x/xerrors"

// Error reports one or more structured failures from Tableau. Use errors.As
// to obtain it from a wrapped error. Details contains one entry per failure,
// including when only one cell fails. Error renders these details as a
// localized summary. JSON omits underlying errors and stack traces.
type Error = xerrors.Error

// ErrorDetail describes one failure, independently of error wrappers and
// localized text formatting. Source and Field are omitted when unavailable.
type ErrorDetail = xerrors.ErrorDetail

// SourceLocation identifies the actual source of a failure. PrimaryWorkbook
// and PrimaryWorksheet identify the schema's source when a shard is loaded.
type SourceLocation = xerrors.SourceLocation

// CellLocation contains the source position and data of a cell. Position can
// also identify a node in a document input. TrimmedData is used for headers.
type CellLocation = xerrors.CellLocation

// FieldLocation identifies the protobuf message and field associated with a
// failure. Options contains the textual Tableau field options, when available.
type FieldLocation = xerrors.FieldLocation
