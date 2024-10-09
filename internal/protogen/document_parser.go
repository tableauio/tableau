package protogen

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

type documentBookParser struct {
	parser *bookParser
}

func newDocumentBookParser(bookName, alias, relSlashPath string, gen *Generator) *documentBookParser {
	parser := newBookParser(bookName, alias, relSlashPath, gen)
	return &documentBookParser{parser: parser}
}

func errWithNodeKV(err error, node *book.Node, pairs ...any) error {
	kvs := append(node.DebugKV(), pairs...)
	return xerrors.WithMessageKV(err, kvs...)
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
			return false, errWithNodeKV(err, node, xerrors.KeyPBFieldType, "map")
		}
	} else if types.IsList(typeCell) {
		err = p.parseListField(field, node)
		if err != nil {
			return false, errWithNodeKV(err, node, xerrors.KeyPBFieldType, "list")
		}
	} else if types.IsStruct(typeCell) {
		err = p.parseStructField(field, node)
		if err != nil {
			return false, errWithNodeKV(err, node, xerrors.KeyPBFieldType, "struct")
		}
	} else {
		// scalar or enum type
		scalarField, err := parseField(p.parser.gen.typeInfos, nameCell, typeCell)
		if err != nil {
			return false, errWithNodeKV(err, node, xerrors.KeyPBFieldType, "scalar/enum")
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
	typeNode := node.GetMetaTypeNode()
	typeCell := typeNode.GetValue()
	variableCell := node.GetMetaVariable()
	keynameCell := node.GetMetaKeyname()
	desc := types.MatchMap(typeCell)
	parsedKeyType := desc.KeyType
	if types.IsEnum(desc.KeyType) {
		// NOTE: support enum as map key, convert key type as `int32`.
		parsedKeyType = "int32"
	}
	valueTypeDesc, err := parseTypeDescriptor(p.parser.gen.typeInfos, desc.ValueType)
	if err != nil {
		return errWithNodeKV(err, typeNode,
			xerrors.KeyPBFieldType, desc.ValueType+" (map value)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}

	mapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.Name)
	fullMapType := fmt.Sprintf("map<%s, %s>", parsedKeyType, valueTypeDesc.FullName)
	mapValueKind := valueTypeDesc.Kind
	parsedValueName := valueTypeDesc.Name
	parsedValueFullName := valueTypeDesc.FullName
	valuePredefined := valueTypeDesc.Predefined

	// whether layout is incell or not
	layout := tableaupb.Layout_LAYOUT_DEFAULT
	if node.GetMetaIncell() {
		layout = tableaupb.Layout_LAYOUT_INCELL
	}

	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return errWithNodeKV(err, typeNode, xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}

	// scalar map
	if mapValueKind == types.ScalarKind || mapValueKind == types.EnumKind {
		keyTypeDesc, err := parseTypeDescriptor(p.parser.gen.typeInfos, desc.KeyType)
		if err != nil {
			return errWithNodeKV(err, typeNode,
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
		field.Name = strcase.ToSnake(variableCell)
		field.Type = mapType
		field.FullType = fullMapType
		// For map type, Predefined indicates the ValueType of map has been defined.
		field.Predefined = valuePredefined
		field.MapEntry = &tableaupb.Field_MapEntry{
			KeyType:       parsedKeyType,
			ValueType:     parsedValueName,
			ValueFullType: parsedValueFullName,
		}
		field.Options = &tableaupb.FieldOptions{
			Name:   node.Name,
			Layout: layout,
			Prop:   ExtractMapFieldProp(prop),
		}

		// special process for key as enum type: create a new simple KV message as map value type.
		if keyTypeDesc.Kind == types.EnumKind {
			field.Options.Key = keynameCell
			// 1. append key to the first value struct field
			scalarField, err := parseField(p.parser.gen.typeInfos, keynameCell, desc.KeyType+desc.Prop.RawProp())
			if err != nil {
				return errWithNodeKV(err, typeNode,
					xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			field.Fields = append(field.Fields, scalarField)
			// 2. append value to the second value struct field
			scalarField, err = parseField(p.parser.gen.typeInfos, book.KeywordValue, desc.ValueType)
			if err != nil {
				return errWithNodeKV(err, typeNode,
					xerrors.KeyPBFieldType, desc.ValueType+" (map value)",
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			field.Fields = append(field.Fields, scalarField)
			field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		}
		return nil
	}
	// struct map
	field.Name = strcase.ToSnake(variableCell)
	field.Type = mapType
	field.FullType = fullMapType
	// For map type, Predefined indicates the ValueType of map has been defined.
	field.Predefined = valuePredefined
	field.MapEntry = &tableaupb.Field_MapEntry{
		KeyType:       parsedKeyType,
		ValueType:     parsedValueName,
		ValueFullType: parsedValueFullName,
	}
	field.Options = &tableaupb.FieldOptions{
		Name: node.Name,
		Prop: ExtractMapFieldProp(prop),
	}
	field.Options.Key = keynameCell
	// struct map
	// auto append key to the first value struct field
	scalarField, err := parseField(p.parser.gen.typeInfos, keynameCell, desc.KeyType+desc.Prop.RawProp())
	if err != nil {
		return errWithNodeKV(err, typeNode,
			xerrors.KeyPBFieldType, desc.KeyType+" (map key)",
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}
	scalarField.Name = strcase.ToSnake(strings.TrimPrefix(node.GetMetaKey(), book.MetaSign))
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
	typeNode := node.GetMetaTypeNode()
	typeCell := typeNode.GetValue()
	variableCell := node.GetMetaVariable()
	desc := types.MatchList(typeCell)
	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return errWithNodeKV(err, typeNode,
			xerrors.KeyPBFieldType, desc.ElemType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}
	// whether layout is incell or not
	layout := tableaupb.Layout_LAYOUT_DEFAULT
	if desc.ElemType == "" || node.GetMetaIncell() {
		layout = tableaupb.Layout_LAYOUT_INCELL
	}
	elemType := desc.ElemType
	if desc.ElemType == "" {
		elemType = desc.ColumnType
	}
	scalarField, err := parseField(p.parser.gen.typeInfos, node.Name, elemType)
	if err != nil {
		return errWithNodeKV(err, typeNode,
			xerrors.KeyPBFieldType, desc.ElemType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
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
	field.Name = strcase.ToSnake(variableCell)
	field.Options = &tableaupb.FieldOptions{
		Name:   node.Name,
		Layout: layout,
		Prop:   ExtractStructFieldProp(prop),
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
	typeNode := node.GetMetaTypeNode()
	typeCell := typeNode.GetValue()
	desc := types.MatchStruct(typeCell)
	prop, err := desc.Prop.FieldProp()
	if err != nil {
		return errWithNodeKV(err, typeNode,
			xerrors.KeyPBFieldType, desc.StructType,
			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	}
	// whether layout is incell or not
	span := tableaupb.Span_SPAN_DEFAULT
	if node.GetMetaIncell() {
		span = tableaupb.Span_SPAN_INNER_CELL
	}
	parseStrictStructField := func(fieldNodes []*book.Node) error {
		scalarField, err := parseField(p.parser.gen.typeInfos, node.Name, desc.StructType)
		if err != nil {
			return errWithNodeKV(err, typeNode,
				xerrors.KeyPBFieldType, desc.StructType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
		}
		proto.Merge(field, scalarField)

		field.Name = strcase.ToSnake(node.Name)
		field.Options = &tableaupb.FieldOptions{
			Name: node.Name,
			Span: span,
			Prop: ExtractStructFieldProp(prop),
		}
		for _, child := range fieldNodes {
			if child.IsMeta() {
				continue
			}
			if err := p.parseSubField(field, child); err != nil {
				return err
			}
		}
		return nil
	}

	structNode := node.GetMetaStructNode()
	if structNode == nil {
		// strict struct
		fieldNodes := node.GetChildrenWithoutMeta()
		if len(fieldNodes) != 0 {
			return parseStrictStructField(fieldNodes)
		}

		// predefined struct
		if desc.ColumnType == "" {
			structField, err := parseField(p.parser.gen.typeInfos, node.Name, desc.StructType)
			if err != nil {
				return errWithNodeKV(err, typeNode,
					xerrors.KeyPBFieldType, desc.StructType,
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			proto.Merge(field, structField)
			field.Options.Span = span
			field.Options.Prop = ExtractStructFieldProp(prop)
			return nil
		}

		// inner cell struct
		fieldPairs, err := parseIncellStruct(desc.StructType)
		if err != nil {
			return err
		}
		if fieldPairs == nil {
			err := errors.Errorf("no fields defined in inner cell struct")
			return errWithNodeKV(err, typeNode,
				xerrors.KeyPBFieldType, desc.StructType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
		}
		scalarField, err := parseField(p.parser.gen.typeInfos, node.Name, desc.ColumnType)
		if err != nil {
			return errWithNodeKV(err, typeNode,
				xerrors.KeyPBFieldName, node.Name,
				xerrors.KeyPBFieldType, desc.ColumnType,
				xerrors.KeyPBFieldOpts, desc.Prop.Text)
		}
		proto.Merge(field, scalarField)
		field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
		field.Options.Prop = ExtractStructFieldProp(prop)

		for i := 0; i < len(fieldPairs); i += 2 {
			fieldType := fieldPairs[i]
			fieldName := fieldPairs[i+1]
			scalarField, err := parseField(p.parser.gen.typeInfos, fieldName, fieldType)
			if err != nil {
				return errWithNodeKV(err, typeNode,
					xerrors.KeyPBFieldName, fieldName,
					xerrors.KeyPBFieldType, fieldType,
					xerrors.KeyPBFieldOpts, desc.Prop.Text)
			}
			field.Fields = append(field.Fields, scalarField)
		}
		return nil
	} else {
		return parseStrictStructField(structNode.Children)
	}
}

func (p *documentBookParser) parseStrictStructField(field *tableaupb.Field, name string, desc *types.StructDescriptor, span tableaupb.Span, children []*book.Node) error {
	// typeNode := node.GetMetaTypeNode()
	// typeCell := typeNode.GetValue()
	// desc := types.MatchStruct(typeCell)
	// prop, err := desc.Prop.FieldProp()
	// if err != nil {
	// 	return errWithNodeKV(err, typeNode,
	// 		xerrors.KeyPBFieldType, desc.StructType,
	// 		xerrors.KeyPBFieldOpts, desc.Prop.Text)
	// }
	// // whether layout is incell or not
	// span := tableaupb.Span_SPAN_DEFAULT
	// if node.GetMetaIncell() {
	// 	span = tableaupb.Span_SPAN_INNER_CELL
	// }

	// structNode := node.GetMetaStructNode()
	// if structNode == nil {
	// 	if desc.ColumnType == "" {
	// 		// predefined struct
	// 		structField, err := parseField(p.parser.gen.typeInfos, node.Name, desc.StructType)
	// 		if err != nil {
	// 			return errWithNodeKV(err, typeNode,
	// 				xerrors.KeyPBFieldType, desc.StructType,
	// 				xerrors.KeyPBFieldOpts, desc.Prop.Text)
	// 		}
	// 		proto.Merge(field, structField)
	// 		field.Options.Span = span
	// 		field.Options.Prop = ExtractStructFieldProp(prop)
	// 		return nil
	// 	}
	// 	// inner cell struct
	// 	fieldPairs, err := parseIncellStruct(desc.StructType)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if fieldPairs == nil {
	// 		err := errors.Errorf("no fields defined in inner cell struct")
	// 		return errWithNodeKV(err, typeNode,
	// 			xerrors.KeyPBFieldType, desc.StructType,
	// 			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	// 	}
	// 	scalarField, err := parseField(p.parser.gen.typeInfos, node.Name, desc.ColumnType)
	// 	if err != nil {
	// 		return errWithNodeKV(err, typeNode,
	// 			xerrors.KeyPBFieldName, node.Name,
	// 			xerrors.KeyPBFieldType, desc.ColumnType,
	// 			xerrors.KeyPBFieldOpts, desc.Prop.Text)
	// 	}
	// 	proto.Merge(field, scalarField)
	// 	field.Options.Span = tableaupb.Span_SPAN_INNER_CELL
	// 	field.Options.Prop = ExtractStructFieldProp(prop)

	// 	for i := 0; i < len(fieldPairs); i += 2 {
	// 		fieldType := fieldPairs[i]
	// 		fieldName := fieldPairs[i+1]
	// 		scalarField, err := parseField(p.parser.gen.typeInfos, fieldName, fieldType)
	// 		if err != nil {
	// 			return errWithNodeKV(err, typeNode,
	// 				xerrors.KeyPBFieldName, fieldName,
	// 				xerrors.KeyPBFieldType, fieldType,
	// 				xerrors.KeyPBFieldOpts, desc.Prop.Text)
	// 		}
	// 		field.Fields = append(field.Fields, scalarField)
	// 	}
	// 	return nil
	// }
	// scalarField, err := parseField(p.parser.gen.typeInfos, name, desc.StructType)
	// if err != nil {
	// 	return err
	// }
	// proto.Merge(field, scalarField)

	// field.Name = strcase.ToSnake(name)
	// field.Options = &tableaupb.FieldOptions{
	// 	Name: name,
	// 	Span: span,
	// 	Prop: ExtractStructFieldProp(prop),
	// }
	// for _, child := range children {
	// 	if child.IsMeta() {
	// 		continue
	// 	}
	// 	if err := p.parseSubField(field, child); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}
