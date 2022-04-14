package xproto

import (
	"path/filepath"
	"strings"

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
	atom.Log.Debugf("import pathes: %v", importPaths)
	atom.Log.Debugf("filenames: %v", filenames)
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
		for _, originMatch := range matches {
			match, err := filepath.Abs(originMatch)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get absolute path for %s", match)
			}
			cleanSlashPath := fs.GetCleanSlashPath(match)
			for _, importPath := range importPaths {
				importPath, err := filepath.Abs(importPath)
				if err != nil {
					return nil, errors.Wrapf(err, "failed to get absolute path for %s", importPath)
				}
				importCleanSlashPath := fs.GetCleanSlashPath(importPath)
				if !strings.HasPrefix(cleanSlashPath, importCleanSlashPath) {
					atom.Log.Debugf("add proto file: %s", originMatch)
					protoFiles = append(protoFiles, originMatch)
				} else {
					protoFiles = append(protoFiles, strings.TrimPrefix(cleanSlashPath, importCleanSlashPath+"/"))
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
