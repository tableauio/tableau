package protogen

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	colNumber      = "Number" // name of column "Number"
	colName        = "Name"   // name of column "Name"
	colType        = "Type"   // name of column "Type"
	colNote        = "Note"   // name of column "Note"
	colAlias       = "Alias"  // name of column "Alias"
	colFieldPrefix = "Field"  // name of column field prefix "Field"
)

// extractTableBlockTypeRow find the first non-empty column as
// the enum/struct/union type "name", and then the subsequent column
// as "note" if provided.
func extractTableBlockTypeRow(cols []string) (name, note string, err error) {
	if len(cols) == 0 || cols[0] == "" {
		err = fmt.Errorf("enum/struct/union name cell not found in table block type row")
		return
	}
	name = cols[0]
	if len(cols) >= 2 {
		note = cols[1]
	}
	return
}

func isEnumTypeBlockHeader(cols []string) bool {
	var containsName, containsAlias bool
	for _, col := range cols {
		switch col {
		case colName:
			containsName = true
		case colAlias:
			containsAlias = true
		}
		if containsName && containsAlias {
			return true
		}
	}
	return false
}

func parseEnumType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator) error {
	desc := &internalpb.EnumDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return err
	}
	prefix := strcase.FromContext(gen.ctx).ToScreamingSnake(ws.Name) + "_"
	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = value.GetNumber()
		}
		name := value.Name
		if gen.OutputOpt.EnumValueWithPrefix && !strings.HasPrefix(name, prefix) {
			name = prefix + name
		}
		field := &internalpb.Field{
			Number: number,
			Name:   strings.TrimSpace(name),
			Alias:  strings.TrimSpace(value.Alias),
		}
		ws.Fields = append(ws.Fields, field)
	}
	return nil
}

func isStructTypeBlockHeader(cols []string) bool {
	var containsName, containsType bool
	for _, col := range cols {
		switch col {
		case colName:
			containsName = true
		case colType:
			containsType = true
		}
		if containsName && containsType {
			return true
		}
	}
	return false
}

func extractStructTypeInfo(sheet *book.Sheet, typeName, parentFilename string, parser book.SheetParser, gen *Generator) error {
	desc := &internalpb.StructDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return err
	}
	firstFieldOptionName := ""
	if len(desc.Fields) != 0 {
		firstFieldOptionName = desc.Fields[0].Name
	}
	// add type info
	info := &xproto.TypeInfo{
		FullName:             protoreflect.FullName(gen.ProtoPackage + "." + typeName),
		ParentFilename:       parentFilename,
		Kind:                 types.MessageKind,
		FirstFieldOptionName: firstFieldOptionName,
	}
	gen.typeInfos.Put(info)
	return nil
}

// basePositioner holds common fields for positioners that need to resolve
// column names to physical column indices. The colMap is lazily initialized
// on the first call to colIndex via sync.Once.
type basePositioner struct {
	tabler book.Tabler
	colMap map[string]int // column name -> 0-based physical column index
	once   sync.Once
}

// colIndex returns the 0-based physical column index for the given column name.
// On the first call, it scans the header row to build the colMap.
// Returns (index, true) if found, or (0, false) if the column does not exist.
func (b *basePositioner) colIndex(name string) (int, bool) {
	b.once.Do(func() {
		headerRow := b.tabler.GetRow(b.tabler.BeginRow())
		b.colMap = make(map[string]int, len(headerRow))
		for i, cell := range headerRow {
			if cell != "" {
				b.colMap[cell] = i
			}
		}
	})
	idx, ok := b.colMap[name]
	return idx, ok
}

// verticalRowNames maps virtual header row indices to column names.
// Index 0 corresponds to NameRow, 1 to TypeRow, 2 to NoteRow.
var verticalRowNames = []string{colName, colType, colNote}

// verticalPositioner correctly maps positions for LAYOUT_VERTICAL sheets
// (e.g., struct type sheets) where cursor iterates over data rows
// instead of columns.
type verticalPositioner struct {
	basePositioner
	dataRow int // 0-based data start row in tabler's coordinate
}

func (p *verticalPositioner) Position(row, col int) string {
	// row: virtual header row index (e.g., 0 for Name, 1 for Type, 2 for Note)
	// col: cursor (field index), maps to actual data row
	if row < 0 || row >= len(verticalRowNames) {
		return ""
	}
	colIdx, ok := p.colIndex(verticalRowNames[row])
	if !ok {
		// The requested column (e.g., Note) does not exist in this table.
		return ""
	}
	return p.tabler.Position(p.dataRow+col, colIdx)
}

