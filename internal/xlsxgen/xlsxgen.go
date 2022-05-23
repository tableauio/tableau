package xlsxgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

type Generator struct {
	ProtoPackage string // protobuf package name.
	InputDir     string // input dir of workbooks.
	OutputDir    string // output dir of generated protoconf files.
	Workbook     string // Workbook name
}

type Cell struct {
	Data string
}
type Row struct {
	Cells []Cell
	Index int
}
type MetaSheet struct {
	Worksheet string // worksheet name
	options.HeaderOption
	Transpose bool // interchange the rows and columns

	Rows   []Row
	colMap map[string]int // colName -> colNum

	defaultMap map[string]string // colName -> default value
}

func NewMetaSheet(worksheet string, header *options.HeaderOption, transpose bool) *MetaSheet {
	rows := make([]Row, header.Datarow)
	for i := range rows {
		rows[i].Index = i
	}
	return &MetaSheet{
		Worksheet:    worksheet,
		HeaderOption: *header,
		Transpose:    transpose,
		Rows:         rows,
		colMap:       make(map[string]int),
		defaultMap:   make(map[string]string),
	}
}

func (sheet *MetaSheet) NewRow() *Row {
	row := Row{
		Cells: make([]Cell, len(sheet.Rows[len(sheet.Rows)-1].Cells)),
		Index: len(sheet.Rows),
	}
	// Critical!!! copy common value from parent node
	copy(row.Cells, sheet.Rows[len(sheet.Rows)-1].Cells)
	sheet.Rows = append(sheet.Rows, row)
	return &row
}

func (sheet *MetaSheet) HasCol(name string) bool {
	_, existed := sheet.colMap[name]
	return existed
}

// Cell get the cell named `name` in the row `row`
// If not exists, insert empty type and note to the cell located in (row, col)
func (sheet *MetaSheet) Cell(row int, col int, name string) *Cell {
	if col, existed := sheet.colMap[name]; existed {
		if len(sheet.Rows[row].Cells) <= col {
			newCols := make([]Cell, col-len(sheet.Rows[row].Cells)+1)
			sheet.Rows[row].Cells = append(sheet.Rows[row].Cells, newCols...)
		}
		return &sheet.Rows[row].Cells[col]
	}
	// cannot access any of datarows when header not set
	if row+1 >= int(sheet.Datarow) {
		errStr := fmt.Sprintf("undefined column %s in row %d", name, row)
		panic(errStr)
	}
	// cannot insert new column to an isolated location
	curLen := len(sheet.Rows[sheet.Namerow-1].Cells)
	if col > curLen || col < 0 {
		errStr := fmt.Sprintf("invalid col %d, which should be in range [0,%d]", col, curLen)
		panic(errStr)
	}
	// only define new column and its type
	sheet.Rows[sheet.Namerow-1].Cells = append(sheet.Rows[sheet.Namerow-1].Cells, Cell{})
	// if there is typerow and noterow
	if sheet.Typerow < sheet.Datarow {
		sheet.Rows[sheet.Typerow-1].Cells = append(sheet.Rows[sheet.Typerow-1].Cells, Cell{})
	}
	if sheet.Noterow < sheet.Datarow {
		sheet.Rows[sheet.Noterow-1].Cells = append(sheet.Rows[sheet.Noterow-1].Cells, Cell{})
	}
	for i := curLen; i > col; i-- {
		colName := sheet.Rows[sheet.Namerow-1].Cells[i-1].Data
		sheet.colMap[colName] = i
		sheet.Rows[sheet.Namerow-1].Cells[i] = sheet.Rows[sheet.Namerow-1].Cells[i-1]
		// if there is typerow and noterow
		if sheet.Typerow < sheet.Datarow {
			sheet.Rows[sheet.Typerow-1].Cells[i] = sheet.Rows[sheet.Typerow-1].Cells[i-1]
		}
		if sheet.Noterow < sheet.Datarow {
			sheet.Rows[sheet.Noterow-1].Cells[i] = sheet.Rows[sheet.Noterow-1].Cells[i-1]
		}
	}
	sheet.colMap[name] = col
	sheet.Rows[sheet.Namerow-1].Cells[col].Data = name
	// if there is typerow and noterow
	if sheet.Typerow < sheet.Datarow {
		sheet.Rows[sheet.Typerow-1].Cells[col].Data = ""
	}
	if sheet.Noterow < sheet.Datarow {
		sheet.Rows[sheet.Noterow-1].Cells[col].Data = "Note"
	}
	return &sheet.Rows[row].Cells[sheet.colMap[name]]
}

