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

	// private keys
	keyReason = "Reason" // error
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

	keyReason,
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

// StringZh render description in English.
func (d *Desc) String() string {
	if d.fields[keyReason] == nil {
		return fmt.Sprintf("Error: %s", d.err.Error())
	}
	debugging := fmt.Sprintf("Debugging: \n%s\n", d.DebugString())
	switch d.fields[KeyModule] {
	case ModuleProto:
		errLine := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse KeyNameCell[%s] \"%s\" and KeyTypeCell[%s] \"%s|%s\" failed: %s", d.fields[KeyBookName], d.fields[KeySheetName], d.fields[KeyNameCellPos], d.fields[KeyNameCell], d.fields[KeyTypeCellPos], d.fields[KeyTypeCell], d.fields[KeyPBFieldOpts], d.fields[keyReason])
		return debugging + errLine
	case ModuleConf:
		errLine := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse Cell[%s] \"%s\" failed: %s", d.fields[KeyBookName], d.fields[KeySheetName], d.fields[KeyDataCellPos], d.fields[KeyDataCell], d.fields[keyReason])
		return debugging + errLine
	default:
		return fmt.Sprintf("Error: %s", d.err)
	}
}

// StringZh render description in Chinese.
func (d *Desc) StringZh() string {
	if d.fields[keyReason] == nil {
		return fmt.Sprintf("Error: %s", d.err.Error())
	}

	debugging := fmt.Sprintf("Debugging: \n%s\n", d.DebugString())
	switch d.fields[KeyModule] {
	case ModuleProto:
		errLine := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 命名单元格[%s]的值\"%s\"和类型单元格[%s]的值\"%s|%s\"解析失败: %s", d.fields[KeyBookName], d.fields[KeySheetName], d.fields[KeyNameCellPos], d.fields[KeyNameCell], d.fields[KeyTypeCellPos], d.fields[KeyTypeCell], d.fields[KeyPBFieldOpts], d.fields[keyReason])
		return debugging + errLine
	case ModuleConf:
		errLine := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 单元格[%s]中的值\"%s\"解析失败: %s", d.fields[KeyBookName], d.fields[KeySheetName], d.fields[KeyDataCellPos], d.fields[KeyDataCell], d.fields[keyReason])
		return debugging + errLine
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
