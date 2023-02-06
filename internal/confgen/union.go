package confgen

import (
	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func IsUnion(md protoreflect.MessageDescriptor) bool {
	return proto.GetExtension(md.Options(), tableaupb.E_Union).(bool)
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

func (u UnionDescriptor) TypeName() string {
	opts := proto.GetExtension(u.Type.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	if opts != nil {
		return opts.Name
	}
	// default
	return strcase.ToCamel(string(u.Type.Name()))
}

func (u UnionDescriptor) ValueFieldName() string {
	opts := proto.GetExtension(u.Value.Options(), tableaupb.E_Oneof).(*tableaupb.OneofOptions)
	if opts != nil {
		return opts.Field
	}
	// default
	return "Field"
}

func (u UnionDescriptor) GetValueByNumber(n int32) protoreflect.FieldDescriptor {
	return GetOneofFieldByNumber(u.Value, n)
}
