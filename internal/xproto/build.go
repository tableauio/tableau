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

// ParseProtos parses the proto paths and proto files to desc.FileDescriptor slices.
func ParseProtos(protoPaths []string, protoFiles ...string) ([]*desc.FileDescriptor, error) {
	atom.Log.Debugf("proto paths: %v", protoPaths)
	atom.Log.Debugf("proto files: %v", protoFiles)
	parser := &protoparse.Parser{
		ImportPaths:  protoPaths,
		LookupImport: desc.LoadFileDescriptor,
	}

	return parser.ParseFiles(protoFiles...)
}

// NewFiles creates a new protoregistry.Files from the proto paths and proto Gob filenames.
func NewFiles(protoPaths []string, protoFiles []string, excludeProtoFiles ...string) (*protoregistry.Files, error) {
	 parsedExcludedProtoFiles := map[string]bool{}
	for _, filename := range excludeProtoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := fs.GetCleanSlashPath(match)
			parsedExcludedProtoFiles[cleanSlashPath] = true
		}
	}
	var parsedProtoFiles []string
	for _, filename := range protoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := fs.GetCleanSlashPath(match)
			if !parsedExcludedProtoFiles[cleanSlashPath] {
				parsedProtoFiles = append(parsedProtoFiles, cleanSlashPath)
			}
		}

		// for _, originMatch := range matches {
		// 	match, err := filepath.Abs(originMatch)
		// 	if err != nil {
		// 		return nil, errors.Wrapf(err, "failed to get absolute path for %s", match)
		// 	}
		// 	cleanSlashPath := fs.GetCleanSlashPath(match)
		// 	for _, importPath := range protoPaths {
		// 		importPath, err := filepath.Abs(importPath)
		// 		if err != nil {
		// 			return nil, errors.Wrapf(err, "failed to get absolute path for %s", importPath)
		// 		}
		// 		importCleanSlashPath := fs.GetCleanSlashPath(importPath)
		// 		if !strings.HasPrefix(cleanSlashPath, importCleanSlashPath) {
		// 			atom.Log.Debugf("add proto file: %s", originMatch)
		// 			parsedProtoFiles = append(parsedProtoFiles, originMatch)
		// 		} else {
		// 			parsedProtoFiles = append(parsedProtoFiles, strings.TrimPrefix(cleanSlashPath, importCleanSlashPath+"/"))
		// 		}
		// 	}
		// }
	}

	atom.Log.Debugf("proto files: %v", parsedProtoFiles)

	descFileDescriptors, err := ParseProtos(protoPaths, parsedProtoFiles...)
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

type TypeInfo struct {
	Fullname       string
	ParentFilename string
}

func GetAllTypeInfo(fileDescs []*desc.FileDescriptor) map[string]*TypeInfo {
	typeInfos := make(map[string]*TypeInfo)
	for _, fileDesc := range fileDescs {
		for _, mt := range fileDesc.GetMessageTypes() {
			typeInfos[mt.GetName()] = &TypeInfo{
				Fullname:       mt.GetFullyQualifiedName(),
				ParentFilename: fileDesc.GetName(),
			}
		}
		for _, mt := range fileDesc.GetEnumTypes() {
			typeInfos[mt.GetName()] = &TypeInfo{
				Fullname:       mt.GetFullyQualifiedName(),
				ParentFilename: fileDesc.GetName(),
			}
		}
	}
	return typeInfos
}
