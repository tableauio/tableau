package protogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
)

const (
	tableauProtoPath   = "tableau/protobuf/tableau.proto"
	timestampProtoPath = "google/protobuf/timestamp.proto"
	durationProtoPath  = "google/protobuf/duration.proto"
)

type bookParser struct {
	gen *Generator

	wb       *tableaupb.Workbook
	withNote bool
}

func newBookParser(bookName, relSlashPath string, gen *Generator) *bookParser {
	// atom.Log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	filename := strcase.ToSnake(bookName)
	if gen.OutputOpt.ProtoFilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), bookName)
		snakePath := strcase.ToSnake(bookPath)
		filename = strings.ReplaceAll(snakePath, "/", "__")
	}
	bp := &bookParser{
		gen: gen,
		wb: &tableaupb.Workbook{
			Options: &tableaupb.WorkbookOptions{
				// NOTE(wenchyzhu): all OS platforms use path slash separator `/`
				// see: https://stackoverflow.com/questions/9371031/how-do-i-create-crossplatform-file-paths-in-go
				Name: relSlashPath,
			},
			Worksheets: []*tableaupb.Worksheet{},
			Name:       filename,
			Imports:    make(map[string]int32),
		},
		withNote: false,
	}

	// custom imports
	for _, path := range gen.InputOpt.ImportFiles {
		bp.wb.Imports[path] = 1
	}
	return bp
}

func (p *bookParser) parseField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, ok bool) {
	nameCell := header.getNameCell(cursor)
	typeCell := header.getTypeCell(cursor)
	noteCell := header.getNoteCell(cursor)
	// atom.Log.Debugf("column: %d, name: %s, type: %s", cursor, nameCell, typeCell)
	if nameCell == "" || typeCell == "" {
		atom.Log.Debugf("no need to parse column %d, as name(%s) or type(%s) is empty", cursor, nameCell, typeCell)
		return cursor, false
	}

	opts := parseroptions.ParseOptions(options...)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	if types.IsMap(typeCell) {
		cursor = p.parseMapField(field, header, cursor, prefix, options...)
	} else if types.IsList(typeCell) {
		cursor = p.parseListField(field, header, cursor, prefix, options...)
	} else if types.IsStruct(typeCell) {
		cursor = p.parseStructField(field, header, cursor, prefix, options...)
	} else {
		// enum or scalar types
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		*field = *p.parseScalarField(trimmedNameCell, typeCell, noteCell)
	}

	return cursor, true
}

func (p *bookParser) parseSubField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) int {
	subField := &tableaupb.Field{}
	cursor, ok := p.parseField(subField, header, cursor, prefix, options...)
	if ok {
		field.Fields = append(field.Fields, subField)
		// if field.Options.Layout == tableaupb.Layout_LAYOUT_HORIZONTAL {
		// 	field.Options.ListMaxLen /= int32(len(field.Fields))
		// }
	}
	return cursor
}

