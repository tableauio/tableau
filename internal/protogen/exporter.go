package protogen

import (
	"path/filepath"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/rogpeppe/go-internal/lockedfile"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/printer"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type bookExporter struct {
	ProtoPackage     string
	ProtoFileOptions map[string]string
	OutputDir        string
	FilenameSuffix   string
	wb               *tableaupb.Workbook

	gen *Generator
}

func newBookExporter(protoPackage string, protoFileOptions map[string]string, outputDir, filenameSuffix string, wb *tableaupb.Workbook, gen *Generator) *bookExporter {
	return &bookExporter{
		ProtoPackage:     protoPackage,
		ProtoFileOptions: protoFileOptions,
		OutputDir:        outputDir,
		FilenameSuffix:   filenameSuffix,
		wb:               wb,
		gen:              gen,
	}
}

func (x *bookExporter) GetProtoFilePath() string {
	return genProtoFilePath(x.wb.Name, x.FilenameSuffix)
}

func (x *bookExporter) export(checkProtoFileConflicts bool) error {
	// log.Debug(proto.MarshalTextString(wb))
	g1 := NewGeneratedBuf()
	g1.P("// Code generated by tableau (protogen v", Version, "). DO NOT EDIT.")
	g1.P("// clang-format off")
	g1.P("")
	g1.P(`syntax = "proto3";`)
	g1.P("")
	g1.P("package ", x.ProtoPackage, ";")
	g1.P("")

	// keep the elements ordered by import path
	set := treeset.NewWithStringComparator()
	set.Add(tableauProtoPath) // default must be imported path
	g3 := NewGeneratedBuf()
	for i, ws := range x.wb.Worksheets {
		x := &sheetExporter{
			be:             x,
			ws:             ws,
			g:              g3,
			isLastSheet:    i == len(x.wb.Worksheets)-1,
			typeInfos:      x.gen.typeInfos,
			nestedMessages: make(map[string]*tableaupb.Field),
			Imports:        make(map[string]bool),
		}
		if err := x.export(); err != nil {
			return err
		}
		for key := range x.Imports {
			set.Add(key)
		}
	}

	// generate imports
	g2 := NewGeneratedBuf()
	for _, key := range set.Values() {
		g2.P(`import "`, key, `";`)
	}
	g2.P("")
	for k, v := range x.ProtoFileOptions {
		g2.P(`option `, k, ` = "`, v, `";`)
	}
	g2.P("option (tableau.workbook) = {", marshalToText(x.wb.Options), "};")
	g2.P("")

	relPath := x.GetProtoFilePath()
	path := filepath.Join(x.OutputDir, relPath)
	log.Infof("%18s: %s", "generated proto", relPath)

	// mu := lockedfile.MutexAt(path)
	// unlock, err := mu.Lock()
	// if err != nil {
	// 	return errors.Wrapf(err, "failed to lock file: %s", path)
	// }
	// defer unlock()

	// NOTE: use file lock to protect .proto file from being writen by multiple goroutines
	// refer: https://github.com/golang/go/issues/33974
	// refer: https://go.googlesource.com/proposal/+/master/design/33974-add-public-lockedfile-pkg.md

	if checkProtoFileConflicts {
		if existed, err := fs.Exists(path); err != nil {
			return xerrors.WrapKV(err)
		} else {
			if existed {
				return xerrors.Errorf("file already exists: %s", path)
			}
		}
	}

	if f, err := lockedfile.Create(path); err != nil {
		return xerrors.WrapKV(err)
	} else {
		defer f.Close()
		if _, err = f.Write(g1.Content()); err != nil {
			return xerrors.WrapKV(err)
		}
		if _, err = f.Write(g2.Content()); err != nil {
			return xerrors.WrapKV(err)
		}
		if _, err = f.Write(g3.Content()); err != nil {
			return xerrors.WrapKV(err)
		}
	}

	return nil
}

type sheetExporter struct {
	be          *bookExporter
	ws          *tableaupb.Worksheet
	g           *GeneratedBuf
	isLastSheet bool
	typeInfos   *xproto.TypeInfos

	Imports        map[string]bool             // import name -> defined
	nestedMessages map[string]*tableaupb.Field // top message scoped type name -> field
}

func (x *sheetExporter) export() error {
	mode := x.ws.GetOptions().GetMode()
	switch x.ws.Options.Mode {
	case tableaupb.Mode_MODE_DEFAULT:
		return x.exportMessager()
	case tableaupb.Mode_MODE_ENUM_TYPE:
		return x.exportEnum()
	case tableaupb.Mode_MODE_STRUCT_TYPE:
		return x.exportStruct()
	case tableaupb.Mode_MODE_UNION_TYPE:
		return x.exportUnion()
	default:
		return errors.Errorf("unknown mode: %d", mode)
	}
}

func (x *sheetExporter) exportEnum() error {
	x.g.P("// Generated from sheet: ", x.ws.GetOptions().GetName(), ".")
	x.g.P("enum ", x.ws.Name, " {")
	// generate the enum value fields
	for i, field := range x.ws.Fields {
		if i == 0 && field.Number != 0 {
			ename := strcase.ToScreamingSnake(x.ws.Name) + "_INVALID"
			x.g.P("  ", ename, " = 0;")
		}
		x.g.P("  ", field.Name, " = ", field.Number, ` [(tableau.evalue).name = "`, field.Alias, `"];`)
	}
	x.g.P("}")
	if !x.isLastSheet {
		x.g.P("")
	}
	return nil
}

func (x *sheetExporter) exportStruct() error {
	x.g.P("// Generated from sheet: ", x.ws.GetOptions().GetName(), ".")
	x.g.P("message ", x.ws.Name, " {")
	// generate the fields
	depth := 1
	for i, field := range x.ws.Fields {
		tagid := i + 1
		if err := x.exportField(depth, tagid, field, x.ws.Name); err != nil {
			return err
		}
	}
	x.g.P("}")
	if !x.isLastSheet {
		x.g.P("")
	}
	return nil
}

func (x *sheetExporter) exportUnion() error {
	x.g.P("// Generated from sheet: ", x.ws.GetOptions().GetName(), ".")
	x.g.P("message ", x.ws.Name, " {")
	x.g.P(`  option (tableau.union) = true;`)
	x.g.P()
	x.g.P(`  Type type = 9999 [(tableau.field) = { name: "Type" }];`)
	x.g.P(`  oneof value {`)
	x.g.P(`    option (tableau.oneof) = {field: "Field"};`)
	x.g.P()
	for _, field := range x.ws.Fields {
		ename := "TYPE_" + strcase.ToScreamingSnake(field.Name)
		x.g.P("    ", field.Name, " ", strcase.ToSnake(field.Name), " = ", field.Number, `; // Bound to enum value: `, ename, ".")
	}
	x.g.P(`  }`)

	// generate enum type
	x.g.P("  enum Type {")
	x.g.P("    TYPE_INVALID = 0;")
	for _, field := range x.ws.Fields {
		ename := "TYPE_" + strcase.ToScreamingSnake(field.Name)
		x.g.P("    ", ename, " = ", field.Number, ` [(tableau.evalue).name = "`, field.Alias, `"];`)
	}
	x.g.P("  }")
	x.g.P()

	// generate message type
	for _, msgField := range x.ws.Fields {
		x.g.P("  message ", msgField.Name, " {")
		// generate the fields
		depth := 2
		for i, field := range msgField.Fields {
			tagid := i + 1
			if err := x.exportField(depth, tagid, field, msgField.Name); err != nil {
				return err
			}
		}
		x.g.P("  }")
	}

	x.g.P("}")
	if !x.isLastSheet {
		x.g.P("")
	}
	return nil
}

func (x *sheetExporter) exportMessager() error {
	// log.Debugf("workbook: %s", x.ws.String())
	x.g.P("message ", x.ws.Name, " {")
	x.g.P("  option (tableau.worksheet) = {", marshalToText(x.ws.Options), "};")
	x.g.P("")
	// generate the fields
	depth := 1
	for i, field := range x.ws.Fields {
		tagid := i + 1
		if err := x.exportField(depth, tagid, field, x.ws.Name); err != nil {
			return err
		}
	}
	x.g.P("}")
	if !x.isLastSheet {
		x.g.P("")
	}
	return nil
}

func (x *sheetExporter) exportField(depth int, tagid int, field *tableaupb.Field, prefix string) error {
	label := ""
	if x.ws.GetOptions().GetFieldPresence() &&
		types.IsScalarType(field.FullType) &&
		!types.IsWellKnownMessage(field.FullType) {
		label = "optional "
	}

	x.g.P(printer.Indent(depth), label, field.FullType, " ", field.Name, " = ", tagid, " ", genFieldOptionsString(field.Options), ";")

	typeName := field.Type
	fullTypeName := field.FullType
	if field.ListEntry != nil {
		typeName = field.ListEntry.ElemType
		fullTypeName = field.ListEntry.ElemFullType
	}
	if field.MapEntry != nil {
		typeName = field.MapEntry.ValueType
		fullTypeName = field.MapEntry.ValueFullType
	}

	if fullTypeName == types.WellKnownMessageTimestamp {
		x.Imports[timestampProtoPath] = true
	} else if fullTypeName == types.WellKnownMessageDuration {
		x.Imports[durationProtoPath] = true
	}

	if field.Predefined {
		// import the predefined type's parent filename.
		// NOTE: excludes self.
		if typeInfo := x.typeInfos.GetByFullName(protoreflect.FullName(fullTypeName)); typeInfo != nil &&
			typeInfo.ParentFilename != x.be.GetProtoFilePath() {
			x.Imports[typeInfo.ParentFilename] = true
		}
	} else {
		nestedMsgName := prefix + "." + typeName
		switch {
		case field.Fields != nil:
			// iff field is a map or list and message type is not imported.
			if isSameFieldMessageType(field, x.nestedMessages[nestedMsgName]) {
				// if the nested message is the same as the previous one,
				// just use the previous one, and don't generate a new one.
				return nil
			}
		case !types.IsScalarType(typeName):
			if _, ok := x.nestedMessages[nestedMsgName]; ok {
				// if the nested message has the same name with the previous one,
				// just use the previous one, and don't generate a new one.
				return nil
			}
		default:
			return nil
		}
		// bookkeeping this nested msessage, so we can check if we can reuse it later.
		x.nestedMessages[nestedMsgName] = field

		// x.g.P("")
		x.g.P(printer.Indent(depth), "message ", typeName, " {")
		for i, f := range field.Fields {
			tagid := i + 1
			if err := x.exportField(depth+1, tagid, f, nestedMsgName); err != nil {
				return err
			}
		}
		x.g.P(printer.Indent(depth), "}")
	}
	return nil
}

func genFieldOptionsString(opts *tableaupb.FieldOptions) string {
	jsonName := ""
	// remember and then clear protobuf built-in options
	if opts.Prop != nil {
		jsonName = opts.Prop.JsonName
		opts.Prop.JsonName = ""

		// set nil if field prop is empty
		if IsEmptyFieldProp(opts.Prop) {
			opts.Prop = nil
		}
	}

	// compose this field options
	fieldOpts := "[(tableau.field) = {" + marshalToText(opts) + "}"
	if jsonName != "" {
		fieldOpts += `, json_name="` + jsonName + `"`
	}
	fieldOpts += "]"
	return fieldOpts
}

func marshalToText(m protoreflect.ProtoMessage) string {
	// text := proto.CompactTextString(field.Options)
	bin, err := prototext.Marshal(m)
	if err != nil {
		panic(err)
	}
	// NOTE: remove redundant spaces/whitespace from a string
	// refer: https://stackoverflow.com/questions/37290693/how-to-remove-redundant-spaces-whitespace-from-a-string-in-golang
	text := strings.Join(strings.Fields(string(bin)), " ")
	return text
}

func isSameFieldMessageType(left, right *tableaupb.Field) bool {
	if left == nil || right == nil {
		return false
	}
	if left.Fields == nil || right.Fields == nil {
		return false
	}
	if len(left.Fields) != len(right.Fields) ||
		left.Type != right.Type ||
		left.FullType != right.FullType {
		return false
	}

	for i, l := range left.Fields {
		r := right.Fields[i]
		if !proto.Equal(l, r) {
			return false
		}
	}
	return true
}
