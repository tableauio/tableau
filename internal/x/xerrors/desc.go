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

// desc keys for bookkeeping
const (
	// The String method processing logic of Desc is dependent on this key's corresponding value.
	// module: default, proto, conf.
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
	// In addition to telling the user exactly why their code is wrong, it's oftentimes
	// furthermore possible to tell them how to fix it.
	//
	// See https://rustc-dev-guide.rust-lang.org/diagnostics.html#suggestions
	keyHelp = "Help"
)

// ordered keys for debugging
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

// Desc holds the structured fields extracted from a single error chain.
// When the error is a joined multi-error, children holds one *Desc per child
// and the numbered-list rendering is handled by ErrString/String.
type Desc struct {
	err      error
	fields   map[string]any
	children []*Desc
}

// collectFields traverses the error chain of err and collects all structured
// fields from each layer that implements fieldsCarrier. The innermost layer
// wins when the same key appears in multiple layers (later iterations overwrite
// as we walk from outer to inner).
//
// When a joined multi-error (errors.Join) is encountered, fields are collected
// from the first child so that E2027-style errors wrapped inside a join are
// still reachable.
func collectFields(err error) map[string]any {
	type multiUnwrapper interface {
		Unwrap() []error
	}
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

// mergeOuterFields merges outerFields into d and all its descendants
// (inner fields always win over outer).
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

// flattenDescs recursively collects all leaf *Desc nodes (those without
// children) from d into dst. This fully flattens any depth of nested joins.
func flattenDescs(d *Desc, dst *[]*Desc) {
	if len(d.children) == 0 {
		*dst = append(*dst, d)
		return
	}
	for _, child := range d.children {
		flattenDescs(child, dst)
	}
}

// NewDesc extracts structured fields from err and returns a *Desc.
//
// It transparently traverses single-chain wrappers (e.g. WrapKV) to find an
// inner errors.Join node, then builds one *Desc per child while merging the
// outer wrapper fields (e.g. Module, BookName, SheetName) into each child.
//
// Multi-layer joins are handled recursively and fully flattened: regardless
// of how many levels of nested joins exist, all leaf errors are collected into
// a single flat children list, each carrying the accumulated outer fields from
// every enclosing WrapKV layer (innermost fields win on key conflicts).
//
// Returns nil when err is nil.
func NewDesc(err error) *Desc {
	if err == nil {
		return nil
	}
	type multiUnwrapper interface {
		Unwrap() []error
	}

	// Walk the single-chain collecting outer fields, until we hit a
	// multi-error node or the end of the chain.
	outerFields := make(map[string]any)
	cur := err
	for cur != nil {
		if fc, ok := cur.(fieldsCarrier); ok {
			maps.Copy(outerFields, fc.Fields())
		}
		if mu, ok := cur.(multiUnwrapper); ok {
			// Found the join node – recursively expand each child.
			children := mu.Unwrap()
			var nonNil []error
			for _, c := range children {
				if c != nil {
					nonNil = append(nonNil, c)
				}
			}
			if len(nonNil) == 0 {
				return nil
			}
			// Recursively expand every child, then fully flatten all leaf nodes.
			var leaves []*Desc
			for _, c := range nonNil {
				inner := NewDesc(c)
				if inner == nil {
					continue
				}
				// Merge accumulated outer fields into the subtree (inner wins).
				mergeOuterFields(inner, outerFields)
				// Flatten: collect all leaf Descs from this subtree.
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
		cur = errors.Unwrap(cur)
	}
	// No multi-error found – treat as a single error.
	return newDescFromSingle(err)
}

func newDescFromSingle(err error) *Desc {
	return &Desc{
		err:    err,
		fields: collectFields(err),
	}
}

// ErrCode returns the error code stored in the structured fields, or "".
func (d *Desc) ErrCode() string {
	val := d.GetValue(keyErrCode)
	if val != nil {
		if ec, ok := val.(string); ok {
			return ec
		}
	}
	return ""
}

// Children returns the individual child *Desc entries when the error is a
// joined multi-error, or nil for a single error.
func (d *Desc) Children() []*Desc {
	return d.children
}

// String renders the description.
func (d *Desc) String() string {
	return d.ErrString(false)
}

// ErrString renders the description with optional debug info.
// For joined multi-errors it renders a numbered list, one entry per child.
// When withDebug is true, each error also includes its stack trace.
func (d *Desc) ErrString(withDebug bool) string {
	// Multi-error: render numbered list.
	if len(d.children) > 0 {
		var sb strings.Builder
		for i, child := range d.children {
			if i > 0 {
				sb.WriteString("\n")
			}
			fmt.Fprintf(&sb, "[%d] %s", i+1, child.ErrString(withDebug))
		}
		return sb.String()
	}
	// Single error.
	if d.err == nil {
		return ""
	}
	if d.fields[KeyReason] == nil {
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
		return d.err.Error()
	}
}

// fieldsString returns a multi-line string of all structured fields in order.
func (d *Desc) fieldsString() string {
	var lines []string
	for _, key := range keys {
		if val := d.fields[key]; val != nil {
			lines = append(lines, fmt.Sprintf("%s: %v", key, val))
		}
	}
	return strings.Join(lines, "\n")
}

// stackString extracts and formats the stack trace from d.err.
// Returns an empty string if no stack trace is available.
func (d *Desc) stackString() string {
	var berr *base
	if !errors.As(d.err, &berr) || berr.stack == nil {
		return ""
	}
	return fmt.Sprintf("%+v", berr.stack)
}

// GetValue returns the value associated with key, or nil if not present.
func (d *Desc) GetValue(key string) any {
	return d.fields[key]
}
