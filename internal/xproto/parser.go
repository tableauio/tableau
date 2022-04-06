package xproto

import (
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	_ "github.com/tableauio/tableau/proto/tableaupb"
)

func ParseProtos(ImportPaths []string, filenames ...string) ([]*desc.FileDescriptor, error) {
	parser := &protoparse.Parser{
		ImportPaths: ImportPaths,
		LookupImport: desc.LoadFileDescriptor,
	}
	return parser.ParseFiles(filenames...)
}
