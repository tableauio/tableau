package xerrors

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"
)

// Error reports one or more structured failures from Tableau. Use Inspect to
// obtain it from an operation's error, or Normalize followed by errors.As.
// Details contains one entry per failure, including when only one cell fails.
// Error renders these details as a localized summary. JSON omits underlying
// errors and stack traces.
type Error struct {
	Details []*ErrorDetail `json:"details"`
	cause   error
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
	cause       error           // runtime-only cause for debugging stacks
	parameters  map[string]any  // error-specific parameters, excluding typed metadata
	emptyFields []string        // distinguishes absent metadata from explicit empty values
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

// GetValue returns the value associated with key in the first detail, or nil
// if the error has no details or the key is absent. Use Details to inspect
// individual failures in an aggregate error.
func (e *Error) GetValue(key string) any {
	if e == nil || len(e.Details) == 0 {
		return nil
	}
	return e.Details[0].GetValue(key)
}

// GetValue returns the value associated with key in the detail, or nil if the
// key is absent. It supports typed source and field metadata as well as
// error-specific parameters.
func (d *ErrorDetail) GetValue(key string) any {
	if d == nil {
		return nil
	}
	return d.fields()[key]
}

// Normalize converts an error to *Error when it carries structured metadata.
// It resolves joins and scoped metadata, preserving the original cause chain.
// Nil, ordinary Go errors, and existing *Error values are
// returned unchanged. Use Wrap, Wrapf, or WrapKV to add context before this step.
func Normalize(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*Error); ok {
		return err
	}
	entries := extractEntries(err)
	structured := false
	for _, entry := range entries {
		structured = structured || len(entry.fields) > 0
	}
	if !structured {
		return err
	}
	return buildError(err, entries)
}

// Inspect returns an independent snapshot of an error's structured details.
// Ordinary errors have one detail containing their message; nil returns nil.
// Editing the snapshot does not change the input error's details. The original
// error chain remains reachable through errors.Is and errors.As.
func Inspect(err error) *Error {
	if err == nil {
		return nil
	}
	return buildError(err, extractEntries(err))
}

func buildError(cause error, entries []errorEntry) *Error {
	if len(entries) == 0 {
		return nil
	}
	e := &Error{cause: cause, Details: make([]*ErrorDetail, 0, len(entries))}
	for _, entry := range entries {
		detail := buildDetail(entry.cause, entry.fields)
		if entry.preserveMessage {
			detail.Message = errorField(entry.fields, KeyReason)
		}
		e.Details = append(e.Details, detail)
	}
	return e
}

// Error implements error by rendering the canonical details.
func (e *Error) Error() string { return e.stringify(false) }

// Unwrap preserves the original error chain.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Format includes structured debugging fields and stacks only for %+v.
func (e *Error) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		_, _ = io.WriteString(s, e.stringify(s.Flag('+')))
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}

func (e *Error) stringify(debug bool) string {
	if e == nil {
		return ""
	}
	var rendered []string
	for _, detail := range e.Details {
		if detail != nil {
			rendered = append(rendered, detail.stringify(debug))
		}
	}
	if len(rendered) == 1 {
		return rendered[0]
	}
	var sb strings.Builder
	for i, text := range rendered {
		if i > 0 {
			sb.WriteByte('\n')
		}
		fmt.Fprintf(&sb, "[%d] %s", i+1, text)
	}
	return sb.String()
}

// String renders this detail using the shared localized summary template.
func (d *ErrorDetail) String() string { return d.stringify(false) }

func (d *ErrorDetail) stringify(debug bool) string {
	if d == nil {
		return ""
	}
	fields := d.fields()
	var text string
	switch {
	case d.Code != "" && (d.Module == ModuleDefault || d.Module == ModuleProto || d.Module == ModuleConf):
		text = renderSummary(d.Module, fields)
	case d.Source != nil:
		text = renderSummary(ModuleDefault, fields)
	default:
		text = d.Message
	}
	if !debug {
		return text
	}
	var stackText string
	var berr *base
	if errors.As(d.cause, &berr) && berr.stack != nil {
		stackText = fmt.Sprintf("%+v", berr.stack)
	}
	switch d.Module {
	case ModuleDefault, ModuleProto, ModuleConf:
		if d.Code != "" {
			return text + "\n--- debugging ---\n" + fieldsString(fields) + "\n" + stackText + "\n"
		}
		return text + stackText
	default:
		return text + stackText
	}
}

