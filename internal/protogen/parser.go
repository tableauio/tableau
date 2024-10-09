package protogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/protogen/parseroptions"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	tableauProtoPath = "tableau/protobuf/tableau.proto"
)

const (
	mapVarSuffix  = "_map"  // map variable name suffix
	listVarSuffix = "_list" // list variable name suffix
)

type bookParser struct {
	gen *Generator
	wb  *tableaupb.Workbook
}

func newBookParser(bookName, alias, relSlashPath string, gen *Generator) *bookParser {
	// log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	protoBookName := bookName // generated proto book file name
	if alias != "" {
		protoBookName = alias
	}
	filename := strcase.ToSnake(protoBookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), protoBookName)
		snakePath := strcase.ToSnake(fs.CleanSlashPath(bookPath))
		filename = strings.ReplaceAll(snakePath, "/", "__")
	}
	bp := &bookParser{
		gen: gen,
		wb: &tableaupb.Workbook{
			Options: &tableaupb.WorkbookOptions{
				// NOTE(wenchy): all OS platforms use path slash separator `/`
				// see: https://stackoverflow.com/questions/9371031/how-do-i-create-crossplatform-file-paths-in-go
				Name:  relSlashPath,
				Alias: alias,
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

func (p *bookParser) GetProtoFilePath() string {
	return genProtoFilePath(p.wb.Name, p.gen.OutputOpt.FilenameSuffix)
}

func (p *bookParser) parseField(field *tableaupb.Field, header *tableHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, parsed bool, err error) {
	nameCell := header.getValidNameCell(&cursor)
	typeCell := header.getTypeCell(cursor)
	// log.Debugf("column: %d, name: %s, type: %s", cursor, nameCell, typeCell)
	if nameCell == "" || typeCell == "" {
		log.Debugf("no need to parse column %d, as name(%s) or type(%s) is empty", cursor, nameCell, typeCell)
		return cursor, false, nil
	}

	if err := header.checkNameConflicts(nameCell, cursor); err != nil {
		return cursor, false, err
	}

	opts := parseroptions.ParseOptions(options...)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}
	if types.IsMap(typeCell) {
		cursor, err = p.parseMapField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "map")
		}
	} else if types.IsList(typeCell) {
		cursor, err = p.parseListField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "list")
		}
	} else if types.IsStruct(typeCell) {
		cursor, err = p.parseStructField(field, header, cursor, prefix, options...)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "struct")
		}
	} else {
		// scalar or enum type
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, typeCell)
		if err != nil {
			return cursor, false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "scalar/enum")
		}
		proto.Merge(field, scalarField)
	}

	return cursor, true, nil
}

func (p *bookParser) parseSubField(field *tableaupb.Field, header *tableHeader, cursor int, prefix string, options ...parseroptions.Option) (int, error) {
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

func (p *bookParser) parseMapField(field *tableaupb.Field, header *tableHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
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
	desc := types.MatchMap(typeCell)

	parsedKeyType := desc.KeyType
	if types.IsEnum(desc.KeyType) {
		// NOTE: support enum as map key, convert key type as `int32`.
		parsedKeyType = "int32"
	}

	valueTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, desc.ValueType)
	if err != nil {
		return cursor, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.ValueType+" (map value)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
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
		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, desc.KeyType+desc.Prop.RawProp())
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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
		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options = &tableaupb.FieldOptions{
			Name:   mapName,
			Key:    trimmedNameCell,
			Layout: layout,
			Prop:   ExtractMapFieldProp(prop),
		}
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, desc.KeyType+desc.Prop.RawProp())
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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

		keyTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, desc.KeyType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.ValueType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
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
		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, mapType+" (incell map)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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

			scalarField, err := parseField(p.gen.typeInfos, types.DefaultMapKeyOptName, desc.KeyType+desc.Prop.RawProp())
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)

			scalarField, err = parseField(p.gen.typeInfos, types.DefaultMapValueOptName, desc.ValueType)
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, desc.ValueType+" (map value)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)
		}
	case tableaupb.Layout_LAYOUT_DEFAULT:
		return cursor, xerrors.Errorf("should not reach default layout: %v", layout)
	}

	return cursor, nil
}

