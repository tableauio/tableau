package xerrors

import (
	"errors"
	"fmt"
	"maps"
	"strings"
)

const (
	ModuleDefault = "default"
	ModuleProto   = "protogen"
	ModuleConf    = "confgen"
)

// desc keys
const (
	// Drives Desc.Stringify rendering; values: default, proto, conf.
	KeyModule = "Module"

	KeyIndir            = "Indir"            // input dir
	KeySubdir           = "Subdir"           // input subdir
	KeyOutdir           = "Outdir"           // output dir
	KeyBookName         = "BookName"         // workbook name
	KeyPrimaryBookName  = "PrimaryBookName"  // primary workbook name
	KeySheetName        = "SheetName"        // worksheet name
	KeyPrimarySheetName = "PrimarySheetName" // primary worksheet name
	KeyNameCellPos      = "NameCellPos"      // name cell position
	KeyNameCell         = "NameCell"         // name cell value
	KeyTrimmedNameCell  = "TrimmedNameCell"  // trimmed name cell value
	KeyTypeCellPos      = "TypeCellPos"      // type cell position
	KeyTypeCell         = "TypeCell"         // type cell value
	KeyNoteCellPos      = "NoteCellPos"      // note cell position
	KeyNoteCell         = "NoteCell"         // note cell value
	KeyDataCellPos      = "DataCellPos"      // data cell position
	KeyDataCell         = "DataCell"         // data data value

	KeyPBMessage   = "PBMessage"   // protobuf message name
	KeyPBFieldName = "PBFieldName" // protobuf message field name
	KeyPBFieldType = "PBFieldType" // protobuf message field type
	KeyPBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	KeyColumnName  = "ColumnName"  // column name

	keyErrCode = "ErrCode"
	keyErrDesc = "ErrDesc"
	KeyReason  = "Reason" // error reason
	// keyHelp suggests how to fix the error.
	// See https://rustc-dev-guide.rust-lang.org/diagnostics.html#suggestions
	keyHelp = "Help"
)

// keys defines the ordered set of field keys used for debug rendering.
var keys = []string{
	KeyModule,

	KeyIndir,
	KeySubdir,
	KeyOutdir,
	KeyBookName,
	KeyPrimaryBookName,
	KeySheetName,
	KeyPrimarySheetName,
	KeyNameCellPos,
	KeyNameCell,
	KeyTrimmedNameCell,
	KeyTypeCellPos,
	KeyTypeCell,
	KeyDataCellPos,
	KeyDataCell,

	KeyPBMessage,
	KeyPBFieldName,
	KeyPBFieldType,
	KeyPBFieldOpts,
	KeyColumnName,

	keyErrCode,
	keyErrDesc,
	KeyReason,
	keyHelp,
}

// multiUnwrapper is implemented by joined errors (e.g. errors.Join).
type multiUnwrapper interface {
	Unwrap() []error
}

// Desc holds structured fields extracted from an error chain.
// For joined multi-errors, children holds one *Desc per child.
type Desc struct {
	err      error
	fields   map[string]any
	children []*Desc
}

// NewDesc builds a *Desc from err. Single-chain wrappers are traversed to find
// inner multi-errors; all nested joins are fully flattened with outer fields
// (e.g. Module, BookName) merged in (innermost wins). Returns nil for nil err.
func NewDesc(err error) *Desc {
	if err == nil {
		return nil
	}
	// Collect outer fields while walking the chain; stop at the first multi-error.
	outerFields := make(map[string]any)
	cur := err
	for cur != nil {
		if fc, ok := cur.(fieldsCarrier); ok {
			maps.Copy(outerFields, fc.Fields())
		}
		if mu, ok := cur.(multiUnwrapper); ok {
			return buildFromChildren(cur, mu.Unwrap(), outerFields)
		}
		cur = errors.Unwrap(cur)
	}
	// No multi-error found: treat as a single error.
	return &Desc{err: err, fields: collectFields(err)}
}

// newDescWithOuter builds a *Desc for a joinError using pre-supplied outerFields,
// avoiding a re-entrant call to Error().
func newDescWithOuter(err error, outerFields map[string]any) *Desc {
	mu, ok := err.(multiUnwrapper)
	if !ok {
		return nil
	}
	return buildFromChildren(err, mu.Unwrap(), outerFields)
}

