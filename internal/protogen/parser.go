package protogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
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
	wb  *tableaupb.Workbook
}

func newBookParser(bookName, relSlashPath string, gen *Generator) *bookParser {
	// log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	filename := strcase.ToSnake(bookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), bookName)
		snakePath := strcase.ToSnake(fs.GetCleanSlashPath(bookPath))
		filename = strings.ReplaceAll(snakePath, "/", "__")
	}
	bp := &bookParser{
		gen: gen,
		wb: &tableaupb.Workbook{
			Options: &tableaupb.WorkbookOptions{
				// NOTE(wenchy): all OS platforms use path slash separator `/`
				// see: https://stackoverflow.com/questions/9371031/how-do-i-create-crossplatform-file-paths-in-go
				Name: relSlashPath,
			},
			Worksheets: []*tableaupb.Worksheet{},
			Name:       filename,
			Imports:    make(map[string]int32),
		},
	}

	// custom imported proto files
	for _, path := range gen.InputOpt.ProtoFiles {
		bp.wb.Imports[path] = 1
	}
	return bp
}

func (p *bookParser) parseField(field *tableaupb.Field, header *sheetHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, parsed bool, err error) {
	nameCell := header.getValidNameCell(&cursor)
	typeCell := header.getTypeCell(cursor)
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
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, typeCell)
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

	valueTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, valueType)
	if err != nil {
		return cursor, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, valueType+" (map value)",
			xerrors.KeyPBFieldOpts, rawPropText)
	}

	mapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.Name)
	fullMapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.FullName)

	mapValueKind := valueTypeDesc.Kind
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
					if mapValueKind == types.ScalarKind || mapValueKind == types.EnumKind {
						layout = tableaupb.Layout_LAYOUT_INCELL // incell map
					}
				}
			} else {
				// only one map item, treat it as incell map
				if mapValueKind == types.ScalarKind || mapValueKind == types.EnumKind {
					layout = tableaupb.Layout_LAYOUT_INCELL // incell map
				}
			}
		}
	} else {
		if mapValueKind == types.ScalarKind || mapValueKind == types.EnumKind {
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
		// extract map field property
		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, keyType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options = &tableaupb.FieldOptions{
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   ExtractMapFieldProp(prop),
		}
		if opts.Nested {
			field.Options.Name = valueTypeDesc.Name
		}
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, keyType+rawPropText)
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
		// horizontal map: continuous N columns belong to this map after this cursor.
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
		// extract map field property
		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, keyType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options = &tableaupb.FieldOptions{
			Name:   mapName,
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   ExtractMapFieldProp(prop),
		}
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, keyType+rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, keyType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Fields = append(field.Fields, scalarField)

		// Parse other fields or skip continuous N columns of the same element type.
		for cursor++; cursor < len(header.namerow); cursor++ {
			typeCell := header.getTypeCell(cursor)
			if typeCell == "" {
				continue // continue to skip this column if type cell is empty
			}
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
		valuePredefined := valueTypeDesc.Predefined
		parsedValueName := valueTypeDesc.Name
		parsedValueFullName := valueTypeDesc.FullName

		keyTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, keyType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, valueType+" (map key)",
				xerrors.KeyPBFieldOpts, rawPropText)
		}

		// special process for key as enum type
		if keyTypeDesc.Kind == types.EnumKind {
			valuePredefined = false
			parsedValueName = trimmedNameCell
			parsedValueFullName = trimmedNameCell
			mapType = fmt.Sprintf("map<%s, %s>", parsedKeyType, parsedValueName)
			fullMapType = mapType
		}

		// auto add suffix "_map".
		field.Name = strcase.ToSnake(trimmedNameCell) + mapVarSuffix
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has already
		// been defined before.
		field.Predefined = valuePredefined
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     parsedValueName,
			ValueFullType: parsedValueFullName,
		}
		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, mapType+" (incell map)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options = &tableaupb.FieldOptions{
			Name:   trimmedNameCell,
			Layout: layout,
			Prop:   prop, // for incell scalar map, need whole prop
		}

		// special process for key as enum type: create a new simple KV message as map value type.
		if keyTypeDesc.Kind == types.EnumKind {
			field.Options.Key = types.DefaultMapKeyOptName

			scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, types.DefaultMapKeyOptName, keyType+rawPropText)
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, keyType+" (map key)",
					xerrors.KeyPBFieldOpts, rawPropText,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)

			scalarField, err = parseScalarOrEnumField(p.gen.typeInfos, types.DefaultMapValueOptName, valueType)
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, valueType+" (map value)",
					xerrors.KeyPBFieldOpts, rawPropText,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)
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
				typeDesc, err := parseTypeDescriptor(p.gen.typeInfos, structType)
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
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, elemType)
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

		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (vertical list)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options.Prop = ExtractListFieldProp(prop)

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

		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, elemType)
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

		// extract list field property
		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (horizontal list)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options.Prop = ExtractListFieldProp(prop)

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
		// Parse other fields or skip continuous N columns of the same element type.
		for cursor++; cursor < len(header.namerow); cursor++ {
			typeCell := header.getTypeCell(cursor)
			if typeCell == "" {
				continue // continue to skip this column if type cell is empty
			}
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
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, colTypeWithProp)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, colType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)

		prop, err := types.ParseProp(rawPropText)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (incell list)",
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		// for incell scalar list, need whole prop
		field.Options.Prop = prop

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

	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

	// struct syntax pattern
	matches := types.MatchStruct(typeCell)
	structType := strings.TrimSpace(matches[1])
	colType := strings.TrimSpace(matches[2])
	rawPropText := strings.TrimSpace(matches[3])
	if colType == "" {
		// incell predefined struct
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, structType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, structType,
				xerrors.KeyPBFieldOpts, rawPropText,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		return cursor, nil
	}

	fieldPairs, err := parseIncellStruct(structType)
	if err != nil {
		return cursor, err
	}
	if fieldPairs != nil {
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, colType)
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
			scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, fieldName, fieldType)
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
		scalarField, err := parseScalarOrEnumField(p.gen.typeInfos, trimmedNameCell, structType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, structType,
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
				// first field
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

func parseScalarOrEnumField(typeInfos xproto.TypeInfoMap, name, typ string) (*tableaupb.Field, error) {
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
	typeDesc, err := parseTypeDescriptor(typeInfos, typ)
	if err != nil {
		return nil, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldOpts, rawPropText,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}

	prop, err := types.ParseProp(rawPropText)
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
			Note: "", // no need to add note now, maybe will be deprecated in the future.
			Prop: ExtractScalarFieldProp(prop),
		},
	}, nil
}