func (p *bookParser) parseMapField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) int {
	// refer: https://developers.google.com/protocol-buffers/docs/proto3#maps
	//
	//	map<key_type, value_type> map_field = N;
	//
	// where the key_type can be any integral or string type (so, any scalar type
	// except for floating point types and bytes). Note that enum is not a valid
	// key_type. The value_type can be any type except another map.
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getNameCell(cursor)
	typeCell := header.getTypeCell(cursor)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	noteCell := header.getNoteCell(cursor)

	// map syntax pattern
	matches := types.MatchMap(typeCell)
	keyType := strings.TrimSpace(matches[1])
	valueType := strings.TrimSpace(matches[2])
	rawPropText := strings.TrimSpace(matches[3])

	parsedKeyType := keyType
	if types.IsEnum(keyType) {
		// NOTE: support enum as map key, convert key type as `int32`.
		parsedKeyType = "int32"
	}
	parsedValueType, valueTypeDefined := p.parseType(valueType)
	mapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, parsedValueType)

	isScalarValue := types.IsScalarType(parsedValueType)
	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

	// preprocess: analyze the correct layout of map.
	layout := tableaupb.Layout_LAYOUT_VERTICAL // set default layout as vertical.
	firstElemIndex := -1
	if index1 := strings.Index(trimmedNameCell, "1"); index1 > 0 {
		firstElemIndex = index1
		layout = tableaupb.Layout_LAYOUT_HORIZONTAL
		nextCursor := cursor + 1
		if nextCursor < len(header.namerow) {
			// Header:
			//
			// TaskParamMap1		TaskParamMap2		TaskParamMap3
			// map<int32, int32>	map<int32, int32>	map<int32, int32>

			// check next cursor
			nextNameCell := header.getNameCell(nextCursor)
			trimmedNextNameCell := strings.TrimPrefix(nextNameCell, prefix)
			if index2 := strings.Index(trimmedNextNameCell, "2"); index2 > 0 {
				nextTypeCell := header.getTypeCell(nextCursor)
				if types.IsMap(nextTypeCell) {
					// The next type cell is also a map type declaration.
					if isScalarValue {
						layout = tableaupb.Layout_LAYOUT_INCELL // incell map
					}
				}
			} else {
				// only one map item, treat it as incell map
				if isScalarValue {
					layout = tableaupb.Layout_LAYOUT_INCELL // incell map
				}
			}
		}
	} else {
		if isScalarValue {
			layout = tableaupb.Layout_LAYOUT_INCELL // incell map
		}
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		if opts.Nested {
			prefix += parsedValueType // add prefix with value type
		}
		field.Name = strcase.ToSnake(parsedValueType) + "_map"
		field.Type = mapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDefined
		field.MapEntry = &tableaupb.MapEntry{
			KeyType:   parsedKeyType,
			ValueType: parsedValueType,
		}

		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		field.Options = &tableaupb.FieldOptions{
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}
		if opts.Nested {
			field.Options.Name = parsedValueType
		}
		field.Fields = append(field.Fields, p.parseScalarField(trimmedNameCell, keyType, noteCell))
		for cursor++; cursor < len(header.namerow); cursor++ {
			if opts.Nested {
				nameCell := header.getNameCell(cursor)
				if !strings.HasPrefix(nameCell, prefix) {
					cursor--
					return cursor
				}
			}
			cursor = p.parseSubField(field, header, cursor, prefix, options...)
		}
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		// horizontal list: continuous N columns belong to this list after this cursor.
		mapName := trimmedNameCell[:firstElemIndex]
		prefix += mapName

		field.Name = strcase.ToSnake(parsedValueType) + "_map"
		field.Type = mapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDefined
		field.MapEntry = &tableaupb.MapEntry{
			KeyType:   parsedKeyType,
			ValueType: parsedValueType,
		}

		trimmedNameCell := strings.TrimPrefix(nameCell, prefix+"1")
		field.Options = &tableaupb.FieldOptions{
			Name:   mapName,
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}
		if opts.Nested {
			field.Options.Name = parsedValueType
		}

		name := strings.TrimPrefix(nameCell, prefix+"1")
		field.Fields = append(field.Fields, p.parseScalarField(name, keyType, noteCell))

		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getNameCell(cursor)
			if types.BelongToFirstElement(nameCell, prefix) {
				cursor = p.parseSubField(field, header, cursor, prefix+"1", options...)
			} else if strings.HasPrefix(nameCell, prefix) {
				continue
			} else {
				cursor--
				return cursor
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		// incell map
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		field.Name = strcase.ToSnake(trimmedNameCell)
		field.Type = mapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDefined
		field.Options = &tableaupb.FieldOptions{
			Name:   trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}
	case tableaupb.Layout_LAYOUT_DEFAULT:
		atom.Log.Panicf("should not reach default layout: %v", layout)
	}

	return cursor
}

