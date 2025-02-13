package protogen

import (
	"fmt"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
)

const (
	colNumber = "Number" // name of column "Number"
	colName   = "Name"   // name of column "Name"
	colAlias  = "Alias"  // name of column "Alias"
)

func isEnumTypeDefinitionBlockHeader(cols []string) bool {
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
		err = fmt.Errorf("name cell not found in enum type row")
	}
	return
}

func parseEnumTypeValues(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser) error {
	desc := &internalpb.EnumDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse enum type sheet: %s", sheet.Name)
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
