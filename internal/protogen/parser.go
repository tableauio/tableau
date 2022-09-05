package protogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

const (
	tableauProtoPath   = "tableau/protobuf/tableau.proto"
	timestampProtoPath = "google/protobuf/timestamp.proto"
	durationProtoPath  = "google/protobuf/duration.proto"
)

const (
	mapVarSuffix  = "_map"  // map variable name suffix
	listVarSuffix = "_list" // list variable name suffix
)

type bookParser struct {
	gen *Generator

	wb       *tableaupb.Workbook
	withNote bool
}

func newBookParser(bookName, relSlashPath string, gen *Generator) *bookParser {
	// log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	filename := strcase.ToSnake(bookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
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

	// custom imported proto files
	for _, path := range gen.InputOpt.ImportedProtoFiles {
		bp.wb.Imports[path] = 1
	}
	return bp
}

func (p *bookParser) parseField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, parsed bool, err error) {
	nameCell := header.getValidNameCell(&cursor)
	typeCell := header.getTypeCell(cursor)
	noteCell := header.getNoteCell(cursor)
	// log.Debugf("column: %d, name: %s, type: %s", cursor, nameCell, typeCell)
	if nameCell == "" || typeCell == "" {
		log.Debugf("no need to parse column %d, as name(%s) or type(%s) is empty", cursor, nameCell, typeCell)
		return cursor, false, nil
	}

	opts := parseroptions.ParseOptions(options...)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	if types.IsMap(typeCell) {
		cursor, err = p.parseMapField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "map", xerrors.KeyTypeCell, typeCell)
		}
	} else if types.IsList(typeCell) {
		cursor, err = p.parseListField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "list", xerrors.KeyTypeCell, typeCell)
		}
	} else if types.IsStruct(typeCell) {
		cursor, err = p.parseStructField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "struct", xerrors.KeyTypeCell, typeCell)
		}
	} else {
		// scalar or enum type
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		scalarField, err := p.parseScalarField(trimmedNameCell, typeCell, noteCell)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "scalar/enum", xerrors.KeyTypeCell, typeCell)
		}
		proto.Merge(field, scalarField)
	}

	return cursor, true, nil
}

func (p *bookParser) parseSubField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (int, error) {
	subField := &tableaupb.Field{}
	cursor, parsed, err := p.parseField(subField, header, cursor, prefix, options...)
	if err != nil {
		return cursor, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "scalar/enum")
	}
	if parsed {
		field.Fields = append(field.Fields, subField)
	}
	return cursor, nil
}