func (sheet *MetaSheet) SetColType(col, typ string) {
	sheet.Cell(int(sheet.Typerow-1), len(sheet.Rows[sheet.Namerow-1].Cells), col).Data = typ
}

func (sheet *MetaSheet) GetColType(col string) string {
	return sheet.Cell(int(sheet.Typerow-1), len(sheet.Rows[sheet.Namerow-1].Cells), col).Data
}

func (sheet *MetaSheet) SetColNote(col, note string) {
	sheet.Cell(int(sheet.Noterow-1), len(sheet.Rows[sheet.Namerow-1].Cells), col).Data = note
}

func (sheet *MetaSheet) SetDefaultValue(col, defaultVal string) {
	// modification is not allowed
	if d, existed := sheet.defaultMap[col]; existed && d != "" {
		return
	}
	sheet.defaultMap[col] = defaultVal
}

func (sheet *MetaSheet) GetDefaultValue(col string) string {
	if _, existed := sheet.defaultMap[col]; !existed {
		return ""
	}
	return sheet.defaultMap[col]
}

func (sheet *MetaSheet) GetLastColName() string {
	row := sheet.Rows[sheet.Namerow-1].Cells
	if len(row) == 0 {
		return ""
	}
	return row[len(row)-1].Data
}

func (sheet *MetaSheet) ForEachCol(rowId int, f func(name string, cell *Cell) error) error {
	for name, i := range sheet.colMap {
		cell := sheet.Cell(rowId, len(sheet.Rows[sheet.Namerow-1].Cells), name)
		if err := f(name, cell); err != nil {
			return errors.Wrapf(err, "call user-defined failed when iterating col %s (%d, %d)", name, rowId, i)
		}
	}
	return nil
}

func (gen *Generator) Generate() {
	err := os.RemoveAll(gen.OutputDir)
	if err != nil {
		panic(err)
	}
	// create output dir
	err = os.MkdirAll(gen.OutputDir, 0700)
	if err != nil {
		panic(err)
	}

	protoPackage := protoreflect.FullName(gen.ProtoPackage)
	protoregistry.GlobalFiles.RangeFilesByPackage(protoPackage, func(fd protoreflect.FileDescriptor) bool {
		atom.Log.Debugf("filepath: %s\n", fd.Path())
		opts := fd.Options().(*descriptorpb.FileOptions)
		workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
		if workbook == nil {
			return true
		}

		atom.Log.Debugf("proto: %s => workbook: %s\n", fd.Path(), workbook)
		msgs := fd.Messages()
		for i := 0; i < msgs.Len(); i++ {
			md := msgs.Get(i)
			// atom.Log.Debugf("%s\n", md.FullName())
			opts := md.Options().(*descriptorpb.MessageOptions)
			worksheetOpt := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
			if worksheetOpt == nil {
				continue
			}
			atom.Log.Infof("generate: %s, message: %s@%s, worksheet: %s@%s", md.Name(), fd.Path(), md.Name(), workbook, worksheetOpt.Name)
			// export the protomsg message.
			_, workbook := TestParseFileOptions(md.ParentFile())
			fmt.Println("==================", workbook)
			msgName, worksheet, namerow, noterow, datarow, transpose := TestParseMessageOptions(md)
			metaSheet := NewMetaSheet(worksheet, &options.HeaderOption{
				Namerow: namerow,
				Noterow: noterow,
				Datarow: datarow,
			}, transpose)
			gen.TestParseFieldOptions(md, &metaSheet.Rows[metaSheet.Namerow-1].Cells, 0, "")
			fmt.Println("==================", msgName)
			if err := gen.ExportSheet(metaSheet); err != nil {
				panic(err)
			}
		}
		return true
	})
}

