package xerrors

import (
	"fmt"
)

const (
	BookName  = "BookName"  // workbook name
	BookPath  = "BookPath"  // workbook path
	SheetName = "SheetName" // worksheet name
	CellPos   = "CellPos"   // cell position
	CellData  = "CellData"  // cell data

	PBMessage   = "PBMessage"   // protobuf message name
	PBFieldName = "PBFieldName" // protobuf message field name
	PBFieldType = "PBFieldType" // protobuf message field type
	PBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	ColumnName  = "ColumnName"  // column name

	Error = "Error" // error
)

type Desc struct {
	BookName  string
	BookPath  string
	SheetName string
	CellPos   string
	CellData  string

	PBMessage   string
	PBFieldName string
	PBFieldType string
	PBFieldOpts string
	ColumnName  string

	Error string
}

func NewDesc() *Desc {
	return &Desc{
		BookName:  "UNKNOWN",
		BookPath:  "UNKNOWN",
		SheetName: "UNKNOWN",
		CellPos:   "UNKNOWN",
		CellData:  "UNKNOWN",

		PBMessage:   "UNKNOWN",
		PBFieldName: "UNKNOWN",
		PBFieldType: "UNKNOWN",
		PBFieldOpts: "UNKNOWN",
		ColumnName:  "UNKNOWN",

		Error: "UNKNOWN",
	}
}

func (d *Desc) UpdateField(name, value string) {
	switch name {
	case BookName:
		d.BookName = value
	case BookPath:
		d.BookPath = value
	case SheetName:
		d.SheetName = value
	case CellPos:
		d.CellPos = value
	case CellData:
		d.CellData = value

	case PBMessage:
		d.PBMessage = value
	case PBFieldName:
		d.PBFieldName = value
	case PBFieldType:
		d.PBFieldType = value
	case PBFieldOpts:
		d.PBFieldOpts = value

	case Error:
		d.Error = value
	default:
		// log.DPanicf("unkown name: %s", name)
	}
}

// StringZh render description in English.
func (d *Desc) String() string {
	overview := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse Cell[%s] value \"%s\" failed: %s", d.BookName, d.SheetName, d.CellPos, d.CellData, d.Error)
	details := fmt.Sprintf("\nDetails: \n\tPBMessage: %s\n\tPBFieldType: %s\n\tPBFieldName: %s\n\tPBFieldOpts: %s", d.PBMessage, d.PBFieldName, d.PBFieldType, d.PBFieldOpts)
	return overview + details
}

// StringZh render description in Chinese.
func (d *Desc) StringZh() string {
	overview := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 单元格[%s]中的值\"%s\"解析失败: %s", d.BookName, d.SheetName, d.CellPos, d.CellData, d.Error)
	details := fmt.Sprintf("\nDetails: \n\t表单结构: %s\n\t字段类型: %s\n\t字段名称: %s\n\t字段选项: %s", d.PBMessage, d.PBFieldName, d.PBFieldType, d.PBFieldOpts)
	return overview + details
}