func (p *bookParser) parseMapField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
	// refer: https://developers.google.com/protocol-buffers/docs/proto3#maps
	//
	//	map<key_type, value_type> map_field = N;
	//
	// where the key_type can be any integral or string type (so, any scalar type
	// except for floating point types and bytes). Note that enum is not a valid
	// key_type. The value_type can be any type except another map.
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getValidNameCell(&cursor)
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
	valueTypeDesc, err := p.parseType(valueType)
	if err != nil {
		return cursor, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, valueType+" (map value)",
			xerrors.KeyPBFieldOpts, rawPropText)
	}

	mapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.Name)
	fullMapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.FullName)

	isScalarValue := types.IsScalarType(valueTypeDesc.Name)
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
			nextNameCell := header.getValidNameCell(&nextCursor)
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
			prefix += valueTypeDesc.Name // add prefix with value type
		}
		// auto add suffix "_map".
		field.Name = strcase.ToSnake(valueTypeDesc.Name) + mapVarSuffix
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDesc.Predefined
		// TODO: support define custom variable name for predefined map value type.
		// We can use descriptor to get the first field of predefined map value type,
		// use its name option as column name, and then extract the custom variable name.
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     valueTypeDesc.Name,
			ValueFullType: valueTypeDesc.FullName,
		}

		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		field.Options = &tableaupb.FieldOptions{
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}
		if opts.Nested {
			field.Options.Name = valueTypeDesc.Name
		}
		scalarField, err := p.parseScalarField(trimmedNameCell, keyType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, keyType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Fields = append(field.Fields, scalarField)
		for cursor++; cursor < len(header.namerow); cursor++ {
			if opts.Nested {
				nameCell := header.getValidNameCell(&cursor)
				if !strings.HasPrefix(nameCell, prefix) {
					cursor--
					return cursor, nil
				}
			}
			cursor, err = p.parseSubField(field, header, cursor, prefix, options...)
			if err != nil {
				return cursor, err
			}
		}
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		// horizontal list: continuous N columns belong to this list after this cursor.
		mapName := trimmedNameCell[:firstElemIndex]
		prefix += mapName
		// auto add suffix "_map".
		field.Name = strcase.ToSnake(mapName) + mapVarSuffix
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDesc.Predefined
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     valueTypeDesc.Name,
			ValueFullType: valueTypeDesc.FullName,
		}

		trimmedNameCell := strings.TrimPrefix(nameCell, prefix+"1")
		field.Options = &tableaupb.FieldOptions{
			Name:   mapName,
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}

		name := strings.TrimPrefix(nameCell, prefix+"1")
		scalarField, err := p.parseScalarField(name, keyType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, keyType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, name)
		}
		field.Fields = append(field.Fields, scalarField)

		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getValidNameCell(&cursor)
			if types.BelongToFirstElement(nameCell, prefix) {
				cursor, err = p.parseSubField(field, header, cursor, prefix+"1", options...)
				if err != nil {
					return cursor, err
				}
			} else if strings.HasPrefix(nameCell, prefix) {
				continue
			} else {
				cursor--
				return cursor, nil
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		// incell map
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		// auto add suffix "_map".
		field.Name = strcase.ToSnake(trimmedNameCell) + mapVarSuffix
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valueTypeDesc.Predefined
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     valueTypeDesc.Name,
			ValueFullType: valueTypeDesc.FullName,
		}
		field.Options = &tableaupb.FieldOptions{
			Name:   trimmedNameCell,
			Layout: layout,
			Prop:   types.ParseProp(rawPropText),
		}
	case tableaupb.Layout_LAYOUT_DEFAULT:
		return cursor, xerrors.Errorf("should not reach default layout: %v", layout)
	}

	return cursor, nil
}