func (p *bookParser) parseListField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) int {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getNameCell(cursor)
	typeCell := header.getTypeCell(cursor)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	noteCell := header.getNoteCell(cursor)

	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

	// list syntax pattern
	matches := types.MatchList(typeCell)
	originalElemType := strings.TrimSpace(matches[1])
	colType := strings.TrimSpace(matches[2])
	rawPropText := strings.TrimSpace(matches[3])

	listElemSpanInnerCell, isScalarElement := false, false
	elemType := originalElemType
	if elemType == "" {
		listElemSpanInnerCell = true
		isScalarElement = true
		elemType = colType
		if matches := types.MatchStruct(colType); len(matches) > 0 {
			elemType = matches[2]
			isScalarElement = false
		}
	}

	// preprocess: analyze the correct layout of list.
	layout := tableaupb.Layout_LAYOUT_VERTICAL // set default layout as vertical.
	firstElemIndex := -1
	if index1 := strings.Index(trimmedNameCell, "1"); index1 > 0 {
		firstElemIndex = index1
		layout = tableaupb.Layout_LAYOUT_HORIZONTAL
		nextCursor := cursor + 1
		if nextCursor < len(header.namerow) {
			// Header:
			//
			// TaskParamList1	TaskParamList2	TaskParamList3
			// []int32			[]int32			[]int32

			// check next cursor
			nextNameCell := header.getNameCell(nextCursor)
			trimmedNextNameCell := strings.TrimPrefix(nextNameCell, prefix)
			if index2 := strings.Index(trimmedNextNameCell, "2"); index2 > 0 {
				nextTypeCell := header.getTypeCell(nextCursor)
				if types.IsList(nextTypeCell) {
					// The next type cell is also a list type declaration.
					if isScalarElement {
						layout = tableaupb.Layout_LAYOUT_INCELL // incell list
					}
				}
			} else {
				// only one list item, treat it as incell list
				if isScalarElement {
					layout = tableaupb.Layout_LAYOUT_INCELL // incell list
				}
			}
		}
	} else {
		if isScalarElement {
			layout = tableaupb.Layout_LAYOUT_INCELL // incell list
		}
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		// vertical list: all columns belong to this list after this cursor.
		scalarField := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		proto.Merge(field, scalarField)
		field.Card = "repeated"
		field.Name = strcase.ToSnake(elemType) + "_list"
		field.Options.Name = "" // Default, name is empty for vertical list
		field.Options.Layout = layout

		if opts.Nested {
			prefix += field.Type // add prefix with value type
			field.Options.Name = field.Type
		}
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

		if matches := types.MatchKeyedList(typeCell); matches != nil {
			// set column type and key if this is a keyed list.
			colType = strings.TrimSpace(matches[2])
			field.Options.Key = trimmedNameCell
		}
		// Parse first field
		colTypeWithProp := colType + rawPropText
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, colTypeWithProp))
		if listElemSpanInnerCell {
			// inner cell element
			tempField := &tableaupb.Field{}
			_, ok := p.parseField(tempField, header, cursor, prefix, firstFieldOptions...)
			if ok {
				field.Fields = tempField.Fields
				field.Predefined = tempField.Predefined
				field.Options.Span = tempField.Options.Span
				field.Options.Name = tempField.Options.Name
			} else {
				atom.Log.Panic("failed to parse list inner cell element, name cell: %s, type cell: %s", nameCell, typeCell)
			}
		} else {
			cursor = p.parseSubField(field, header, cursor, prefix, firstFieldOptions...)
		}
		// Parse other fields
		for cursor++; cursor < len(header.namerow); cursor++ {
			if opts.Nested {
				nameCell := header.getNameCell(cursor)
				if !strings.HasPrefix(nameCell, prefix) {
					cursor--
					return cursor
				}
			}
			cursor = p.parseSubField(field, header, cursor, prefix, options...)
		}

	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		// horizontal list: continuous N columns belong to this list after this cursor.
		listName := trimmedNameCell[:firstElemIndex]
		prefix += listName

		scalarField := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		proto.Merge(field, scalarField)
		field.Card = "repeated"
		field.Name = strcase.ToSnake(listName) + "_list"
		field.Options.Name = listName // name is empty for vertical list
		field.Options.Layout = layout

		// Parse first field
		colTypeWithProp := colType + rawPropText
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, colTypeWithProp))
		if listElemSpanInnerCell {
			// inner cell element
			tempField := &tableaupb.Field{}
			_, ok := p.parseField(tempField, header, cursor, prefix+"1", firstFieldOptions...)
			if ok {
				field.Fields = tempField.Fields
				field.Predefined = tempField.Predefined
				field.Options.Span = tempField.Options.Span
			} else {
				atom.Log.Panic("failed to parse list inner cell element, name cell: %s, type cell: %s", nameCell, typeCell)
			}
		} else {
			// cross cell element
			cursor = p.parseSubField(field, header, cursor, prefix+"1", firstFieldOptions...)
		}
		// Parse other fields or skip cotinuous N columns of the same element type.
		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getNameCell(cursor)
			if !listElemSpanInnerCell && types.BelongToFirstElement(nameCell, prefix) {
				cursor = p.parseSubField(field, header, cursor, prefix+"1", options...)
			} else if strings.HasPrefix(nameCell, prefix) {
				continue
			} else {
				cursor--
				return cursor
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		// incell list
		scalarField := p.parseScalarField(trimmedNameCell, elemType+rawPropText, noteCell)
		proto.Merge(field, scalarField)
		field.Card = "repeated"
		field.Options.Layout = layout
	case tableaupb.Layout_LAYOUT_DEFAULT:
		atom.Log.Panicf("should not reach default layout: %s", layout)
	}
	return cursor
}

