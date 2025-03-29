package protogen

import (
	"path/filepath"
	"strings"

	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/xerrors"
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
	filename := gen.strcaseCtx.ToSnake(protoBookName)
	if gen.OutputOpt.FilenameWithSubdirPrefix {
		bookPath := filepath.Join(filepath.Dir(relSlashPath), protoBookName)
		snakePath := gen.strcaseCtx.ToSnake(xfs.CleanSlashPath(bookPath))
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

// parseBasicField parses scalar or enum type.
func (p *bookParser) parseBasicField(name, typ string) (*internalpb.Field, error) {
	return parseBasicField(p.gen.typeInfos, p.gen.strcaseCtx, name, typ)
}

// parseBasicField parses scalar or enum type.
func parseBasicField(typeInfos *xproto.TypeInfos, strcaseCtx strcase.Context, name, typ string) (*internalpb.Field, error) {
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
	pureName := strings.TrimPrefix(name, book.MetaSign) // remove leading meta sign "@""
	return &internalpb.Field{
		Name:       strcaseCtx.ToSnake(pureName),
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
