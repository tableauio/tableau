package protogen

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

type documentBookParser struct {
	gen *Generator
	wb  *tableaupb.Workbook
}

func newDocumentBookParser(bookName, relSlashPath string, gen *Generator) *documentBookParser {
	// log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	filename := strcase.ToSnake(bookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), bookName)
		snakePath := strcase.ToSnake(fs.CleanSlashPath(bookPath))
		filename = strings.ReplaceAll(snakePath, "/", "__")
	}
	bp := &documentBookParser{
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

func (x *documentBookParser) GetProtoFilePath() string {
	return genProtoFilePath(x.wb.Name, x.gen.OutputOpt.FilenameSuffix)
}

func (p *documentBookParser) parseField(field *tableaupb.Field, node *book.Node) (parsed bool, err error) {
	nameCell := node.Name
	if nameCell == book.SheetKey {
		return false, nil
	}
	typeCell := node.GetMetaType()

	if types.IsMap(typeCell) {
		err = p.parseMapField(field, node)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "map")
		}
	} else if types.IsList(typeCell) {
		err = p.parseListField(field, node)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "list")
		}
	} else if types.IsStruct(typeCell) {
		err = p.parseStructField(field, node)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "struct")
		}
	} else {
		// scalar or enum type
		scalarField, err := parseField(p.gen.typeInfos, nameCell, typeCell)
		if err != nil {
			return false, xerrors.WithMessageKV(err, xerrors.KeyPBFieldType, "scalar/enum")
		}
		proto.Merge(field, scalarField)
	}

	return true, nil
}

func (p *documentBookParser) parseSubField(field *tableaupb.Field, node *book.Node) error {
	subField := &tableaupb.Field{}
	parsed, err := p.parseField(subField, node)
	if err != nil {
		return err
	}
	if parsed {
		field.Fields = append(field.Fields, subField)
	}
	return nil
}

