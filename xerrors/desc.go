package xerrors

import (
	"fmt"
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
	KeyReason  = "Reason" // error
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

type Desc struct {
	err    error
	fields map[string]any
}

func NewDesc(err error) *Desc {
	if err == nil {
		return nil
	}
	desc := &Desc{
		err:    err,
		fields: map[string]any{},
	}

	// NOTE: In the splits slice, the latter key-value pairs will overwrite
	// earlier ones if they have the same key, so the last one wins.
	splits := strings.Split(err.Error(), "|")
	for _, s := range splits {
		kv := strings.SplitN(s, ":", 2)
		if len(kv) == 2 {
			key, val := strings.Trim(kv[0], " :"), strings.Trim(kv[1], " :")
			if key != "" {
				desc.setField(key, val)
			}
		}
	}
	return desc
}

func (d *Desc) setField(key, val string) {
	d.fields[key] = val
}

func (d *Desc) ErrCode() string {
	val := d.GetValue(keyErrCode)
	if val != nil {
		ecode, ok := val.(string)
		if ok {
			return ecode
		}
	}
	return ""
}

func (d *Desc) String() string {
	return d.ErrString(true)
}

// ErrString renders description in specified language.
func (d *Desc) ErrString(withDebug bool) string {
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
	val := d.GetValue(KeyModule)
	if val != nil {
		module = val.(string)
	}
	switch module {
	case ModuleDefault, ModuleProto, ModuleConf:
		errmsg := renderSummary(module, d.fields)
		if withDebug {
			errmsg = fmt.Sprintf("Debugging: \n%s\n", d.DebugString()) + errmsg
		}
		return errmsg
	default:
		return d.err.Error()
	}
}

func (d *Desc) DebugString() string {
	str := ""
	for _, key := range keys {
		val := d.fields[key]
		if val != nil {
			str += fmt.Sprintf("\t%s: %v\n", key, val)
		}
	}
	return str
}

func (d *Desc) GetValue(key string) any {
	return d.fields[key]
}
