package xproto

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
)

func ParseProtos(ImportPaths []string, filenames ...string) ([]*desc.FileDescriptor, error) {
	parser := &protoparse.Parser{
		ImportPaths: ImportPaths,
		// LookupImportProto: func(importPath string) (*dpb.FileDescriptorProto, error) {
		// 	atom.Log.Debugf("importPath: %s", importPath)
		// 	switch importPath {
		// 	case "tableau/protobuf/tableau.proto":
		// 		atom.Log.Debugf("ImportPath: %s", importPath)
		// 		return protodesc.ToFileDescriptorProto(tableaupb.File_tableau_proto), nil
		// 	case "tableau/protobuf/meta.proto":
		// 		return protodesc.ToFileDescriptorProto(tableaupb.File_meta_proto), nil
		// 	case "tableau/protobuf/workbook.proto":
		// 		return protodesc.ToFileDescriptorProto(tableaupb.File_workbook_proto), nil
		// 	default:
		// 		return nil, errors.New("not found")
		// 	}
		// },
	}
	return parser.ParseFiles(filenames...)
}
