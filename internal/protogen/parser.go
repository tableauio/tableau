package protogen

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
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
	wb  *internalpb.Workbook
}

func newBookParser(bookName, alias, relSlashPath string, gen *Generator) *bookParser {
	// log.Debugf("filenameWithSubdirPrefix: %v", filenameWithSubdirPrefix)
	protoBookName := bookName // generated proto book file name
	if alias != "" {
		protoBookName = alias
	}
	filename := strcase.FromContext(gen.ctx).ToSnake(protoBookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), protoBookName)
		snakePath := strcase.FromContext(gen.ctx).ToSnake(xfs.CleanSlashPath(bookPath))
		filename = strings.ReplaceAll(snakePath, "/", "__")
	}
	// sep and subsep
	var sep, subsep string
	if gen.InputOpt.Header != nil {
		sep = gen.InputOpt.Header.Sep
		subsep = gen.InputOpt.Header.Subsep
	}
	if sep == "" {
		sep = options.DefaultSep
	}
	if subsep == "" {
		subsep = options.DefaultSubsep
	}
	bp := &bookParser{
		gen: gen,
		wb: &internalpb.Workbook{
			Options: &tableaupb.WorkbookOptions{
				// NOTE(wenchy): all OS platforms use path slash separator `/`
				// see: https://stackoverflow.com/questions/9371031/how-do-i-create-crossplatform-file-paths-in-go
				Name:   relSlashPath,
				Alias:  alias,
				Sep:    sep,
				Subsep: subsep,
			},
			Worksheets: []*internalpb.Worksheet{},
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

// parseBasicField parses scalar, enum, and wellknown message types.
func (p *bookParser) parseBasicField(name, typ, note string) (*internalpb.Field, error) {
	return parseBasicField(p.gen.ctx, p.gen.typeInfos, name, typ, note)
}

// parseBasicField parses scalar, enum, and wellknown message types.
func parseBasicField(ctx context.Context, typeInfos *xproto.TypeInfos, name, typ, note string) (*internalpb.Field, error) {
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
		return nil, xerrors.WrapKV(err,
			xerrors.KeyPBFieldOpts, prop.Text,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}

	fieldProp, err := prop.FieldProp()
	if err != nil {
		return nil, xerrors.WrapKV(err,
			xerrors.KeyPBFieldOpts, prop.Text,
			xerrors.KeyPBFieldType, typ,
			xerrors.KeyTrimmedNameCell, name)
	}
	pureName := strings.TrimPrefix(name, book.MetaSign) // remove leading meta sign "@"
	return &internalpb.Field{
		Name:       strcase.FromContext(ctx).ToSnake(pureName),
		Type:       typeDesc.Name,
		FullType:   typeDesc.FullName,
		Note:       strings.TrimSpace(note),
		Predefined: typeDesc.Predefined,
		Options: &tableaupb.FieldOptions{
			Name: name,
			// Currently, there is no need to set note, but maybe in the future,
			// we want to get note by protobuf reflection, then should set it.
			Note: "",
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
			return nil, xerrors.Newf("predefined type not found: %s", rawType)
		}
	}
	return types.ParseTypeDescriptor(rawType), nil
}

// parseIncellStructField parses one field of an incell struct definition.
// It accepts scalar (including well-known types, e.g. datetime, duration,
// fraction, comparator, version) and enum types, and also a "repeated" form
// `[]ElemType` which produces a repeated proto field. The generated field
// name is auto-suffixed with "_list", so users should declare the singular
// form (e.g. `[]int32 ID` generates `repeated int32 id_list`).
func (p *bookParser) parseIncellStructField(name, typ, note string) (*internalpb.Field, error) {
	return parseIncellStructField(p.gen.ctx, p.gen.typeInfos, name, typ, note)
}

func parseIncellStructField(ctx context.Context, typeInfos *xproto.TypeInfos, name, typ, note string) (*internalpb.Field, error) {
	if !strings.HasPrefix(typ, "[]") {
		return parseBasicField(ctx, typeInfos, name, typ, note)
	}
	elemType := strings.TrimSpace(typ[2:])
	if elemType == "" {
		return nil, xerrors.Newf("empty element type in incell struct repeated field: %s", typ)
	}
	if strings.ContainsAny(elemType, "[{") {
		return nil, xerrors.Newf("nested composite type is not allowed in incell struct repeated field: %s", typ)
	}
	// Element kind check: only scalar or enum is allowed. Well-known types
	// (e.g. datetime, duration, fraction, comparator, version) are already
	// classified as types.ScalarKind by types.ParseTypeDescriptor, so they
	// are supported here too.
	elemTypeForDesc := elemType
	if desc := types.MatchEnum(elemTypeForDesc); desc != nil {
		elemTypeForDesc = desc.EnumType
	} else if desc := types.MatchScalar(elemTypeForDesc); desc != nil {
		elemTypeForDesc = desc.ScalarType
	}
	elemDesc, err := parseTypeDescriptor(typeInfos, elemTypeForDesc)
	if err != nil {
		return nil, err
	}
	if elemDesc.Kind != types.ScalarKind && elemDesc.Kind != types.EnumKind {
		return nil, xerrors.Newf("only scalar (including well-known types) or enum element type is allowed in incell struct repeated field: %s", typ)
	}
	// Reuse parseBasicField to build the element field, then promote it to a
	// repeated field by setting ListEntry and adjusting Type/FullType.
	field, err := parseBasicField(ctx, typeInfos, name, elemType, note)
	if err != nil {
		return nil, err
	}
	field.ListEntry = &internalpb.Field_ListEntry{
		ElemType:     field.Type,
		ElemFullType: field.FullType,
	}
	field.Type = "repeated " + field.Type
	field.FullType = "repeated " + field.FullType
	// Auto-append "_list" suffix so users don't need to pluralize the name.
	field.Name = strcase.FromContext(ctx).ToSnake(strings.TrimPrefix(name, book.MetaSign)) + listVarSuffix
	return field, nil
}

// parseIncellStruct parses incell struct type definition. For example:
//   - int32 ID
//   - int32 ID, string Name
//   - []int32 ID, string Name (repeated field, generates `id_list`)
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
			return nil, xerrors.Newf("illegal type-variable pair: %v in incell struct: %s", pair, structType)
		}
		fieldPairs = append(fieldPairs, kv...)
	}
	return fieldPairs, nil
}