// ExportSheet export a worksheet.
func (gen *Generator) ExportSheet(metaSheet *MetaSheet) error {
	// create output dir
	if err := os.MkdirAll(gen.OutputDir, 0700); err != nil {
		return errors.WithMessagef(err, "failed to create output dir: %s", gen.OutputDir)
	}
	filename := filepath.Join(gen.OutputDir, gen.Workbook)
	var wb *excelize.File
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		wb = excelize.NewFile()
		t := time.Now()
		datetime := t.Format(time.RFC3339)
		err := wb.SetDocProps(&excelize.DocProperties{
			Category:       "category",
			ContentStatus:  "Draft",
			Created:        datetime,
			Creator:        "Tableau",
			Description:    "This file was created by Tableau",
			Identifier:     "xlsx",
			Keywords:       "Spreadsheet",
			LastModifiedBy: "Tableau",
			Modified:       datetime,
			Revision:       "0",
			Subject:        "Configuration",
			Title:          gen.Workbook,
			Language:       "en-US",
			Version:        "1.0.0",
		})
		if err != nil {
			panic(err)
		}
		// The newly created workbook will by default contain a worksheet named `Sheet1`.
		wb.SetSheetName("Sheet1", metaSheet.Worksheet)
		wb.SetDefaultFont("Courier")
	} else {
		wb, err = excelize.OpenFile(filename)
		if err != nil {
			panic(err)
		}
		wb.NewSheet(metaSheet.Worksheet)
	}

	{
		style, err := wb.NewStyle(&excelize.Style{
			Fill: excelize.Fill{
				Type:  "gradient",
				Color: []string{"#FFFFFF", "#E5E5E5"},
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "top",
				WrapText:   true,
			},
			Font: &excelize.Font{
				Bold:   true,
				Family: "Times New Roman",
				// Color:  "EEEEEEEE",
			},
			Border: []excelize.Border{
				{
					Type:  "top",
					Color: "EEEEEEEE",
					Style: 1,
				},
				{
					Type:  "bottom",
					Color: "EEEEEEEE",
					Style: 2,
				},
				{
					Type:  "left",
					Color: "EEEEEEEE",
					Style: 1,
				},
				{
					Type:  "right",
					Color: "EEEEEEEE",
					Style: 1,
				},
			},
		})
		if err != nil {
			panic(err)
		}
		wb.SetRowHeight(metaSheet.Worksheet, 1, 50)
		for j, row := range metaSheet.Rows {
			for i, cell := range row.Cells {
				hanWidth := 1 * float64(getHanCount(cell.Data))
				letterWidth := 1 * float64(getLetterCount(cell.Data))
				digitWidth := 1 * float64(getDigitCount(cell.Data))
				width := hanWidth + letterWidth + digitWidth + 4.0
				// width := 2 * float64(utf8.RuneCountInString(cell.Data))
				colname, err := excelize.ColumnNumberToName(i + 1)
				if err != nil {
					panic(err)
				}
				wb.SetColWidth(metaSheet.Worksheet, colname, colname, width)

				axis, err := excelize.CoordinatesToCellName(i+1, j+1)
				if err != nil {
					panic(err)
				}
				err = wb.SetCellValue(metaSheet.Worksheet, axis, cell.Data)
				if err != nil {
					panic(err)
				}

				// err = wb.AddComment(metaSheet.Worksheet, axis, `{"author":"Tableau: ","text":"\n`+cell.Data+`, \nthis is a comment."}`)
				// if err != nil {
				// 	panic(err)
				// }
				// set style
				wb.SetCellStyle(metaSheet.Worksheet, axis, axis, style)
				if err != nil {
					panic(err)
				}
				// atom.Log.Debugf("%s(%v) ", cell.Data, width)

				gen.setDataValidation(wb, metaSheet, i)
			}
		}
	}

	err := wb.SaveAs(filename)
	if err != nil {
		panic(err)
	}
	return nil
}

func (gen *Generator) setDataValidation(wb *excelize.File, metaSheet *MetaSheet, col int) {
	// test for validation
	// - min
	// - max
	// - droplist
	dataStartAxis, err := excelize.CoordinatesToCellName(col+1, 2)
	if err != nil {
		panic(err)
	}
	dataEndAxis, err := excelize.CoordinatesToCellName(col+1, 1000)
	if err != nil {
		panic(err)
	}

	if col == 0 {
		dataAxis, err := excelize.CoordinatesToCellName(col+1, 2)
		if err != nil {
			panic(err)
		}

		// unique key validation
		dv := excelize.NewDataValidation(true)
		dv.Sqref = dataStartAxis + ":" + dataEndAxis
		dv.Type = "custom"
		// dv.SetInput("Key", "Must be unique in this column")
		// NOTE(wenchyzhu): Five XML escape characters
		// "   &quot;
		// '   &apos;
		// <   &lt;
		// >   &gt;
		// &   &amp;
		//
		// `<formula1>=COUNTIF($A$2:$A$1000,A2)<2</formula1`
		//					||
		//					\/
		// `<formula1>=COUNTIF($A$2:$A$1000,A2)&lt;2</formula1`
		formula := fmt.Sprintf("=COUNTIF($A$2:$A$10000,%s)<2", dataAxis)
		dv.Formula1 = fmt.Sprintf("<formula1>%s</formula1>", escapeXML(formula))

		dv.SetError(excelize.DataValidationErrorStyleStop, "Error", "Key must be unique!")
		err = wb.AddDataValidation(metaSheet.Worksheet, dv)
		if err != nil {
			panic(err)
		}
	} else if col == 1 {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = dataStartAxis + ":" + dataEndAxis
		dv.SetDropList([]string{"1", "2", "3"})
		dv.SetInput("Options", "1: coin\n2: gem\n3: coupon")
		err := wb.AddDataValidation(metaSheet.Worksheet, dv)
		if err != nil {
			panic(err)
		}
	} else if col == 2 {
		dv := excelize.NewDataValidation(true)
		dv.Sqref = dataStartAxis + ":" + dataEndAxis
		dv.SetRange(10, 20, excelize.DataValidationTypeWhole, excelize.DataValidationOperatorBetween)
		dv.SetError(excelize.DataValidationErrorStyleStop, "error title", "error body")
		err := wb.AddDataValidation(metaSheet.Worksheet, dv)
		if err != nil {
			panic(err)
		}
	}
}

