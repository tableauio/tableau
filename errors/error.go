// Package errors provides structured Tableau errors for inspection and reporting.
package errors

import "github.com/tableauio/tableau/internal/x/xerrors"

// Error reports one or more structured failures from Tableau. Use Inspect to
// obtain it from an operation's error, or Normalize followed by errors.As.
// Details contains one entry per failure, including when only one cell fails.
// Error renders these details as a localized summary. JSON omits underlying
// errors and stack traces.
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

// Normalize exposes structured Tableau errors for inspection and serialization.
// Call it on an operation's error before using errors.As to obtain *Error.
// Nil, ordinary Go errors, and existing *Error values are returned unchanged;
// converted errors retain their original causes for errors.Is and errors.As.
func Normalize(err error) error {
	return xerrors.Normalize(err)
}

// Inspect returns an independent snapshot of any error's structured details,
// including ordinary Go errors. Nil returns nil. Editing the snapshot does not
// change the input error's details, and its original causes remain reachable
// through errors.Is and errors.As. Use Normalize to retain ordinary Go errors.
func Inspect(err error) *Error {
	return xerrors.Inspect(err)
}
