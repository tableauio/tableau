package types

import (
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const DefaultMapFieldOptNameSuffix = "Map"

const DefaultMapKeyOptName = "Key"
const DefaultMapValueOptName = "Value"

func CheckMessageWithOnlyKVFields(msg protoreflect.Message) bool {
	md := msg.Descriptor()
	if md.Fields().Len() == 2 {
		keyFd := md.Fields().Get(0)
		valFd := md.Fields().Get(1)
		return expectFieldOptName(keyFd, DefaultMapKeyOptName) && expectFieldOptName(valFd, DefaultMapValueOptName)
	}
	return false
}

func expectFieldOptName(fd protoreflect.FieldDescriptor, name string) bool {
	fdOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	return fdOpts != nil && fdOpts.Name == name
}