func (p *bookParser) parseListField(field *tableaupb.Field, header *tableHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getValidNameCell(&cursor)
	typeCell := header.getTypeCell(cursor)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}

	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

	// list syntax pattern
	desc := types.MatchList(typeCell)

	listElemSpanInnerCell, isScalarElement := false, false
	elemType := desc.ElemType
	pureElemTypeName := desc.ElemType
	if elemType == "" {
		listElemSpanInnerCell = true
		isScalarElement = true
		elemType = desc.ColumnType
		pureElemTypeName = desc.ColumnType
		if structDesc := types.MatchStruct(desc.ColumnType); structDesc != nil {
			elemType = structDesc.ColumnType
			pureElemTypeName = structDesc.ColumnType
			if structDesc.ColumnType == "" {
				// incell predefined struct
				listElemSpanInnerCell = true
				elemType = structDesc.StructType
				typeDesc, err := parseTypeDescriptor(p.gen.typeInfos, structDesc.StructType)
				if err != nil {
					return cursor, xerrors.WithMessageKV(err,
						xerrors.KeyPBFieldType, structDesc.StructType,
						xerrors.KeyPBFieldOpts, structDesc.Prop.Text)
				}
				pureElemTypeName = typeDesc.Name
			}
			isScalarElement = false
		}
	} else {
		typeDesc, err := parseTypeDescriptor(p.gen.typeInfos, desc.ElemType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.ElemType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
		}
		pureElemTypeName = typeDesc.Name
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
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, elemType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType+" (list element)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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

		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (vertical list)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options.Prop = ExtractListFieldProp(prop, types.IsScalarType(field.ListEntry.ElemType))

		if opts.Nested {
			prefix += field.ListEntry.ElemType // add prefix with value type
			field.Options.Name = field.ListEntry.ElemType
		}
		trimmedNameCell := strings.TrimPrefix(nameCell, prefix)

		colType := desc.ColumnType
		if keydeListDesc := types.MatchKeyedList(typeCell); keydeListDesc != nil {
			// set column type and key if this is a keyed list.
			colType = keydeListDesc.ColumnType
			field.Options.Key = trimmedNameCell
		}
		// Parse first field
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, colType+desc.Prop.RawProp()))
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

		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, elemType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, elemType+" (list element)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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
		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (horizontal list)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		field.Options.Prop = ExtractListFieldProp(prop, types.IsScalarType(field.ListEntry.ElemType))

		// Parse first field
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, desc.ColumnType+desc.Prop.RawProp()))
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
		colType := desc.ColumnType
		if keyedListDesc := types.MatchKeyedList(typeCell); keyedListDesc != nil {
			// set column type and key if this is a keyed list.
			colType = keyedListDesc.ColumnType
			key = trimmedNameCell
		}
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, colType+desc.Prop.RawProp())
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, colType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)

		prop, err := desc.Prop.FieldProp()
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, field.Type+" (incell list)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
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

