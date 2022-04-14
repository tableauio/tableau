package xproto

import (
	"path/filepath"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/fs"
	_ "github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ParseProtos parses the import paths and proto Glob filenames to desc.FileDescriptor slices.
func ParseProtos(importPaths []string, filenames ...string) ([]*desc.FileDescriptor, error) {
	parser := &protoparse.Parser{
		ImportPaths:  importPaths,
		LookupImport: desc.LoadFileDescriptor,
	}

	return parser.ParseFiles(filenames...)
}

// NewFiles creates a new protoregistry.Files from the import paths and proto filenames.
func NewFiles(importPaths []string, filenames ...string) (*protoregistry.Files, error) {
	var protoFiles []string
	for _, filename := range filenames {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := fs.GetCleanSlashPath(match)
			for _, importPath := range importPaths {
				importCleanSlashPath := fs.GetCleanSlashPath(importPath)
				rel, err := filepath.Rel(importCleanSlashPath, cleanSlashPath)
				if err != nil {
					protoFiles = append(protoFiles, match)
				} else {
					atom.Log.Debugf("convert rel: %s -> %s", match, rel)
					protoFiles = append(protoFiles, rel)
				}
			}
		}
	}

	descFileDescriptors, err := ParseProtos(importPaths, protoFiles...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse protos")
	}

	fds := desc.ToFileDescriptorSet(descFileDescriptors...)
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to creates a new protoregistry.Files from the provided FileDescriptorSet message")
	}
	return files, nil
}