func (p *bookParser) parseListField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getValidNameCell(&cursor)
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
	pureElemTypeName := originalElemType
	elemType := originalElemType
	if elemType == "" {
		listElemSpanInnerCell = true
		isScalarElement = true
		elemType = colType
		pureElemTypeName = colType
		if matches := types.MatchStruct(colType); len(matches) > 0 {
			structType := strings.TrimSpace(matches[1])
			colType := strings.TrimSpace(matches[2])
			elemType = colType
			pureElemTypeName = colType
			// rawPropText := strings.TrimSpace(matches[3])
			if colType == "" {
				// incell predefined struct
				listElemSpanInnerCell = true
				elemType = structType
				typeDesc, err := p.parseType(structType)
				if err != nil {
					return cursor, xerrors.WithMessageKV(err,
						xerrors.KeyPBFieldType, structType,
						xerrors.KeyPBFieldOpts, rawPropText)
				}
				pureElemTypeName = typeDesc.Name
			}
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
			nextNameCell := header.getValidNameCell(&nextCursor)
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
		scalarField, err := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType+" (list element)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Type = "repeated " + scalarField.Type
		field.FullType = "repeated " + scalarField.FullType
		field.ListEntry = &tableaupb.Field_ListEntry{
			ElemType:     scalarField.Type,
			ElemFullType: scalarField.FullType,
		}
		// auto add suffix "_list".
		field.Name = strcase.ToSnake(pureElemTypeName) + listVarSuffix

		field.Options.Name = "" // Default, name is empty for vertical list
		field.Options.Layout = layout

		if opts.Nested {
			prefix += field.ListEntry.ElemType // add prefix with value type
			field.Options.Name = field.ListEntry.ElemType
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
			_, parsed, err := p.parseField(tempField, header, cursor, prefix, firstFieldOptions...)
			if err != nil {
				return cursor, err
			}
			if parsed {
				field.Fields = tempField.Fields
				field.Predefined = tempField.Predefined
				field.Options.Span = tempField.Options.Span
				field.Options.Name = tempField.Options.Name
			} else {
				return cursor, xerrors.Errorf("failed to parse list inner cell element, name cell: %s, type cell: %s", nameCell, typeCell)
			}
		} else {
			cursor, err = p.parseSubField(field, header, cursor, prefix, firstFieldOptions...)
			if err != nil {
				return cursor, err
			}
		}
		// Parse other fields
		for cursor++; cursor < len(header.namerow); cursor++ {
			if opts.Nested {
				nameCell := header.getValidNameCell(&cursor)
				if !strings.HasPrefix(nameCell, prefix) {
					cursor--
					return cursor, nil
				}
			}
			cursor, err = p.parseSubField(field, header, cursor, prefix, options...)
			if err != nil {
				return cursor, err
			}
		}

	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		// horizontal list: continuous N columns belong to this list after this cursor.
		listName := trimmedNameCell[:firstElemIndex]
		prefix += listName

		scalarField, err := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType+" (list element)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Type = "repeated " + scalarField.Type
		field.FullType = "repeated " + scalarField.FullType
		field.ListEntry = &tableaupb.Field_ListEntry{
			ElemType:     scalarField.Type,
			ElemFullType: scalarField.FullType,
		}
		// auto add suffix "_list".
		field.Name = strcase.ToSnake(listName) + listVarSuffix
		field.Options.Name = listName
		field.Options.Layout = layout

		if prop := types.ParseProp(rawPropText); prop != nil && (prop.Fixed || prop.Length != 0) {
			// only set prop if fixed or length is set.
			field.Options.Prop = &tableaupb.FieldProp{
				Fixed:  prop.Fixed,
				Length: prop.Length,
			}
		}

		// Parse first field
		colTypeWithProp := colType + rawPropText
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, colTypeWithProp))
		if listElemSpanInnerCell {
			// inner cell element
			tempField := &tableaupb.Field{}
			_, parsed, err := p.parseField(tempField, header, cursor, prefix+"1", firstFieldOptions...)
			if err != nil {
				return cursor, err
			}
			if parsed {
				field.Fields = tempField.Fields
				field.Predefined = tempField.Predefined
				field.Options.Span = tempField.Options.Span
			} else {
				return cursor, xerrors.Errorf("failed to parse list inner cell element, name cell: %s, type cell: %s", nameCell, typeCell)
			}
		} else {
			// cross cell element
			cursor, err = p.parseSubField(field, header, cursor, prefix+"1", firstFieldOptions...)
			if err != nil {
				return cursor, err
			}
		}
		// Parse other fields or skip cotinuous N columns of the same element type.
		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getValidNameCell(&cursor)
			if !listElemSpanInnerCell && types.BelongToFirstElement(nameCell, prefix) {
				cursor, err = p.parseSubField(field, header, cursor, prefix+"1", options...)
				if err != nil {
					return cursor, err
				}
			} else if strings.HasPrefix(nameCell, prefix) {
				continue
			} else {
				cursor--
				return cursor, nil
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		// incell list
		key := ""
		if matches := types.MatchKeyedList(typeCell); matches != nil {
			// set column type and key if this is a keyed list.
			colType = strings.TrimSpace(matches[2])
			key = trimmedNameCell
		}
		colTypeWithProp := colType + rawPropText
		scalarField, err := p.parseScalarField(trimmedNameCell, colTypeWithProp, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, colType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		// auto add suffix "_list".
		field.Name += listVarSuffix
		field.Type = "repeated " + scalarField.Type
		field.FullType = "repeated " + scalarField.FullType
		field.ListEntry = &tableaupb.Field_ListEntry{
			ElemType:     scalarField.Type,
			ElemFullType: scalarField.FullType,
		}
		field.Options.Layout = layout
		field.Options.Key = key
	case tableaupb.Layout_LAYOUT_DEFAULT:
		return cursor, xerrors.Errorf("should not reach default layout: %s", layout)
	}
	return cursor, nil
}