func (p *bookParser) parseStructField(field *tableaupb.Field, header *tableHeader, cursor int, prefix string, options ...parseroptions.Option) (cur int, err error) {
	opts := parseroptions.ParseOptions(options...)

	nameCell := header.getValidNameCell(&cursor)
	typeCell := header.getTypeCell(cursor)
	if opts.GetVTypeCell(cursor) != "" {
		typeCell = opts.GetVTypeCell(cursor)
	}

	trimmedNameCell := strings.TrimPrefix(nameCell, prefix)
	// struct syntax pattern
	desc := types.MatchStruct(typeCell)
	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return cursor, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.StructType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, trimmedNameCell)
	}

	if desc.ColumnType == "" {
		// incell predefined struct
		structField, err := parseField(p.gen.typeInfos, trimmedNameCell, desc.StructType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.StructType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, structField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		field.Options.Prop = ExtractStructFieldProp(prop)
		return cursor, nil
	}

	fieldPairs, err := parseIncellStruct(desc.StructType)
	if err != nil {
		return cursor, err
	}
	if fieldPairs != nil {
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, desc.ColumnType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.ColumnType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		field.Options.Prop = ExtractStructFieldProp(prop)

		for i := 0; i < len(fieldPairs); i += 2 {
			fieldType := fieldPairs[i]
			fieldName := fieldPairs[i+1]
			scalarField, err := parseField(p.gen.typeInfos, fieldName, fieldType)
			if err != nil {
				return cursor, xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, fieldType,
					xerrors.KeyPBFieldOpts, desc.Prop.Text,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			field.Fields = append(field.Fields, scalarField)
		}
	} else {
		// cross cell struct
		// NOTE(wenchy): each column name should be prefixed with the same struct variable name.
		scalarField, err := parseField(p.gen.typeInfos, trimmedNameCell, desc.StructType)
		if err != nil {
			return cursor, xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.StructType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, trimmedNameCell)
		}
		proto.Merge(field, scalarField)

		structName := field.Type // default: struct name is same as the type name
		if desc.CustomName != "" {
			structName = desc.CustomName
		}
		if field.Predefined {
			// Find predefined type's first field's option name
			fullMsgName := protoreflect.FullName(field.FullType)
			typeInfo := p.gen.typeInfos.GetByFullName(fullMsgName)
			if typeInfo == nil {
				return cursor, xerrors.ErrorKV(fmt.Sprintf("predefined type not found: %v", fullMsgName),
					xerrors.KeyPBFieldType, desc.StructType,
					xerrors.KeyPBFieldOpts, desc.Prop.Text,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			if typeInfo.FirstFieldOptionName == "" {
				return cursor, xerrors.ErrorKV(fmt.Sprintf("predefined type's first field option name not set: %v", fullMsgName),
					xerrors.KeyPBFieldType, desc.StructType,
					xerrors.KeyPBFieldOpts, desc.Prop.Text,
					xerrors.KeyTrimmedNameCell, trimmedNameCell)
			}
			if index := strings.Index(trimmedNameCell, typeInfo.FirstFieldOptionName); index != -1 {
				structName = trimmedNameCell[:index]
			}
		}

		field.Name = strcase.ToSnake(structName)
		field.Options = &tableaupb.FieldOptions{
			Name: structName,
			Prop: ExtractStructFieldProp(prop),
		}
		prefix += structName
		firstFieldOptions := append(options, parseroptions.VTypeCell(cursor, desc.ColumnType+desc.Prop.RawProp()))
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

func parseField(typeInfos *xproto.TypeInfos, name, typ string) (*tableaupb.Field, error) {
	var prop types.PropDescriptor
	// enum syntax pattern
	if desc := types.MatchEnum(typ); desc != nil {
		typ = desc.EnumType
		prop = desc.Prop
	} else if desc := types.MatchScalar(typ); desc != nil {
		// scalar syntax pattern
		typ = desc.ScalarType
		prop = desc.Prop
	}
	typeDesc, err := parseTypeDescriptor(typeInfos, typ)
	if err != nil {
		return nil, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldOpts, prop.Text,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}

	fieldProp, err := prop.FieldProp()
	if err != nil {
		return nil, xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldOpts, prop.Text,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}
	pureName := strings.TrimPrefix(name, book.MetaSign) // remove leading meta sign "@""
	return &tableaupb.Field{
		Name:       strcase.ToSnake(pureName),
		Type:       typeDesc.Name,
		FullType:   typeDesc.FullName,
		Predefined: typeDesc.Predefined,
		Options: &tableaupb.FieldOptions{
			Name: name,
			Note: "", // no need to add note now, maybe will be deprecated in the future.
			Prop: ExtractScalarFieldProp(fieldProp),
		},
	}, nil
}

func parseTypeDescriptor(typeInfos *xproto.TypeInfos, rawType string) (*types.Descriptor, error) {
	// enum syntax pattern
	if desc := types.MatchEnum(rawType); desc != nil {
		rawType = desc.EnumType
	}

	if strings.Contains(rawType, ".") {
		// This messge type is predefined
		if typeInfo := typeInfos.Get(rawType); typeInfo != nil {
			return &types.Descriptor{
				Name:       string(typeInfo.FullName.Name()),
				FullName:   string(typeInfo.FullName),
				Predefined: true,
				Kind:       typeInfo.Kind,
			}, nil
		} else {
			return nil, xerrors.Errorf("predefined type not found: %s", rawType)
		}
	}
	return types.ParseTypeDescriptor(rawType), nil
}

// parseIncellStruct parses incell struct type definition. For example:
//   - int32 ID
//   - int32 ID, string Name
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