// fields adapts the typed detail for existing localization templates. It never
// stores a second copy of the source metadata or mutates the detail.
func (d *ErrorDetail) fields() map[string]any {
	fields := maps.Clone(d.parameters)
	if fields == nil {
		fields = make(map[string]any)
	}
	for _, key := range d.emptyFields {
		fields[key] = ""
	}
	put := func(key, value string) {
		if value != "" {
			fields[key] = value
		}
	}
	put(KeyErrCode, d.Code)
	put(KeyErrDesc, d.Description)
	put(KeyReason, d.Message)
	put(KeyHelp, d.Help)
	put(KeyModule, d.Module)
	cell := func(c *CellLocation, pos, data, trimmed string) {
		if c != nil {
			put(pos, c.Position)
			put(data, c.Data)
			if trimmed != "" {
				put(trimmed, c.TrimmedData)
			}
		}
	}
	if s := d.Source; s != nil {
		put(KeyBookName, s.Workbook)
		put(KeyPrimaryBookName, s.PrimaryWorkbook)
		put(KeySheetName, s.Worksheet)
		put(KeyPrimarySheetName, s.PrimaryWorksheet)
		put(KeyReferBookName, s.ReferencedWorkbook)
		put(KeyReferSheetName, s.ReferencedWorksheet)
		put(KeyIndir, s.InputDir)
		put(KeySubdir, s.Subdir)
		put(KeyOutdir, s.OutputDir)
		cell(s.Cell, KeyDataCellPos, KeyDataCell, "")
		cell(s.NameCell, KeyNameCellPos, KeyNameCell, KeyTrimmedNameCell)
		cell(s.TypeCell, KeyTypeCellPos, KeyTypeCell, "")
		cell(s.NoteCell, KeyNoteCellPos, KeyNoteCell, "")
	}
	if f := d.Field; f != nil {
		put(KeyPBMessage, f.Message)
		put(KeyPBFieldName, f.Name)
		put(KeyPBFieldType, f.Type)
		put(KeyPBFieldOpts, f.Options)
		put(KeyColumnName, f.Column)
	}
	return fields
}

func normalizeFields(fields map[string]any) {
	if fields[KeyReason] == nil {
		return
	}
	if fields[KeyModule] == nil && fields[KeyErrCode] != nil {
		fields[KeyModule] = ModuleDefault
	}
	module, _ := fields[KeyModule].(string)
	switch module {
	case ModuleDefault, ModuleProto, ModuleConf:
		ensureEcode(fields)
	}
}

func fieldsString(fields map[string]any) string {
	var lines []string
	for _, key := range debugKeyOrder {
		if val := fields[key]; val != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", key, val))
		}
	}
	return strings.Join(lines, "\n")
}

// detailKeys are the metadata represented by the typed public model.
var detailKeys = append(append([]string(nil), debugKeyOrder...), KeyNoteCellPos, KeyNoteCell)

func buildDetail(cause error, fields map[string]any) *ErrorDetail {
	normalizeFields(fields)
	detail := &ErrorDetail{
		cause:       cause,
		Code:        errorField(fields, KeyErrCode),
		Description: errorField(fields, KeyErrDesc),
		Message:     errorField(fields, KeyReason),
		Help:        errorField(fields, KeyHelp),
		Module:      errorField(fields, KeyModule),
	}
	switch detail.Module {
	case ModuleDefault, ModuleProto, ModuleConf:
		if fields[KeyReason] == nil && cause != nil {
			detail.Message = cause.Error()
		}
	default:
		if cause != nil {
			detail.Message = cause.Error()
		}
	}
	source := &SourceLocation{
		Workbook:            errorField(fields, KeyBookName),
		PrimaryWorkbook:     errorField(fields, KeyPrimaryBookName),
		Worksheet:           errorField(fields, KeySheetName),
		PrimaryWorksheet:    errorField(fields, KeyPrimarySheetName),
		ReferencedWorkbook:  errorField(fields, KeyReferBookName),
		ReferencedWorksheet: errorField(fields, KeyReferSheetName),
		InputDir:            errorField(fields, KeyIndir),
		Subdir:              errorField(fields, KeySubdir),
		OutputDir:           errorField(fields, KeyOutdir),
		Cell:                errorCell(fields, KeyDataCellPos, KeyDataCell, ""),
		NameCell:            errorCell(fields, KeyNameCellPos, KeyNameCell, KeyTrimmedNameCell),
		TypeCell:            errorCell(fields, KeyTypeCellPos, KeyTypeCell, ""),
		NoteCell:            errorCell(fields, KeyNoteCellPos, KeyNoteCell, ""),
	}
	if *source != (SourceLocation{}) {
		detail.Source = source
	}
	field := &FieldLocation{
		Message: errorField(fields, KeyPBMessage),
		Name:    errorField(fields, KeyPBFieldName),
		Type:    errorField(fields, KeyPBFieldType),
		Options: errorField(fields, KeyPBFieldOpts),
		Column:  errorField(fields, KeyColumnName),
	}
	if *field != (FieldLocation{}) {
		detail.Field = field
	}
	// Keep only error-specific parameters and empty-value presence. Known
	// metadata lives exclusively in the typed fields above.
	detail.parameters = maps.Clone(fields)
	for _, key := range detailKeys {
		if value, ok := fields[key]; ok && value != nil && fmt.Sprint(value) == "" {
			detail.emptyFields = append(detail.emptyFields, key)
		}
		delete(detail.parameters, key)
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
