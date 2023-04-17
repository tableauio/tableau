package xproto

import (
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	_ "github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
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
				for _, protoPath := range protoPaths {
					cleanProtoPath := fs.GetCleanSlashPath(protoPath) + "/"
					cleanSlashPath = strings.TrimPrefix(cleanSlashPath, cleanProtoPath)
				}
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

	FirstFieldOptionName string // only for MessageKind
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
	log.Debugf("remember new generated predefined type: %v", info)
	x.infos[info.FullName] = info
}

// Get retrieves type info by name in proto package.
// It will auto prepend proto package to inputed name to
// generate the full name of type.
func (x *TypeInfos) Get(name string) *TypeInfo {
	fullName := x.protoPackage + "." + name
	return x.GetByFullName(fullName)
}

// GetByFullName retrieves type info by type's full name.
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
	// find first field option name
	firstFieldOptionName := ""
	if IsUnion(md) {
		desc := ExtractUnionDescriptor(md)
		if desc != nil {
			// union's first field is enum type field.
			fieldOpts := proto.GetExtension(desc.Type.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
			firstFieldOptionName = fieldOpts.GetName()
		}
	} else if md.Fields().Len() != 0 {
		// struct's first field
		fd := md.Fields().Get(0)
		fieldOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
		firstFieldOptionName = fieldOpts.GetName()
	}
	info := &TypeInfo{
		FullName:             string(md.FullName()),
		ParentFilename:       md.ParentFile().Path(),
		Kind:                 types.MessageKind,
		FirstFieldOptionName: firstFieldOptionName,
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
