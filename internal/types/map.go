package types

import (
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const DefaultMapFieldOptNameSuffix = "Map"

const DefaultMapKeyOptName = "Key"
const DefaultMapValueOptName = "Value"
const DefaultDocumentMapKeyOptName = "@key"
const DefaultDocumentMapValueOptName = "@value"

func CheckMessageWithOnlyKVFields(md protoreflect.MessageDescriptor) bool {
	if md.Fields().Len() == 2 {
		keyFd := md.Fields().Get(0)
		valFd := md.Fields().Get(1)
		isTableKV := expectFieldOptName(keyFd, DefaultMapKeyOptName) && expectFieldOptName(valFd, DefaultMapValueOptName)
		if isTableKV {
			return true
		}
		isDocumentKV := expectFieldOptName(keyFd, DefaultDocumentMapKeyOptName) && expectFieldOptName(valFd, DefaultDocumentMapValueOptName)
		return isDocumentKV
	}
	return false
}

func expectFieldOptName(fd protoreflect.FieldDescriptor, name string) bool {
	fdOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	return fdOpts != nil && fdOpts.Name == name
}
