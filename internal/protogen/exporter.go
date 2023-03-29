package protogen

import (
	"path/filepath"
	"strings"

	"github.com/emirpasic/gods/sets/treeset"
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
	ProtoPackage        string
	ProtoFileOptions    map[string]string
	OutputDir           string
	ProtoFilenameSuffix string
	wb                  *tableaupb.Workbook

	gen *Generator
}

func newBookExporter(protoPackage string, protoFileOptions map[string]string, outputDir, protoFilenameSuffix string, wb *tableaupb.Workbook, gen *Generator) *bookExporter {
	return &bookExporter{
		ProtoPackage:        protoPackage,
		ProtoFileOptions:    protoFileOptions,
		OutputDir:           outputDir,
		ProtoFilenameSuffix: protoFilenameSuffix,
		wb:                  wb,
		gen:                 gen,
	}
}

func (x *bookExporter) export(checkProtoFileConflicts bool) error {
	// log.Debug(proto.MarshalTextString(wb))
	g1 := NewGeneratedBuf()
	g1.P("// Code generated by tableau (", AppVersion(), "). DO NOT EDIT.")
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

	relPath := x.wb.Name + x.ProtoFilenameSuffix + ".proto"
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
	ws          *tableaupb.Worksheet
	g           *GeneratedBuf
	isLastSheet bool
	typeInfos   *xproto.TypeInfos

	Imports        map[string]bool             // import name -> defined
	nestedMessages map[string]*tableaupb.Field // type name -> field
}

func (x *sheetExporter) export() error {
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
	if x.ws.Options != nil && x.ws.Options.FieldPresence && types.IsScalarType(field.FullType) {
		label = "optional "
	}

	jsonName := ""
	// remember and then clear protobuf built-in options
	if field.Options.Prop != nil {
		jsonName = field.Options.Prop.JsonName
		field.Options.Prop.JsonName = ""

		// set nil if field prop is empty
		if IsEmptyFieldProp(field.Options.Prop) {
			field.Options.Prop = nil
		}
	}

	// compose this field options
	fieldOpt := " [(tableau.field) = {" + marshalToText(field.Options) + "}"
	if jsonName != "" {
		fieldOpt += `, json_name="` + jsonName + `"`
	}
	fieldOpt += "]"

	x.g.P(printer.Indent(depth), label, field.FullType, " ", field.Name, " = ", tagid, fieldOpt, ";")

	// if field.FullType == "google.protobuf.Timestamp" {
	// 	x.Imports[timestampProtoPath] = true
	// } else if field.FullType == "google.protobuf.Duration" {
	// 	x.Imports[durationProtoPath] = true
	// }

	typeName := field.Type
	if field.ListEntry != nil {
		typeName = field.ListEntry.ElemType
	}
	if field.MapEntry != nil {
		typeName = field.MapEntry.ValueType
	}

	if typeName == "google.protobuf.Timestamp" {
		x.Imports[timestampProtoPath] = true
	} else if typeName == "google.protobuf.Duration" {
		x.Imports[durationProtoPath] = true
	}

	if field.Predefined {
		// NOTE: import corresponding message's custom defined proto file
		if typeInfo := x.typeInfos.Get(typeName); typeInfo != nil {
			x.Imports[typeInfo.ParentFilename] = true
		}
	} else {
		if field.Fields != nil {
			// iff field is a map or list and message type is not imported.
			nestedMsgName := prefix + "." + typeName
			if isSameFieldMessageType(field, x.nestedMessages[nestedMsgName]) {
				// if the nested message is the same as the previous one,
				// just use the previous one, and don't generate a new one.
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
	}
	return nil
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