func parseStructType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator, debugBookName, debugSheetName string) error {
	desc := &internalpb.StructDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return err
	}
	bp := newTableParser("struct", "", "", gen)
	t := sheet.Tabler()
	shHeader := &tableHeader{
		Header: &tableparser.Header{
			NameRow: 1,
			TypeRow: 2,
			NoteRow: 3,
		},
		Positioner: &verticalPositioner{
			basePositioner: basePositioner{tabler: t},
			dataRow:        t.BeginRow() + 1, // StructDescriptor's datarow is 2 (1-based)
		},
	}
	for _, field := range desc.Fields {
		shHeader.nameRowData = append(shHeader.nameRowData, strings.TrimSpace(field.Name))
		shHeader.typeRowData = append(shHeader.typeRowData, field.Type)
		shHeader.noteRowData = append(shHeader.noteRowData, strings.TrimSpace(field.Note))
	}
	var parsed bool
	var err error
	for cursor := 0; cursor < len(shHeader.nameRowData); cursor++ {
		subField := &internalpb.Field{}
		cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "", "")
		if err != nil {
			return wrapDebugErr(err, debugBookName, debugSheetName, shHeader, cursor)
		}
		if parsed {
			ws.Fields = append(ws.Fields, subField)
		}
	}
	return nil
}

func isUnionTypeBlockHeader(cols []string) bool {
	var containsName, containsAlias, containsField1 bool
	for _, col := range cols {
		switch col {
		case colName:
			containsName = true
		case colAlias:
			containsAlias = true
		case colFieldPrefix + "1":
			containsField1 = true
		}
		if containsName && containsAlias && containsField1 {
			return true
		}
	}
	return false
}

func extractUnionTypeInfo(sheet *book.Sheet, typeName, parentFilename string, parser book.SheetParser, gen *Generator) error {
	// add union self type info
	info := &xproto.TypeInfo{
		FullName:             protoreflect.FullName(gen.ProtoPackage + "." + typeName),
		ParentFilename:       parentFilename,
		Kind:                 types.MessageKind,
		FirstFieldOptionName: "Type", // NOTE: union's first field name is special!
	}
	gen.typeInfos.Put(info)

	// add union enum type info
	enumInfo := &xproto.TypeInfo{
		FullName:       protoreflect.FullName(gen.ProtoPackage + "." + typeName + "." + "Type"),
		ParentFilename: parentFilename,
		Kind:           types.EnumKind,
	}
	gen.typeInfos.Put(enumInfo)

	desc := &internalpb.UnionDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return err
	}
	// add types nested in union type
	for _, value := range desc.Values {
		firstFieldOptionName := ""
		if len(value.Fields) != 0 {
			// name located at first line of cell
			firstFieldOptionName = book.ExtractFromCell(value.Fields[0], 1)
		}
		info := &xproto.TypeInfo{
			FullName:             protoreflect.FullName(gen.ProtoPackage + "." + typeName + "." + strings.TrimSpace(value.Name)),
			ParentFilename:       parentFilename,
			Kind:                 types.MessageKind,
			FirstFieldOptionName: firstFieldOptionName,
		}
		gen.typeInfos.Put(info)
	}
	return nil
}

// unionFieldPositioner correctly maps positions for union type sheets where
// cursor iterates over field columns within a specific union value row.
type unionFieldPositioner struct {
	basePositioner
	valueRow int // 0-based row of current union value in tabler's coordinate
}

func (p *unionFieldPositioner) Position(row, col int) string {
	// row param is unused since name/type/note are all in the same cell (different lines).
	// col is the cursor (field index within this value), maps to "Field1", "Field2", ... via colIndex.
	name := colFieldPrefix + strconv.Itoa(col+1) // 0-based col -> "Field1", "Field2", ...
	if colIdx, ok := p.colIndex(name); ok {
		return p.tabler.Position(p.valueRow, colIdx)
	}
	return ""
}

func parseUnionType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator, debugBookName, debugSheetName string) error {
	desc := &internalpb.UnionDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return err
	}

	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = *value.Number
		}
		field := &internalpb.Field{
			Number: number,
			Name:   strings.TrimSpace(value.Name),
			Alias:  strings.TrimSpace(value.Alias),
		}
		if typ := strings.TrimSpace(value.Type); typ != "" {
			typeDesc, err := parseTypeDescriptor(gen.typeInfos, typ)
			if err != nil {
				return xerrors.Wrapf(err, "failed to parse union type %s of sheet: %s", typ, sheet.Name)
			}
			field.Type = typeDesc.Name
			field.FullType = typeDesc.FullName
		}

		// create a book parser
		bp := newTableParser("union", "", "", gen)
		t := sheet.Tabler()
		shHeader := &tableHeader{
			Header: &tableparser.Header{
				NameRow:  1,
				TypeRow:  1,
				NoteRow:  1,
				NameLine: 1,
				TypeLine: 2,
				NoteLine: 3,
			},
			Positioner: &unionFieldPositioner{
				basePositioner: basePositioner{tabler: t},
				valueRow:       t.BeginRow() + 1 + i, // datarow=2 (1-based), i is the value index
			},
			nameRowData: value.Fields,
			typeRowData: value.Fields,
			noteRowData: value.Fields,
		}
		var parsed bool
		var err error
		for cursor := 0; cursor < len(shHeader.nameRowData); cursor++ {
			subField := &internalpb.Field{}
			cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "", "", tableparser.Mode(tableaupb.Mode_MODE_UNION_TYPE))
			if err != nil {
				return wrapDebugErr(err, debugBookName, debugSheetName, shHeader, cursor)
			}
			if parsed {
				field.Fields = append(field.Fields, subField)
			}
		}

		ws.Fields = append(ws.Fields, field)
	}
	return nil
}
