package xproto

import (
	"strings"
	"sync"

	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

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

// TypeInfos holds all type infos by full name mapping.
type TypeInfos struct {
	mu           sync.RWMutex
	protoPackage string
	infos        map[protoreflect.FullName]*TypeInfo
}

// Put stores a new type info.
func (x *TypeInfos) Put(info *TypeInfo) {
	x.mu.Lock()
	defer x.mu.Unlock()
	log.Debugf("add new type: %v", info)
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
	x.mu.RLock()
	defer x.mu.RUnlock()
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
	firstFieldOptionName := parseFirstFieldOptionName(md)
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

// parseFirstFieldOptionName parses the first field option name of the message.
//   - If the message is a union, return the name of the enum type field.
//   - Else if the message has sub fields, return the name of the first field. Besides,
//     if the first field's kind is message and its span is not inner cell,
//     then recursively parse sub fields' option name and concat them.
//   - Otherwise, return empty string.
func parseFirstFieldOptionName(md protoreflect.MessageDescriptor) string {
	if IsUnion(md) {
		desc := ExtractUnionDescriptor(md)
		if desc != nil {
			// union's first field is enum type field.
			fieldOpts := proto.GetExtension(desc.Type.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
			return fieldOpts.GetName()
		}
	} else if md.Fields().Len() != 0 {
		// struct's first field
		fd := md.Fields().Get(0)
		fieldOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
		firstFieldOptionName := fieldOpts.GetName()
		// if the first field's kind is message, and field's span is not inner cell,
		// then we need to parse sub message's first field option name recursively.
		if subMD := fd.Message(); subMD != nil && fieldOpts.GetSpan() != tableaupb.Span_SPAN_INNER_CELL {
			firstFieldOptionName += parseFirstFieldOptionName(subMD)
		}
		return firstFieldOptionName
	}
	return ""
}
