package xproto

import (
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
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
		return nil, xerrors.Wrapf(err, "failed to ParseFiles from proto files")
	}
	fds := desc.ToFileDescriptorSet(fileDescs...)
	files, err := protodesc.NewFiles(fds)
	if err != nil {
		return nil, xerrors.Wrapf(err, "failed to creates a new protoregistry.Files from the provided FileDescriptorSet message")
	}
	return files, nil
}

// NewFiles creates a new protoregistry.Files from the proto paths and proto Gob filenames.
func NewFiles(protoPaths []string, protoFiles []string, excludeProtoFiles ...string) (*protoregistry.Files, error) {
	parsedExcludedProtoFiles := map[string]bool{}
	for _, filename := range excludeProtoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			parsedExcludedProtoFiles[cleanSlashPath] = true
		}
	}
	var parsedProtoFiles []string
	for _, filename := range protoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			if !parsedExcludedProtoFiles[cleanSlashPath] {
				for _, protoPath := range protoPaths {
					cleanProtoPath := xfs.CleanSlashPath(protoPath) + "/"
					cleanSlashPath = strings.TrimPrefix(cleanSlashPath, cleanProtoPath)
				}
				parsedProtoFiles = append(parsedProtoFiles, cleanSlashPath)
			}
		}

		// for _, originMatch := range matches {
		// 	match, err := filepath.Abs(originMatch)
		// 	if err != nil {
		// 		return nil, xerrors.Wrapf(err, "failed to get absolute path for %s", match)
		// 	}
		// 	cleanSlashPath := xfs.CleanSlashPath(match)
		// 	for _, importPath := range protoPaths {
		// 		importPath, err := filepath.Abs(importPath)
		// 		if err != nil {
		// 			return nil, xerrors.Wrapf(err, "failed to get absolute path for %s", importPath)
		// 		}
		// 		importCleanSlashPath := xfs.CleanSlashPath(importPath)
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
	FullName       protoreflect.FullName
	ParentFilename string
	Kind           types.Kind

	FirstFieldOptionName string // only for MessageKind
}

func NewTypeInfos(protoPackage string) *TypeInfos {
	return &TypeInfos{
		protoPackage: protoPackage,
		infos:        map[protoreflect.FullName]*TypeInfo{},
	}
}

type TypeInfos struct {
	protoPackage string
	infos        map[protoreflect.FullName]*TypeInfo // full name -> type info
}

func (x *TypeInfos) Put(info *TypeInfo) {
	log.Debugf("remember new generated predefined type: %v", info)
	x.infos[info.FullName] = info
}

// Get retrieves type info by name in proto package.
//
// NOTE: if name is prefixed with ".", then default proto package name will be
// prepended to generate full name. For example: ".ItemType" will be conveted to
// "<ProtoPackage>.ItemType"
func (x *TypeInfos) Get(name string) *TypeInfo {
	var fullName string
	if strings.HasPrefix(name, ".") {
		// prepend default proto package
		fullName = x.protoPackage + name
	} else {
		fullName = name
	}
	return x.GetByFullName(protoreflect.FullName(fullName))
}

// GetByFullName retrieves type info by type's full name.
func (x *TypeInfos) GetByFullName(fullName protoreflect.FullName) *TypeInfo {
	return x.infos[fullName]
}

func GetAllTypeInfo(files *protoregistry.Files, protoPackage string) *TypeInfos {
	typeInfos := NewTypeInfos(protoPackage)
	files.RangeFiles(func(fileDesc protoreflect.FileDescriptor) bool {
		extractTypeInfos(fileDesc.Messages(), typeInfos)
		for i := 0; i < fileDesc.Enums().Len(); i++ {
			ed := fileDesc.Enums().Get(i)
			info := &TypeInfo{
				FullName:       ed.FullName(),
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
		FullName:             md.FullName(),
		ParentFilename:       md.ParentFile().Path(),
		Kind:                 types.MessageKind,
		FirstFieldOptionName: firstFieldOptionName,
	}
	typeInfos.Put(info)

	for i := 0; i < md.Enums().Len(); i++ {
		ed := md.Enums().Get(i)
		info := &TypeInfo{
			FullName:       ed.FullName(),
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
