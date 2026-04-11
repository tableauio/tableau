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

// NewDesc extracts structured fields from err and returns a *Desc.
//
// It transparently traverses single-chain wrappers (e.g. WrapKV) to find an
// inner errors.Join node, then builds one *Desc per child while merging the
// outer wrapper fields (e.g. Module, BookName, SheetName) into each child.
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
			// Found the join node – build one Desc per child.
			children := mu.Unwrap()
			var nonNil []error
			for _, c := range children {
				if c != nil {
					nonNil = append(nonNil, c)
				}
			}
			switch len(nonNil) {
			case 0:
				return nil
			case 1:
				// Merge outer fields with child fields (innermost wins).
				merged := make(map[string]any, len(outerFields))
				maps.Copy(merged, outerFields)
				maps.Copy(merged, collectFields(nonNil[0]))
				return &Desc{err: nonNil[0], fields: merged}
			default:
				descs := make([]*Desc, 0, len(nonNil))
				for _, c := range nonNil {
					merged := make(map[string]any, len(outerFields))
					maps.Copy(merged, outerFields)
					maps.Copy(merged, collectFields(c))
					descs = append(descs, &Desc{err: c, fields: merged})
				}
				return &Desc{err: err, children: descs}
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
			errmsg = fmt.Sprintf("Debugging: \n%s\n", d.debugString()) + errmsg
		}
		return errmsg
	default:
		return d.err.Error()
	}
}

// debugString returns a multi-line string of all structured fields in order.
func (d *Desc) debugString() string {
	str := new(strings.Builder)
	for _, key := range keys {
		if val := d.fields[key]; val != nil {
			fmt.Fprintf(str, "\t%s: %v\n", key, val)
		}
	}
	return str.String()
}

// GetValue returns the value associated with key, or nil if not present.
func (d *Desc) GetValue(key string) any {
	return d.fields[key]
}
