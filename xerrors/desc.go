package xerrors

import (
	"fmt"
	"reflect"
	"strings"
	"unicode"
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

	Indir           = "Indir"           // input dir
	Subdir          = "Subdir"          // input subdir
	Outdir          = "Outdir"          // output dir
	BookName        = "BookName"        // workbook name
	SheetName       = "SheetName"       // worksheet name
	NameCellPos     = "NameCellPos"     // name cell position
	NameCell        = "NameCell"        // name cell value
	TrimmedNameCell = "TrimmedNameCell" // trimmed name cell value
	TypeCellPos     = "TypeCellPos"     // type cell position
	TypeCell        = "TypeCell"        // type cell value
	DataCellPos     = "DataCellPos"     // data cell position
	DataCell        = "DataCell"        // data data value

	PBMessage   = "PBMessage"   // protobuf message name
	PBFieldName = "PBFieldName" // protobuf message field name
	PBFieldType = "PBFieldType" // protobuf message field type
	PBFieldOpts = "PBFieldOpts" // protobuf message field options (extensions)
	ColumnName  = "ColumnName"  // column name

	Reason = "Reason" // error
)

type Desc struct {
	Module string

	Indir           string
	Subdir          string
	Outdir          string
	BookName        string
	SheetName       string
	NameCellPos     string
	NameCell        string
	TrimmedNameCell string
	TypeCellPos     string
	TypeCell        string
	DataCellPos     string
	DataCell        string

	PBMessage   string
	PBFieldName string
	PBFieldType string
	PBFieldOpts string
	ColumnName  string

	Reason string

	err error
}

const unknown = "UNKNOWN"

func NewDesc(err error) *Desc {
	desc := &Desc{
		Module: ModuleDefault,

		Indir:           unknown,
		Subdir:          unknown,
		Outdir:          unknown,
		BookName:        unknown,
		SheetName:       unknown,
		NameCellPos:     unknown,
		NameCell:        unknown,
		TrimmedNameCell: unknown,
		TypeCellPos:     unknown,
		TypeCell:        unknown,
		DataCellPos:     unknown,
		DataCell:        unknown,

		PBMessage:   unknown,
		PBFieldName: unknown,
		PBFieldType: unknown,
		PBFieldOpts: unknown,
		ColumnName:  unknown,

		Reason: unknown,

		err: err,
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

	case Indir:
		d.Indir = value
	case Subdir:
		d.Subdir = value
	case Outdir:
		d.Outdir = value
	case BookName:
		d.BookName = value
	case SheetName:
		d.SheetName = value
	case NameCellPos:
		d.NameCellPos = value
	case NameCell:
		d.NameCell = value
	case TrimmedNameCell:
		d.TrimmedNameCell = value
	case TypeCellPos:
		d.TypeCellPos = value
	case TypeCell:
		d.TypeCell = value
	case DataCellPos:
		d.DataCellPos = value
	case DataCell:
		d.DataCell = value

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
	if d.Reason == unknown {
		return fmt.Sprintf("Error: %s", d.err.Error())
	}
	switch d.Module {
	case ModuleProto:
		overview := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse NameCell[%s] \"%s\" and TypeCell[%s] \"%s\" failed: %s", d.BookName, d.SheetName, d.NameCellPos, d.NameCell, d.TypeCellPos, d.TypeCell, d.Reason)
		details := fmt.Sprintf("\nDebugging: \n%s", d.DebugString())
		return overview + details
	case ModuleConf:
		overview := fmt.Sprintf("Error: Workbook: %s, Worksheet: %s, parse Cell[%s] \"%s\" failed: %s", d.BookName, d.SheetName, d.DataCellPos, d.DataCell, d.Reason)
		details := fmt.Sprintf("\nDebugging: \n%s", d.DebugString())
		return overview + details
	default:
		return fmt.Sprintf("Error: %s", d.err)
	}
}

// StringZh render description in Chinese.
func (d *Desc) StringZh() string {
	if d.Reason == unknown {
		return fmt.Sprintf("Error: %s", d.err.Error())
	}

	switch d.Module {
	case ModuleProto:
		overview := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 命名单元格[%s]的值\"%s\"和类型单元格[%s]的值\"%s\"解析失败: %s", d.BookName, d.SheetName, d.NameCellPos, d.NameCell, d.TypeCellPos, d.TypeCell, d.Reason)
		details := fmt.Sprintf("\nDebugging: \n%s", d.DebugString())
		return overview + details
	case ModuleConf:
		overview := fmt.Sprintf("Error: 工作簿: %s, 表单: %s, 单元格[%s]中的值\"%s\"解析失败: %s", d.BookName, d.SheetName, d.DataCellPos, d.DataCell, d.Reason)
		details := fmt.Sprintf("\nDebugging: \n%s", d.DebugString())
		return overview + details
	default:
		return fmt.Sprintf("Error: %s", d.err)
	}
}

func (d *Desc) DebugString() (str string) {
	v := reflect.ValueOf(*d)
	typeOfS := v.Type()

	for i := 0; i < v.NumField(); i++ {
		name := string(typeOfS.Field(i).Name)
		firstRune := rune(name[0])
		if unicode.IsLetter(firstRune) && unicode.IsUpper(firstRune) {
			value := v.Field(i).Interface().(string)
			if value != unknown {
				str += fmt.Sprintf("\t%s: %v\n", name, value)
			}
		}
	}
	return str
}
