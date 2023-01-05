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

	KeyIndir           = "Indir"           // input dir
	KeySubdir          = "Subdir"          // input subdir
	KeyOutdir          = "Outdir"          // output dir
	KeyBookName        = "BookName"        // workbook name
	KeySheetName       = "SheetName"       // worksheet name
	KeyNameCellPos     = "NameCellPos"     // name cell position
	KeyNameCell        = "NameCell"        // name cell value
	KeyTrimmedNameCell = "TrimmedNameCell" // trimmed name cell value
	KeyTypeCellPos     = "TypeCellPos"     // type cell position
	KeyTypeCell        = "TypeCell"        // type cell value
	KeyDataCellPos     = "DataCellPos"     // data cell position
	KeyDataCell        = "DataCell"        // data data value

	KeyPBMessage   = "PBMessage"   // protobuf message name
	KeyPBFieldName = "PBFieldName" // protobuf message field name
	KeyPBFieldType = "PBFieldType" // protobuf message field type
	KeyPBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	KeyColumnName  = "ColumnName"  // column name

	// private keys below
	keyErrCode = "ErrCode"
	keyErrDesc = "ErrDesc"
	keyReason  = "Reason" // error
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
	KeySheetName,
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
	keyReason,
	keyHelp,
}

type Desc struct {
	err    error
	fields map[string]interface{}
}

func NewDesc(err error) *Desc {
	desc := &Desc{
		err:    err,
		fields: map[string]interface{}{},
	}

	splits := strings.Split(err.Error(), "|")
	for _, s := range splits {
		kv := strings.SplitN(s, ":", 2)
		if len(kv) == 2 {
			key, val := strings.Trim(kv[0], " :"), strings.Trim(kv[1], " :")
			desc.setField(key, val)
		}
	}
	return desc
}

func (d *Desc) setField(key, val string) {
	d.fields[key] = val
}

func (d *Desc) ErrCode() string {
	val := d.fields["ErrCode"]
	if val != nil {
		ecode, ok := val.(string)
		if ok {
			return ecode
		}
	}
	return ""
}

// String render description in specified language.
func (d *Desc) String() string {
	if d.fields[keyReason] == nil {
		return fmt.Sprintf("Error: %s", d.err.Error())
	}
	debugging := fmt.Sprintf("Debugging: \n%s\n", d.DebugString())
	module := d.fields[KeyModule].(string)
	switch module {
	case ModuleProto:
		return debugging + renderSummary(module, d.fields)
	case ModuleConf:
		return debugging + renderSummary(module, d.fields)
	default:
		return fmt.Sprintf("Error: %s", d.err)
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