func escapeXML(in string) string {
	var b bytes.Buffer
	err := xml.EscapeText(&b, []byte(in))
	if err != nil {
		panic(err)
	}
	return b.String()
}

func getHanCount(s string) int {
	count := 0
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			count++
		}
	}
	return count
}

func getLetterCount(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			count++
		}
	}
	return count
}

func getDigitCount(s string) int {
	count := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			count++
		}
	}
	return count
}

// TestParseFileOptions is aimed to parse the options of a protobuf definition file.
func TestParseFileOptions(fd protoreflect.FileDescriptor) (string, *tableaupb.WorkbookOptions) {
	opts := fd.Options().(*descriptorpb.FileOptions)
	protofile := string(fd.FullName())
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	atom.Log.Debugf("file:%s, workbook:%s\n", fd.Path(), workbook)
	return protofile, workbook
}

// TestParseMessageOptions is aimed to parse the options of a protobuf message.
func TestParseMessageOptions(md protoreflect.MessageDescriptor) (string, string, int32, int32, int32, bool) {
	opts := md.Options().(*descriptorpb.MessageOptions)
	msgName := string(md.Name())
	worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)

	worksheetName := worksheet.Name
	namerow := worksheet.Namerow
	if worksheet.Namerow != 0 {
		namerow = 1 // default
	}
	noterow := worksheet.Noterow
	if noterow == 0 {
		noterow = 1 // default
	}
	datarow := worksheet.Datarow
	if datarow == 0 {
		datarow = 2 // default
	}
	transpose := worksheet.Transpose
	atom.Log.Debugf("message:%s, worksheetName:%s, namerow:%d, noterow:%d, datarow:%d, transpose:%v\n", msgName, worksheetName, namerow, noterow, datarow, transpose)
	return msgName, worksheetName, namerow, noterow, datarow, transpose
}

func getTabStr(depth int) string {
	tab := ""
	for i := 0; i < depth; i++ {
		tab += "\t"
	}
	return tab
}

