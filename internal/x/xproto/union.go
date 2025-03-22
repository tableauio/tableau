package xproto

import (
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func IsUnion(md protoreflect.MessageDescriptor) bool {
	return proto.GetExtension(md.Options(), tableaupb.E_Union).(*tableaupb.UnionOptions) != nil
}

func IsUnionField(fd protoreflect.FieldDescriptor) bool {
	if fd.Kind() == protoreflect.MessageKind {
		return IsUnion(fd.Message())
	}
	return false
}

// GetOneofFieldByNumber returns the FieldDescriptor for a field numbered n.
// It returns nil if not found.
func GetOneofFieldByNumber(od protoreflect.OneofDescriptor, n int32) protoreflect.FieldDescriptor {
	return od.Fields().ByNumber(protowire.Number(n))
}

func ExtractUnionDescriptor(md protoreflect.MessageDescriptor) *UnionDescriptor {
	var desc UnionDescriptor
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if fd.Kind() == protoreflect.EnumKind {
			desc.Type = fd
		} else if fd.Kind() == protoreflect.MessageKind {
			if fd.ContainingOneof() != nil {
				desc.Value = fd.ContainingOneof()
			}
		}
		if desc.Type != nil && desc.Value != nil {
			return &desc
		}
	}
	return nil
}

type UnionDescriptor struct {
	Type  protoreflect.FieldDescriptor
	Value protoreflect.OneofDescriptor
}

// TypeName returns the type field name.
// It returns CameCase style of proto field name if not set explicitly in field extension.
func (u UnionDescriptor) TypeName() string {
	opts := proto.GetExtension(u.Type.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	if opts != nil {
		return opts.Name
	}
	// default
	return string(u.Type.Name())
}

// ValueFieldName returns the value field name.
// It returns "Field" if not set explicitly in oneof extension.
func (u UnionDescriptor) ValueFieldName() string {
	opts := proto.GetExtension(u.Value.Options(), tableaupb.E_Oneof).(*tableaupb.OneofOptions)
	if opts != nil {
		return opts.Field
	}
	// default
	return "Field"
}

// GetValueByNumber returns the FieldDescriptor for a field numbered n.
// It returns nil if not found.
func (u UnionDescriptor) GetValueByNumber(n int32) protoreflect.FieldDescriptor {
	return GetOneofFieldByNumber(u.Value, n)
}