func (p *bookParser) parseStructField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getValidNameCell(&cursor)
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
	if colType == "" {
		// incell predefined struct
		scalarField, err := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		return cursor, nil
	}

	fieldPairs, err := ParseIncellStruct(elemType)
	if err != nil {
		return cursor, err
	}
	if fieldPairs != nil {
		scalarField, err := p.parseScalarField(trimmedNameCell, colType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, colType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL

		for i := 0; i < len(fieldPairs); i += 2 {
			fieldType := fieldPairs[i]
			fieldName := fieldPairs[i+1]
			scalarField, err := p.parseScalarField(fieldName, fieldType, "")
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, fieldType,
					xerrors.KeyPBFieldOpts, rawPropText,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)
		}
	} else {
		// cross cell struct
		// NOTE(wenchy): treated as nested named struct
		scalarField, err := p.parseScalarField(trimmedNameCell, elemType, noteCell)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
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
		cursor, err = p.parseSubField(field, header, cursor, prefix, firstFieldOptions...)
		if err != nil {
			return cursor, err
		}
		for cursor++; cursor < len(header.namerow); cursor++ {
			nameCell := header.getValidNameCell(&cursor)
			if !strings.HasPrefix(nameCell, prefix) {
				cursor--
				return cursor, nil
			}
			cursor, err = p.parseSubField(field, header, cursor, prefix, options...)
			if err != nil {
				return cursor, err
			}
		}
	}

	return cursor, nil
}

func (p *bookParser) parseScalarField(name, typ, note string) (*tableaupb.Field, error) {
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
	typeDesc, err := p.parseType(typ)
	if err != nil {
		return nil, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldOpts, rawPropText,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}

	return &tableaupb.Field{
		Name:       strcase.ToSnake(name),
		Type:       typeDesc.Name,
		FullType:   typeDesc.FullName,
		Predefined: typeDesc.Predefined,
		Options: &tableaupb.FieldOptions{
			Name: name,
			Note: p.genNote(note),
			Prop: types.ParseProp(rawPropText),
		},
	}, nil
}

func (p *bookParser) genNote(note string) string {
	if p.withNote {
		return note
	}
	return ""
}

type typeDesc struct {
	Name       string
	FullName   string
	Predefined bool
}

func (p *bookParser) parseType(rawType string) (*typeDesc, error) {
	if strings.HasPrefix(rawType, ".") {
		// This messge type is defined in imported proto
		name := strings.TrimPrefix(rawType, ".")
		if typeInfo, ok := p.gen.typeInfos[name]; ok {
			return &typeDesc{
				Name:       name,
				FullName:   typeInfo.Fullname,
				Predefined: true,
			}, nil
		} else {
			return nil, xerrors.Errorf("predefined type not found: %s", name)
		}
	}
	switch rawType {
	case "datetime", "date":
		return &typeDesc{
			Name:       "google.protobuf.Timestamp",
			FullName:   "google.protobuf.Timestamp",
			Predefined: true,
		}, nil
	case "duration", "time":
		return &typeDesc{
			Name:       "google.protobuf.Duration",
			FullName:   "google.protobuf.Duration",
			Predefined: true,
		}, nil
	default:
		return &typeDesc{
			Name:       rawType,
			FullName:   rawType,
			Predefined: false,
		}, nil
	}
}

func ParseIncellStruct(elemType string) ([]string, error) {
	fields := strings.Split(elemType, ",")
	if len(fields) == 1 && len(strings.Split(fields[0], " ")) == 1 {
		// cross cell struct
		return nil, nil
	}

	fieldPairs := make([]string, 0)
	for _, pair := range strings.Split(elemType, ",") {
		kv := strings.Split(strings.TrimSpace(pair), " ")
		if len(kv) != 2 {
			return nil, xerrors.Errorf("illegal type-variable pair: %v in incell struct: %s", pair, elemType)
		}
		fieldPairs = append(fieldPairs, kv...)
	}
	return fieldPairs, nil
}