func (p *documentBookParser) parseMapField(field *tableaupb.Field, node *book.Node) error {
	typeCell := node.GetMetaType()
	desc := types.MatchMap(typeCell)
	parsedKeyType := desc.KeyType
	if types.IsEnum(desc.KeyType) {
		// NOTE: support enum as map key, convert key type as `int32`.
		parsedKeyType = "int32"
	}
	valueTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, desc.ValueType)
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.ValueType+" (map value)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}

	mapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.Name)
	fullMapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.FullName)
	mapValueKind := valueTypeDesc.Kind
	parsedValueName := valueTypeDesc.Name
	parsedValueFullName := valueTypeDesc.FullName
	valuePredefined := valueTypeDesc.Predefined

	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, node.Name)
	}

	// scalar map
	if mapValueKind == types.ScalarKind || mapValueKind == types.EnumKind {
		keyTypeDesc, err := parseTypeDescriptor(p.gen.typeInfos, desc.KeyType)
		if err != nil {
			return xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
		}
		// special process for key as enum type: create a new simple KV message as map value type.
		if keyTypeDesc.Kind == types.EnumKind {
			valuePredefined = false
			parsedValueName = node.Name + "Value"
			// custom value type name
			if structNode := node.GetMetaStructNode(); structNode != nil && structNode.Value != "" {
				parsedValueName = structNode.Value
			}
			parsedValueFullName = parsedValueName
			mapType = fmt.Sprintf("map<%s, %s>", parsedKeyType, parsedValueName)
			fullMapType = mapType
		}

		// auto add suffix "_map".
		// field.Name = strcase.ToSnake(valueTypeDesc.Name) + mapVarSuffix
		field.Name = strcase.ToSnake(valueTypeDesc.Name)
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valuePredefined
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     parsedValueName,
			ValueFullType: parsedValueFullName,
		}
		// auto add suffix "_map".
		// field.Name = strcase.ToSnake(node.Name) + mapVarSuffix
		field.Name = strcase.ToSnake(node.Name)
		field.Options = &tableaupb.FieldOptions{
			Name: node.Name,
			Prop: ExtractMapFieldProp(prop),
		}

		// special process for key as enum type: create a new simple KV message as map value type.
		if keyTypeDesc.Kind == types.EnumKind {
			field.Options.Key = book.KeywordKey
			// 1. append key to the first value struct field
			scalarField, err := parseField(p.gen.typeInfos, book.KeywordKey, desc.KeyType+desc.Prop.RawProp())
			if err != nil {
				return xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			field.Fields = append(field.Fields, scalarField)
			// 2. append value to the second value struct field
			scalarField, err = parseField(p.gen.typeInfos, book.KeywordValue, desc.ValueType)
			if err != nil {
				return xerrors.WithMessageKV(err,
					xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			field.Fields = append(field.Fields, scalarField)
			field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		}
		return nil
	}
	// struct map
	// auto add suffix "_map".
	// field.Name = strcase.ToSnake(valueTypeDesc.Name) + mapVarSuffix
	field.Name = strcase.ToSnake(valueTypeDesc.Name)
	field.Type = mapType
	field.FullType = fullMapType
	// For map type, Predefined indicates the ValueType of map has been defined.
	field.Predefined = valuePredefined
	field.MapEntry = &tableaupb.Field_MapEntry{
		KeyType:       parsedKeyType,
		ValueType:     parsedValueName,
		ValueFullType: parsedValueFullName,
	}
	// auto add suffix "_map".
	// field.Name = strcase.ToSnake(node.Name) + mapVarSuffix
	field.Name = strcase.ToSnake(node.Name)
	field.Options = &tableaupb.FieldOptions{
		Name: node.Name,
		Prop: ExtractMapFieldProp(prop),
	}
	field.Options.Key = book.KeywordKey
	// struct map
	// auto append key to the first value struct field
	scalarField, err := parseField(p.gen.typeInfos, book.KeywordKey, desc.KeyType+desc.Prop.RawProp())
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}
	scalarField.Name = strcase.ToSnake(node.GetMetaKey())
	field.Fields = append(field.Fields, scalarField)
	// parse other value fields
	structNode := node.GetMetaStructNode()
	if structNode != nil {
		for _, child := range structNode.Children {
			if child.IsMeta() {
				continue
			}
			if err := p.parseSubField(field, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *documentBookParser) parseListField(field *tableaupb.Field, node *book.Node) error {
	typeCell := node.GetMetaType()
	desc := types.MatchList(typeCell)
	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.ElemType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, node.Name)
	}
	scalarField, err := parseField(p.gen.typeInfos, node.Name, desc.ElemType)
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.ElemType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, node.Name)
	}
	proto.Merge(field, scalarField)

	field.Type = "repeated " + scalarField.Type
	field.FullType = "repeated " + scalarField.FullType
	field.ListEntry = &tableaupb.Field_ListEntry{
		ElemType:     scalarField.Type,
		ElemFullType: scalarField.FullType,
	}
	// auto add suffix "_list".
	// field.Name = strcase.ToSnake(node.Name) + listVarSuffix
	field.Name = strcase.ToSnake(node.Name)
	field.Options = &tableaupb.FieldOptions{
		Name: node.Name,
		Prop: ExtractStructFieldProp(prop),
	}
	structNode := node.GetMetaStructNode()
	if structNode != nil {
		for _, child := range structNode.Children {
			if child.IsMeta() {
				continue
			}
			if err := p.parseSubField(field, child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *documentBookParser) parseStructField(field *tableaupb.Field, node *book.Node) error {
	typeCell := node.GetMetaType()
	desc := types.MatchStruct(typeCell)
	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.StructType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, node.Name)
	}
	structNode := node.GetMetaStructNode()
	if structNode == nil {
		// predefined struct
		structField, err := parseField(p.gen.typeInfos, node.Name, desc.StructType)
		if err != nil {
			return xerrors.WithMessageKV(err,
				xerrors.KeyPBFieldType, desc.StructType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text,
				xerrors.KeyTrimmedNameCell, node.Name)
		}
		proto.Merge(field, structField)
		field.Options.Prop = ExtractStructFieldProp(prop)
		return nil
	}
	scalarField, err := parseField(p.gen.typeInfos, node.Name, desc.StructType)
	if err != nil {
		return xerrors.WithMessageKV(err,
			xerrors.KeyPBFieldType, desc.StructType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text,
			xerrors.KeyTrimmedNameCell, node.Name)
	}
	proto.Merge(field, scalarField)

	field.Name = strcase.ToSnake(node.Name)
	field.Options = &tableaupb.FieldOptions{
		Name: node.Name,
		Prop: ExtractStructFieldProp(prop),
	}
	for _, child := range structNode.Children {
		if child.IsMeta() {
			continue
		}
		if err := p.parseSubField(field, child); err != nil {
			return err
		}
	}
	return nil
}
