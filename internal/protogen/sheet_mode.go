package protogen

import (
	"fmt"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
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
	colAlias       = "Alias"  // name of column "Alias"
	colFieldPrefix = "Field"  // name of column field prefix "Field"
)

// extractSheetBlockTypeRow find the first none-empty column as "name", and then
// the subsequent column as "note" if provided.
func extractSheetBlockTypeRow(cols []string) (name, note string, err error) {
	if len(cols) == 0 || cols[0] == "" {
		err = fmt.Errorf("name cell not found in struct type name row")
		return
	}
	name = cols[0]
	if len(cols) >= 2 {
		note = cols[1]
	}
	return
}

func isEnumTypeBlockHeader(cols []string) bool {
	if len(cols) >= 3 {
		return cols[0] == colNumber && cols[1] == colName && cols[2] == colAlias
	}
	return false
}

func parseEnumType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, withPrefix bool) error {
	desc := &internalpb.EnumDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse enum type sheet (block): %s", sheet.Name)
	}
	prefix := strcase.ToScreamingSnake(ws.Name) + "_"
	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = *value.Number
		}
		name := value.Name
		if withPrefix && !strings.HasPrefix(name, prefix) {
			name = prefix + name
		}
		field := &internalpb.Field{
			Number: number,
			Name:   name,
			Alias:  value.Alias,
		}
		ws.Fields = append(ws.Fields, field)
	}
	return nil
}

func isStructTypeBlockHeader(cols []string) bool {
	if len(cols) >= 2 {
		return cols[0] == colName && cols[1] == colType
	}
	return false
}

func extractStructTypeInfo(sheet *book.Sheet, typeName, parentFilename string, parser book.SheetParser, gen *Generator) error {
	desc := &internalpb.StructDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse struct type sheet: %s", sheet.Name)
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

func parseStructType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator, debugBookName, debugSheetName string) error {
	desc := &internalpb.StructDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse struct type sheet (block): %s", sheet.Name)
	}
	bp := newTableParser("struct", "", "", gen)
	shHeader := &tableHeader{
		Header: &parseroptions.Header{
			NameRow: 1,
			TypeRow: 2,
		},
	}
	for _, field := range desc.Fields {
		shHeader.nameRowData = append(shHeader.nameRowData, field.Name)
		shHeader.typeRowData = append(shHeader.typeRowData, field.Type)
		shHeader.noteRowData = append(shHeader.noteRowData, "")
	}
	var parsed bool
	var err error
	for cursor := 0; cursor < len(shHeader.nameRowData); cursor++ {
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

func isUnionTypeBlockHeader(cols []string) bool {
	if len(cols) >= 3 {
		return cols[0] == colName && cols[1] == colAlias && strings.HasPrefix(cols[2], colFieldPrefix)
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
		return xerrors.Wrapf(err, "failed to parse union type sheet: %s", sheet.Name)
	}
	// add types nested in union type
	for _, value := range desc.Values {
		firstFieldOptionName := ""
		if len(value.Fields) != 0 {
			// name located at first line of cell
			firstFieldOptionName = book.ExtractFromCell(value.Fields[0], 1)
		}
		info := &xproto.TypeInfo{
			FullName:             protoreflect.FullName(gen.ProtoPackage + "." + typeName + "." + value.Name),
			ParentFilename:       parentFilename,
			Kind:                 types.MessageKind,
			FirstFieldOptionName: firstFieldOptionName,
		}
		gen.typeInfos.Put(info)
	}
	return nil
}

func parseUnionType(ws *internalpb.Worksheet, sheet *book.Sheet, parser book.SheetParser, gen *Generator, debugBookName, debugSheetName string) error {
	desc := &internalpb.UnionDescriptor{}
	if err := parser.Parse(desc, sheet); err != nil {
		return xerrors.Wrapf(err, "failed to parse union type sheet: %s", sheet.Name)
	}

	for i, value := range desc.Values {
		number := int32(i + 1)
		if value.Number != nil {
			number = *value.Number
		}
		name := value.Name
		var prop *tableaupb.FieldProp
		if desc := types.MatchScalar(name); desc != nil {
			name = desc.ScalarType
			prop, _ = desc.Prop.FieldProp()
		}
		field := &internalpb.Field{
			Number: number,
			Name:   name,
			Alias:  value.Alias,
		}
		if prop != nil {
			field.Options = &tableaupb.FieldOptions{
				Prop: prop,
			}
		}
		// create a book parser
		bp := newTableParser("union", "", "", gen)

		shHeader := &tableHeader{
			Header: &parseroptions.Header{
				NameRow:  1,
				TypeRow:  1,
				NameLine: 1,
				TypeLine: 2,
			},
			nameRowData: value.Fields,
			typeRowData: value.Fields,
			noteRowData: value.Fields,
		}
		var parsed bool
		var err error
		for cursor := 0; cursor < len(shHeader.nameRowData); cursor++ {
			fieldNumber := cursor + 1
			subField := &internalpb.Field{}
			cursor, parsed, err = bp.parseField(subField, shHeader, cursor, "", parseroptions.Mode(tableaupb.Mode_MODE_UNION_TYPE))
			if err != nil {
				return wrapDebugErr(err, debugBookName, debugSheetName, shHeader, cursor)
			}
			subField.Number = int32(fieldNumber)
			if parsed {
				field.Fields = append(field.Fields, subField)
			}
		}

		ws.Fields = append(ws.Fields, field)
	}
	return nil
}