// collectFields walks the error chain and collects fields from every
// fieldsCarrier layer; innermost values win on key conflicts.
// For joined errors it descends into the first child to stay reachable.
func collectFields(err error) map[string]any {
	fields := make(map[string]any)
	cur := err
	for cur != nil {
		if fc, ok := cur.(fieldsCarrier); ok {
			maps.Copy(fields, fc.Fields())
		}
		next := errors.Unwrap(cur)
		if next == nil {
			// Single-chain unwrap returned nil; check for a joined multi-error
			// and descend into its first child to keep collecting fields.
			if mu, ok := cur.(multiUnwrapper); ok {
				children := mu.Unwrap()
				if len(children) > 0 {
					next = children[0]
				}
			}
		}
		cur = next
	}
	return fields
}

// mergeOuterFields merges outerFields into d and its descendants; inner fields win.
func mergeOuterFields(d *Desc, outerFields map[string]any) {
	if len(outerFields) == 0 {
		return
	}
	if len(d.children) > 0 {
		for _, child := range d.children {
			mergeOuterFields(child, outerFields)
		}
		return
	}
	merged := make(map[string]any, len(outerFields)+len(d.fields))
	maps.Copy(merged, outerFields)
	maps.Copy(merged, d.fields) // inner fields win
	d.fields = merged
}

// flattenDescs appends all leaf *Desc nodes from d into dst.
func flattenDescs(d *Desc, dst *[]*Desc) {
	if len(d.children) == 0 {
		*dst = append(*dst, d)
		return
	}
	for _, child := range d.children {
		flattenDescs(child, dst)
	}
}

// buildFromChildren expands errs into leaf *Desc nodes with outerFields merged in
// (inner fields win) and returns a *Desc rooted at err.
func buildFromChildren(err error, errs []error, outerFields map[string]any) *Desc {
	var leaves []*Desc
	for _, c := range errs {
		if c == nil {
			continue
		}
		inner := NewDesc(c)
		if inner == nil {
			continue
		}
		mergeOuterFields(inner, outerFields)
		flattenDescs(inner, &leaves)
	}
	switch len(leaves) {
	case 0:
		return nil
	case 1:
		return leaves[0]
	default:
		return &Desc{err: err, children: leaves}
	}
}

// String renders the description without debug info.
func (d *Desc) String() string {
	return d.Stringify(false)
}

// Stringify renders the description. When withDebug is true, each error also
// includes its structured fields and stack trace.
// Joined multi-errors are rendered as a numbered list.
func (d *Desc) Stringify(withDebug bool) string {
	// Multi-error: numbered list.
	if len(d.children) > 0 {
		var sb strings.Builder
		for i, child := range d.children {
			if i > 0 {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "[%d] %s", i+1, child.Stringify(withDebug))
		}
		return sb.String()
	}
	// Single error
	if d.err == nil {
		return ""
	}
	if d.fields[KeyReason] == nil {
		if withDebug {
			return d.err.Error() + d.stackString()
		}
		return d.err.Error()
	}
	if d.fields[KeyModule] == nil && d.fields[keyErrCode] != nil {
		d.fields[KeyModule] = ModuleDefault
	}
	var module string
	if val := d.GetValue(KeyModule); val != nil {
		module = val.(string)
	}
	switch module {
	case ModuleDefault, ModuleProto, ModuleConf:
		errmsg := renderSummary(module, d.fields)
		if withDebug {
			errmsg += "\n--- debugging ---\n" + d.fieldsString() + "\n" + d.stackString() + "\n"
		}
		return errmsg
	default:
		if withDebug {
			return d.err.Error() + d.stackString()
		}
		return d.err.Error()
	}
}

// fieldsString returns all structured fields as an ordered multi-line string.
func (d *Desc) fieldsString() string {
	var lines []string
	for _, key := range keys {
		if val := d.fields[key]; val != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", key, val))
		}
	}
	return strings.Join(lines, "\n")
}

// stackString returns the formatted stack trace from d.err, or "" if unavailable.
func (d *Desc) stackString() string {
	var berr *base
	if !errors.As(d.err, &berr) || berr.stack == nil {
		return ""
	}
	return fmt.Sprintf("%+v", berr.stack)
}

// GetValue returns the field value for key, or nil if absent.
func (d *Desc) GetValue(key string) any {
	return d.fields[key]
}