// TestParseFieldOptions is aimed to parse the options of all the fields of a protobuf message.
func (gen *Generator) TestParseFieldOptions(md protoreflect.MessageDescriptor, row *[]Cell, depth int, prefix string) {
	opts := md.Options().(*descriptorpb.MessageOptions)
	worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	worksheetName := ""
	if worksheet != nil {
		worksheetName = worksheet.Name
	}
	pkg := md.ParentFile().Package()
	atom.Log.Debugf("%s// %s, '%s', %v, %v, %v\n", getTabStr(depth), md.FullName(), worksheetName, md.IsMapEntry(), prefix, pkg)
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if string(pkg) != gen.ProtoPackage && pkg != "google.protobuf" {
			atom.Log.Debugf("%s// no need to proces package: %v\n", getTabStr(depth), pkg)
			return
		}
		msgName := ""
		if fd.Kind() == protoreflect.MessageKind {
			msgName = string(fd.Message().FullName())
		}

		// default value
		name := strcase.ToCamel(string(fd.FullName().Name()))
		span := tableaupb.Span_SPAN_DEFAULT
		key := ""
		layout := tableaupb.Layout_LAYOUT_DEFAULT
		sep := ""
		subsep := ""

		opts := fd.Options().(*descriptorpb.FieldOptions)
		field := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
		if field != nil {
			name = field.Name
			span = field.Span
			key = field.Key
			layout = field.Layout
			sep = field.Sep
			subsep = field.Subsep
		} else {
			// default processing
			if fd.IsList() {
				// truncate suffix `List` (CamelCase) corresponding to `_list` (snake_case)
				name = strings.TrimSuffix(name, "List")
			} else if fd.IsMap() {
				// truncate suffix `Map` (CamelCase) corresponding to `_map` (snake_case)
				// name = strings.TrimSuffix(name, "Map")
				name = ""
				key = "Key"
			}
		}
		if sep == "" {
			sep = ","
		}
		if subsep == "" {
			subsep = ":"
		}
		atom.Log.Debugf("%s%s(%v) %s(%s) %s = %d [(name) = \"%s\", (type) = %s, (key) = \"%s\", (layout) = \"%s\", (sep) = \"%s\"];",
			getTabStr(depth), fd.Cardinality().String(), fd.IsMap(), fd.Kind().String(), msgName, fd.FullName().Name(), fd.Number(), prefix+name, span.String(), key, layout.String(), sep)
		atom.Log.Debugw("field metadata",
			"tabs", depth,
			"cardinality", fd.Cardinality().String(),
			"isMap", fd.IsMap(),
			"kind", fd.Kind().String(),
			"msgName", msgName,
			"fullName", fd.FullName(),
			"number", fd.Number(),
			"name", prefix+name,
			"span", span.String(),
			"key", key,
			"layout", layout.String(),
			"sep", sep,
		)
		if fd.IsMap() {
			valueFd := fd.MapValue()
			if layout == tableaupb.Layout_LAYOUT_INCELL {
				if valueFd.Kind() == protoreflect.MessageKind {
					panic("in-cell map do not support value as message type")
				}
				fmt.Println("cell(FIELD_TYPE_CELL_MAP): ", prefix+name)
				*row = append(*row, Cell{Data: prefix + name})
			} else {
				if valueFd.Kind() == protoreflect.MessageKind {
					if layout == tableaupb.Layout_LAYOUT_HORIZONTAL {
						size := 2
						for i := 1; i <= size; i++ {
							// fmt.Println("cell: ", prefix+name+strconv.Itoa(i)+key)
							gen.TestParseFieldOptions(valueFd.Message(), row, depth+1, prefix+name+strconv.Itoa(i))
						}
					} else {
						// fmt.Println("cell: ", prefix+name+strconv.Itoa(i)+key)
						gen.TestParseFieldOptions(valueFd.Message(), row, depth+1, prefix+name)
					}
				} else {
					// value is scalar type
					key := "Key"     // deafult key name
					value := "Value" // deafult value name
					fmt.Println("cell(scalar map key): ", prefix+name+key)
					fmt.Println("cell(scalar map value): ", prefix+name+value)

					*row = append(*row, Cell{Data: prefix + name + key})
					*row = append(*row, Cell{Data: prefix + name + value})
				}
			}
		} else if fd.IsList() {
			if fd.Kind() == protoreflect.MessageKind {
				if layout == tableaupb.Layout_LAYOUT_VERTICAL {
					gen.TestParseFieldOptions(fd.Message(), row, depth+1, prefix+name)
				}
			} else {
				if layout == tableaupb.Layout_LAYOUT_INCELL {
					fmt.Println("cell(FIELD_TYPE_CELL_LIST): ", prefix+name)
					*row = append(*row, Cell{Data: prefix + name})
				} else {
					panic(fmt.Sprintf("unknown list layout: %v\n", layout))
				}
			}
		} else {
			if fd.Kind() == protoreflect.MessageKind {
				if span == tableaupb.Span_SPAN_INNER_CELL {
					fmt.Println("cell(FIELD_TYPE_CELL_MESSAGE): ", prefix+name)
					*row = append(*row, Cell{Data: prefix + name})
				} else {
					subMsgName := string(fd.Message().FullName())
					_, found := xproto.WellKnownMessages[subMsgName]
					if found {
						fmt.Println("cell(special message): ", prefix+name)
						*row = append(*row, Cell{Data: prefix + name})
					} else {
						pkgName := fd.Message().ParentFile().Package()
						if string(pkgName) != gen.ProtoPackage {
							panic(fmt.Sprintf("unknown message %v in package %v", subMsgName, pkgName))
						}
						gen.TestParseFieldOptions(fd.Message(), row, depth+1, prefix+name)
					}
				}
			} else {
				fmt.Println("cell: ", prefix+name)
				*row = append(*row, Cell{Data: prefix + name})
			}
		}
	}
}
