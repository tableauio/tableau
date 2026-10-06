package tableau

import (
	"fmt"
	"io"
	"strings"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// Error reports one or more structured failures from Tableau. Use errors.As
// to obtain it from a wrapped error. Details contains one entry per failure,
// including when only one cell fails. Error renders the localized summary;
// JSON serializes Details without the underlying errors or stack traces.
type Error struct {
	Details []*ErrorDetail `json:"details"`
	cause   error
	desc    *xerrors.Desc
}

// ErrorDetail describes one failure, independently of error wrappers and
// localized text formatting. Source and Field are omitted when unavailable.
type ErrorDetail struct {
	Code        string          `json:"code,omitempty"`
	Description string          `json:"description,omitempty"`
	Message     string          `json:"message"`
	Help        string          `json:"help,omitempty"`
	Module      string          `json:"module,omitempty"`
	Source      *SourceLocation `json:"source,omitempty"`
	Field       *FieldLocation  `json:"field,omitempty"`
}

// SourceLocation identifies the actual source of a failure. PrimaryWorkbook
// and PrimaryWorksheet identify the schema's source when a shard is loaded.
type SourceLocation struct {
	Workbook            string        `json:"workbook,omitempty"`
	PrimaryWorkbook     string        `json:"primaryWorkbook,omitempty"`
	Worksheet           string        `json:"worksheet,omitempty"`
	PrimaryWorksheet    string        `json:"primaryWorksheet,omitempty"`
	ReferencedWorkbook  string        `json:"referencedWorkbook,omitempty"`
	ReferencedWorksheet string        `json:"referencedWorksheet,omitempty"`
	InputDir            string        `json:"inputDir,omitempty"`
	Subdir              string        `json:"subdir,omitempty"`
	OutputDir           string        `json:"outputDir,omitempty"`
	Cell                *CellLocation `json:"cell,omitempty"`
	NameCell            *CellLocation `json:"nameCell,omitempty"`
	TypeCell            *CellLocation `json:"typeCell,omitempty"`
	NoteCell            *CellLocation `json:"noteCell,omitempty"`
}

// CellLocation contains the source position and data of a cell. Position can
// also identify a node in a document input. TrimmedData is used for headers.
type CellLocation struct {
	Position    string `json:"position,omitempty"`
	Data        string `json:"data,omitempty"`
	TrimmedData string `json:"trimmedData,omitempty"`
}

// FieldLocation identifies the protobuf message and field associated with a
// failure. Options contains the textual Tableau field options, when available.
type FieldLocation struct {
	Message string `json:"message,omitempty"`
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	Options string `json:"options,omitempty"`
	Column  string `json:"column,omitempty"`
}

// WrapError exposes structured Tableau failures as *Error. It returns nil for
// nil, preserves ordinary Go errors, and leaves an existing *Error intact.
// The original cause remains available to errors.Is and errors.As.
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	desc := xerrors.NewDesc(err)
	if desc == nil {
		return err
	}
	leaves := desc.Leaves()
	details := make([]*ErrorDetail, 0, len(leaves))
	structured := false
	for _, leaf := range leaves {
		fields := leaf.Fields()
		structured = structured || len(fields) > 0
		details = append(details, errorDetail(leaf, fields))
	}
	if !structured {
		return err
	}
	return &Error{Details: details, cause: err, desc: desc}
}

// Error implements error, using Tableau's shared internal renderer.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.desc != nil {
		return e.desc.String()
	}
	// Publicly constructed errors can also carry details without a renderer.
	var messages []string
	for _, detail := range e.Details {
		if detail != nil {
			messages = append(messages, detail.Message)
		}
	}
	return strings.Join(messages, "\n")
}

// Unwrap preserves the original error chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Format includes internal debugging fields and the stack only for %+v.
func (e *Error) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') && e != nil && e.desc != nil {
		_, _ = io.WriteString(s, e.desc.Stringify(true))
	} else if verb == 'q' {
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	} else {
		_, _ = io.WriteString(s, e.Error())
	}
}

func errorDetail(desc *xerrors.Desc, fields map[string]any) *ErrorDetail {
	detail := &ErrorDetail{
		Code:        errorField(fields, xerrors.KeyErrCode),
		Description: errorField(fields, xerrors.KeyErrDesc),
		Message:     errorField(fields, xerrors.KeyReason),
		Help:        errorField(fields, xerrors.KeyHelp),
		Module:      errorField(fields, xerrors.KeyModule),
	}
	if fields[xerrors.KeyReason] == nil {
		detail.Message = desc.String()
	}
	source := &SourceLocation{
		Workbook:            errorField(fields, xerrors.KeyBookName),
		PrimaryWorkbook:     errorField(fields, xerrors.KeyPrimaryBookName),
		Worksheet:           errorField(fields, xerrors.KeySheetName),
		PrimaryWorksheet:    errorField(fields, xerrors.KeyPrimarySheetName),
		ReferencedWorkbook:  errorField(fields, xerrors.KeyReferBookName),
		ReferencedWorksheet: errorField(fields, xerrors.KeyReferSheetName),
		InputDir:            errorField(fields, xerrors.KeyIndir),
		Subdir:              errorField(fields, xerrors.KeySubdir),
		OutputDir:           errorField(fields, xerrors.KeyOutdir),
		Cell:                errorCell(fields, xerrors.KeyDataCellPos, xerrors.KeyDataCell, ""),
		NameCell:            errorCell(fields, xerrors.KeyNameCellPos, xerrors.KeyNameCell, xerrors.KeyTrimmedNameCell),
		TypeCell:            errorCell(fields, xerrors.KeyTypeCellPos, xerrors.KeyTypeCell, ""),
		NoteCell:            errorCell(fields, xerrors.KeyNoteCellPos, xerrors.KeyNoteCell, ""),
	}
	if *source != (SourceLocation{}) {
		detail.Source = source
	}
	field := &FieldLocation{
		Message: errorField(fields, xerrors.KeyPBMessage),
		Name:    errorField(fields, xerrors.KeyPBFieldName),
		Type:    errorField(fields, xerrors.KeyPBFieldType),
		Options: errorField(fields, xerrors.KeyPBFieldOpts),
		Column:  errorField(fields, xerrors.KeyColumnName),
	}
	if *field != (FieldLocation{}) {
		detail.Field = field
	}
	return detail
}

func errorField(fields map[string]any, key string) string {
	if value := fields[key]; value != nil {
		return fmt.Sprint(value)
	}
	return ""
}

func errorCell(fields map[string]any, positionKey, dataKey, trimmedKey string) *CellLocation {
	_, hasPosition := fields[positionKey]
	_, hasData := fields[dataKey]
	_, hasTrimmed := fields[trimmedKey]
	if !hasPosition && !hasData && !hasTrimmed {
		return nil
	}
	return &CellLocation{
		Position:    errorField(fields, positionKey),
		Data:        errorField(fields, dataKey),
		TrimmedData: errorField(fields, trimmedKey),
	}
}
