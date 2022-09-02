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

const (
	// The String method processing logic of Desc is dependent on this key's corresponding value.
	// module: default, proto, conf.
	Module = "Module"

	BookName  = "BookName"  // workbook name
	SheetName = "SheetName" // worksheet name
	CellPos   = "CellPos"   // cell position
	CellData  = "CellData"  // cell data
	NameCell  = "NameCell"  // name cell
	TypeCell  = "TypeCell"  // type cell

	PBMessage   = "PBMessage"   // protobuf message name
	PBFieldName = "PBFieldName" // protobuf message field name
	PBFieldType = "PBFieldType" // protobuf message field type
	PBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	ColumnName  = "ColumnName"  // column name

	Reason = "Reason" // error
)

type Desc struct {
	Module string

	BookName  string
	SheetName string
	CellPos   string
	CellData  string
	NameCell  string
	TypeCell  string

	PBMessage   string
	PBFieldName string
	PBFieldType string
	PBFieldOpts string
	ColumnName  string

	Reason string

	Error error
}

func NewDesc(err error) *Desc {
	desc := &Desc{
		Module: ModuleDefault,

		BookName:  "UNKNOWN",
		SheetName: "UNKNOWN",
		CellPos:   "UNKNOWN",
		CellData:  "UNKNOWN",
		NameCell:  "UNKNOWN",
		TypeCell:  "UNKNOWN",

		PBMessage:   "UNKNOWN",
		PBFieldName: "UNKNOWN",
		PBFieldType: "UNKNOWN",
		PBFieldOpts: "UNKNOWN",
		ColumnName:  "UNKNOWN",

		Reason: "UNKNOWN",

		Error: err,
	}

	splits := strings.Split(err.Error(), "|")
	for _, s := range splits {
		kv := strings.SplitN(s, ":", 2)
		if len(kv) == 2 {
			key, val := strings.Trim(kv[0], " :"), strings.Trim(kv[1], " :")
			desc.updateField(key, val)
		}
	}
	return desc
}

func (d *Desc) updateField(name, value string) {
	switch name {
	case Module:
		d.Module = value
	case BookName:
		d.BookName = value
	case SheetName:
		d.SheetName = value
	case CellPos:
		d.CellPos = value
	case CellData:
		d.CellData = value
	case NameCell:
		d.NameCell = value
	case TypeCell:
		d.TypeCell = value

	case PBMessage:
		d.PBMessage = value
	case PBFieldName:
		d.PBFieldName = value
	case PBFieldType:
		d.PBFieldType = value
	case PBFieldOpts:
		d.PBFieldOpts = value

	case Reason:
		d.Reason = value
	default:
		// log.DPanicf("unkown name: %s", name)
	}
}

// StringZh render description in English.
func (d *Desc) String() string {
	switch d.Module {
	case ModuleProto:
		return d.Error.Error()
	case ModuleConf:
		overview := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse Cell[%s] value \"%s\" failed: %s", d.BookName, d.SheetName, d.CellPos, d.CellData, d.Reason)
		details := fmt.Sprintf("\nDetails: \n\tPBMessage: %s\n\tPBFieldType: %s\n\tPBFieldName: %s\n\tPBFieldOpts: %s", d.PBMessage, d.PBFieldType, d.PBFieldName, d.PBFieldOpts)
		return overview + details
	default:
		return fmt.Sprintf("Error: %s", d.Reason)
	}
}

// StringZh render description in Chinese.
func (d *Desc) StringZh() string {
	switch d.Module {
	case ModuleProto:
		return d.Error.Error()
	case ModuleConf:
		overview := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 单元格[%s]中的值\"%s\"解析失败: %s", d.BookName, d.SheetName, d.CellPos, d.CellData, d.Reason)
		details := fmt.Sprintf("\nDetails: \n\t表单结构: %s\n\t字段类型: %s\n\t字段名称: %s\n\t字段选项: %s", d.PBMessage, d.PBFieldType, d.PBFieldName, d.PBFieldOpts)
		return overview + details
	default:
		return fmt.Sprintf("Error: %s", d.Error)
	}
}
