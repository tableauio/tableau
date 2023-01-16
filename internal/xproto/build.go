package xproto

import (
	"path/filepath"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	_ "github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// ParseProtos parses the proto paths and proto files to desc.FileDescriptor slices.
func ParseProtos(protoPaths []string, protoFiles ...string) (*protoregistry.Files, error) {
	log.Debugf("proto paths: %v", protoPaths)
	log.Debugf("proto files: %v", protoFiles)
	parser := &protoparse.Parser{
		ImportPaths:  protoPaths,
		LookupImport: desc.LoadFileDescriptor,
	}

	fileDescs, err := parser.ParseFiles(protoFiles...)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ParseFiles from proto files")
	}
	fds := desc.ToFileDescriptorSet(fileDescs...)
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to creates a new protoregistry.Files from the provided FileDescriptorSet message")
	}
	return files, nil
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
		// 			log.Debugf("add proto file: %s", originMatch)
		// 			parsedProtoFiles = append(parsedProtoFiles, originMatch)
		// 		} else {
		// 			parsedProtoFiles = append(parsedProtoFiles, strings.TrimPrefix(cleanSlashPath, importCleanSlashPath+"/"))
		// 		}
		// 	}
		// }
	}

	log.Debugf("proto files: %v", parsedProtoFiles)

	return ParseProtos(protoPaths, parsedProtoFiles...)
}

type TypeInfo struct {
	FullName       string
	ParentFilename string
	Kind           types.Kind
}

func NewTypeInfos(protoPackage string) *TypeInfos {
	return &TypeInfos{
		protoPackage: protoPackage,
		infos:        map[string]*TypeInfo{},
	}
}

type TypeInfos struct {
	protoPackage string
	infos        map[string]*TypeInfo // full name -> type info
}

func (x *TypeInfos) Put(info *TypeInfo) {
	x.infos[info.FullName] = info
}

// Get retrieves type info by name in proto package.
// It will auto prepend proto package to inputed name to 
// generate the full name of type. 
func (x *TypeInfos) Get(name string) *TypeInfo {
	fullName := x.protoPackage + "." + name
	return x.GetByFullName(fullName)
}

func (x *TypeInfos) GetByFullName(fullName string) *TypeInfo {
	return x.infos[fullName]
}

func GetAllTypeInfo(files *protoregistry.Files, protoPackage string) *TypeInfos {
	typeInfos := NewTypeInfos(protoPackage)
	files.RangeFilesByPackage(protoreflect.FullName(protoPackage), func(fileDesc protoreflect.FileDescriptor) bool {
		extractTypeInfos(fileDesc.Messages(), typeInfos)
		for i := 0; i < fileDesc.Enums().Len(); i++ {
			ed := fileDesc.Enums().Get(i)
			info := &TypeInfo{
				FullName:       string(ed.FullName()),
				ParentFilename: ed.ParentFile().Path(),
				Kind:           types.EnumKind,
			}
			typeInfos.Put(info)
		}
		return true
	})
	return typeInfos
}

// extractTypeInfosRecursively extracts all type infos (including nested types)
// from message descriptors recursively.
func extractTypeInfos(mds protoreflect.MessageDescriptors, typeInfos *TypeInfos) {
	for i := 0; i < mds.Len(); i++ {
		extractTypeInfosFromMessage(mds.Get(i), typeInfos)
	}
}

func extractTypeInfosFromMessage(md protoreflect.MessageDescriptor, typeInfos *TypeInfos) {
	if md.IsMapEntry() {
		// ignore auto-generated message type to
		// represent the entry type for a map field.
		return
	}
	info := &TypeInfo{
		FullName:       string(md.FullName()),
		ParentFilename: md.ParentFile().Path(),
		Kind:           types.MessageKind,
	}
	typeInfos.Put(info)

	for i := 0; i < md.Enums().Len(); i++ {
		ed := md.Enums().Get(i)
		info := &TypeInfo{
			FullName:       string(ed.FullName()),
			ParentFilename: ed.ParentFile().Path(),
			Kind:           types.EnumKind,
		}
		typeInfos.Put(info)
	}
	// nested types
	for i := 0; i < md.Messages().Len(); i++ {
		subMD := md.Messages().Get(i)
		extractTypeInfosFromMessage(subMD, typeInfos)
	}
}