func (p *bookParser) parseStructField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) int {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getNameCell(cursor)
	typeCell := header.getTypeCell(cursor)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	noteCell := header.getNoteCell(cursor)

	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

	// struct syntax pattern
	matches := types.MatchStruct(typeCell)
	elemType := strings.TrimSpace(matches[1])
	colType := strings.TrimSpace(matches[2])
	rawPropText := strings.TrimSpace(matches[3])

	if fieldPairs := ParseIncellStruct(elemType); fieldPairs != nil {
		scalarField := p.parseScalarField(trimmedNameCell, colType, noteCell)
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL

		for i := 0; i < len(fieldPairs); i += 2 {
			fieldType := fieldPairs[i]
			fieldName := fieldPairs[i+1]
			field.Fields = append(field.Fields, p.parseScalarField(fieldName, fieldType, ""))
		}
	} else {
		// cross cell struct
		// NOTE(wenchy): treated as nested named struct
		scalarField := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		proto.Merge(field, scalarField)

		structName := field.Type // default: struct name is same as the type name
		if field.Predefined {
			// Find predefined type's first field's tableau name
			// structName = field.Type
			fullMsgName := p.gen.ProtoPackage + "." + field.Type
			for _, fileDesc := range p.gen.fileDescs {
				md := fileDesc.FindMessage(fullMsgName)
				if md == nil {
					continue
				}
				fds := md.GetFields()
				if len(fds) == 0 {
					break
				}
				fd := fds[0]
				// fist field
				opts := fd.GetFieldOptions()
				fieldOpts := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
				if fieldOpts != nil {
					if index := strings.Index(trimmedNameCell, fieldOpts.Name); index != -1 {
						structName = trimmedNameCell[:index]
					}
				}
				break
			}
		}

		field.Name = strcase.ToSnake(structName)
		field.Options = &tableaupb.FieldOptions{
			Name: structName,
		}
		prefix += structName
		colTypeWithProp := colType + rawPropText
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, colTypeWithProp))
		cursor = p.parseSubField(field, header, cursor, prefix, firstFieldOptions...)
		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getNameCell(cursor)
			if !strings.HasPrefix(nameCell, prefix) {
				cursor--
				return cursor
			}
			cursor = p.parseSubField(field, header, cursor, prefix, options...)
		}
	}

	return cursor
}

func (p *bookParser) parseScalarField(name, typ, note string) *tableaupb.Field {
	rawPropText := ""
	// enum syntax pattern
	if matches := types.MatchEnum(typ); len(matches) > 0 {
		enumType := strings.TrimSpace(matches[1])
		typ = enumType
		rawPropText = matches[2]
	} else {
		// scalar syntax pattern
		splits := strings.SplitN(typ, "|", 2)
		typ = splits[0]
		if len(splits) > 1 {
			rawPropText = splits[1]
		}
	}
	typ, predefined := p.parseType(typ)

	return &tableaupb.Field{
		Name:        strcase.ToSnake(name),
		Type:        typ,
		Predefined: predefined,
		Options: &tableaupb.FieldOptions{
			Name: name,
			Note: p.genNote(note),
			Prop: types.ParseProp(rawPropText),
		},
	}
}

func (p *bookParser) genNote(note string) string {
	if p.withNote {
		return note
	}
	return ""
}

func (p *bookParser) parseType(typ string) (string, bool) {
	if strings.HasPrefix(typ, ".") {
		// This messge type is defined in imported proto
		typ = strings.TrimPrefix(typ, ".")
		return typ, true
	}
	switch typ {
	case "datetime", "date":
		typ = "google.protobuf.Timestamp"
	case "duration", "time":
		typ = "google.protobuf.Duration"
	default:
		return typ, false
	}
	return typ, false
}

func ParseIncellStruct(elemType string) []string {
	fields := strings.Split(elemType, ",")
	if len(fields) == 1 && len(strings.Split(fields[0], " ")) == 1 {
		// cross cell struct
		return nil
	}

	fieldPairs := make([]string, 0)
	for _, pair := range strings.Split(elemType, ",") {
		kv := strings.Split(strings.TrimSpace(pair), " ")
		if len(kv) != 2 {
			atom.Log.Panicf("illegal type-variable pair: %v in incell struct: %s", pair, elemType)
		}
		fieldPairs = append(fieldPairs, kv...)
	}
	return fieldPairs
}