func parseTypeDescriptor(typeInfos xproto.TypeInfoMap, rawType string) (*types.Descriptor, error) {
	// enum syntax pattern
	if matches := types.MatchEnum(rawType); len(matches) > 0 {
		enumType := strings.TrimSpace(matches[1])
		rawType = enumType
	}

	if strings.HasPrefix(rawType, ".") {
		// This messge type is defined in imported proto
		name := strings.TrimPrefix(rawType, ".")
		if typeInfo, ok := typeInfos[name]; ok {
			return &types.Descriptor{
				Name:       name,
				FullName:   typeInfo.Fullname,
				Predefined: true,
				Kind:       typeInfo.Kind,
			}, nil
		} else {
			return nil, xerrors.Errorf("predefined type not found: %s", name)
		}
	}
	switch rawType {
	case "datetime", "date":
		return &types.Descriptor{
			Name:       "google.protobuf.Timestamp",
			FullName:   "google.protobuf.Timestamp",
			Predefined: true,
			Kind:       types.ScalarKind,
		}, nil
	case "time", "duration":
		return &types.Descriptor{
			Name:       "google.protobuf.Duration",
			FullName:   "google.protobuf.Duration",
			Predefined: true,
			Kind:       types.ScalarKind,
		}, nil
	default:
		desc := &types.Descriptor{
			Name:       rawType,
			FullName:   rawType,
			Predefined: false,
		}
		if types.IsScalarType(desc.Name) {
			desc.Kind = types.ScalarKind
		} else {
			desc.Kind = types.MessageKind
		}
		return desc, nil
	}
}

// parseIncellStruct parses incell struct type definition. For example:
//  - int32 ID
//  - int32 ID, string Name
func parseIncellStruct(structType string) ([]string, error) {
	fields := strings.Split(structType, ",")
	if len(fields) == 1 && len(strings.Split(fields[0], " ")) == 1 {
		// cross cell struct
		return nil, nil
	}

	fieldPairs := make([]string, 0)
	for _, pair := range strings.Split(structType, ",") {
		kv := strings.Split(strings.TrimSpace(pair), " ")
		if len(kv) != 2 {
			return nil, xerrors.Errorf("illegal type-variable pair: %v in incell struct: %s", pair, structType)
		}
		fieldPairs = append(fieldPairs, kv...)
	}
	return fieldPairs, nil
}
