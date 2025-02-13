package protogen

import (
	"fmt"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
)

const (
	colNumber = "Number" // name of column "Number"
	colName   = "Name"   // name of column "Name"
	colType   = "Type"   // name of column "Type"
	colAlias  = "Alias"  // name of column "Alias"
)

func isEnumTypeBlockHeader(cols []string) bool {
	if len(cols) < 3 {
		return false
	}
	return cols[0] == colNumber && cols[1] == colName && cols[2] == colAlias
}

// extractEnumTypeRow find the first none-empty column as "name", and then
// the two subsequent columns as "alias" and "note" if provided.
func extractEnumTypeRow(cols []string) (name, alias, note string, err error) {
	for i, cell := range cols {
		if cell != "" {
			name = cell
			if i+1 < len(cols) {
				alias = cols[i+1]
			}
			if i+2 < len(cols) {
				note = cols[i+2]
			}
			break
		}
	}
	if name == "" {
		err = fmt.Errorf("name cell not found in enum type name row")
	}
	return
}

func parseEnumTypeValues(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser) error {
	desc := &internalpb.EnumDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse enum type sheet (block): %s", sheet.Name)
	}
	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = *value.Number
		}
		field := &internalpb.Field{
			Number: number,
			Name:   value.Name,
			Alias:  value.Alias,
		}
		ws.Fields = append(ws.Fields, field)
	}
	return nil
}

func isStructTypeBlockHeader(cols []string) bool {
	if len(cols) < 2 {
		return false
	}
	return cols[0] == colName && cols[1] == colType
}

// extractStructTypeRow find the first none-empty column as "name", and then
// the two subsequent columns as "alias" and "note" if provided.
func extractStructTypeRow(cols []string) (name, alias, note string, err error) {
	if len(cols) == 0 || cols[0] == "" {
		err = fmt.Errorf("name cell not found in struct type name row")
		return
	}
	name = cols[0]
	if len(cols) >= 2 {
		alias = cols[1]
	}
	if len(cols) >= 3 {
		note = cols[2]
	}
	return
}

func parseStructTypeValues(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator, debugBookName, debugSheetName string) error {
	desc := &internalpb.StructDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse struct type sheet (block): %s", sheet.Name)
	}
	bp := newBookParser("struct", "", "", gen)
	shHeader := &tableHeader{
		meta: &tableaupb.WorksheetOptions{
			Namerow: 1,
			Typerow: 2,
		},
		validNames: map[string]int{},
	}
	for _, field := range desc.Fields {
		shHeader.namerow = append(shHeader.namerow, field.Name)
		shHeader.typerow = append(shHeader.typerow, field.Type)
		shHeader.noterow = append(shHeader.noterow, "")
	}
	var parsed bool
	var err error
	for cursor := 0; cursor < len(shHeader.namerow); cursor++ {
		subField := &internalpb.Field{}
		cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "")
		if err != nil {
			return wrapDebugErr(err, debugBookName, debugSheetName, shHeader, cursor)
		}
		if parsed {
			ws.Fields = append(ws.Fields, subField)
		}
	}
	return nil
}
