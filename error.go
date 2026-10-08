package tableau

import "github.com/tableauio/tableau/internal/x/xerrors"

// Error reports one or more structured failures from Tableau. Use Inspect to
// obtain it from an operation's error.
// Details contains one entry per failure, including when only one cell fails.
// Error renders these details as a localized summary. JSON omits underlying
// errors and stack traces.
type Error = xerrors.Error

// ErrorDetail describes one failure, independently of error wrappers and
// localized text formatting. Source and Field are omitted when unavailable.
type ErrorDetail = xerrors.ErrorDetail

// SourceLocation identifies the actual source of a failure. PrimaryWorkbook
// and PrimaryWorksheet identify the schema's source when a shard is loaded.
// Aliases and merger/scatter specifiers describe that schema, independently
// of the actual workbook, worksheet, and cell where the failure occurred.
// WorksheetAlias is the protobuf message name when it differs from the
// schema worksheet name.
type SourceLocation = xerrors.SourceLocation

// CellLocation contains the source position and data of a cell. Position can
// also identify a node in a document input. TrimmedData is used for headers.
type CellLocation = xerrors.CellLocation

// FieldLocation identifies the protobuf message and field associated with a
// failure. Options contains the textual Tableau field options, when available.
type FieldLocation = xerrors.FieldLocation

// Inspect collects all failures in err into an independent *Error snapshot.
// Wrapped and joined errors are flattened into Details, preserving each
// failure's metadata. The Error method renders the details consistently,
// numbering multiple failures in one sequence: [1], [2], [3], ...
// Call Inspect once at the reporting boundary.
//
// Editing the snapshot does not change err. Original causes remain reachable
// through errors.Is and errors.As. An ordinary Go error yields one detail
// containing its message; nil returns nil.
func Inspect(err error) *Error {
	return xerrors.Inspect(err)
}

// WrapKV adds metadata to err. It captures the caller's stack only when err
// has no stack trace.
// Existing metadata on each failure takes precedence over these values.
// Fields on an errors.Join result apply to its children; Tableau's collected
// errors retain their individual source scopes. Use Inspect for reporting.
// It preserves the cause chain and returns nil when err is nil.
// keysAndValues contains alternating keys and values; an odd length panics.
func WrapKV(err error, keysAndValues ...any) error {
	return xerrors.WrapKVWithCallerSkip(1, err, keysAndValues...)
}

// E0005 marks custom check failures with code E0005 (custom check failed).
// Joined errors retain one detail per failure and their original messages.
// Existing codes and metadata take precedence; causes and stacks are preserved.
// Nil returns nil. Use Inspect at the reporting boundary for text or JSON.
func E0005(err error) error {
	return xerrors.WrapEcodeWithCallerSkip(1, err, xerrors.ErrE0005)
}
