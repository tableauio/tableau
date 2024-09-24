package xproto

import (
	"fmt"

	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var ErrDuplicateKey = fmt.Errorf("duplicate key")

// Merge merges src into dst, which must be a message with the same descriptor.
//
// NOTE: message should only has two kinds of field:
//  1. list
//  2. map: src should not has duplicate key in dst
func Merge(dst, src proto.Message) error {
	dstMsg, srcMsg := dst.ProtoReflect(), src.ProtoReflect()
	return mergeMessage(dstMsg, srcMsg)
}

// only list and map is supported
func mergeMessage(dst, src protoreflect.Message) error {
	var err error
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsList():
			mergeList(dst.Mutable(fd).List(), v.List(), fd)
		case fd.IsMap():
			err = mergeMap(dst.Mutable(fd).Map(), v.Map(), fd)
			if err != nil {
				return false
			}
		default:
			err = fmt.Errorf("field: %v is not list or map", fd.Name())
			return false
		}
		return true
	})
	return err
}

func mergeList(dst, src protoreflect.List, fd protoreflect.FieldDescriptor) {
	// Merge semantics appends to the end of the existing list.
	for i, n := 0, src.Len(); i < n; i++ {
		dst.Append(src.Get(i))
	}
}

func mergeMap(dst, src protoreflect.Map, fd protoreflect.FieldDescriptor) (err error) {
	// Merge semantics replaces, rather than merges into existing entries.
	src.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		if dst.Has(k) {
			err = ErrDuplicateKey
			return false
		}
		dst.Set(k, v)
		return true
	})
	return err
}

// CheckMapDuplicateKey checks the map field's duplicate key in message with the same descriptor.
func CheckMapDuplicateKey(dst, src proto.Message) error {
	dstMsg, srcMsg := dst.ProtoReflect(), src.ProtoReflect()
	var err error
	srcMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			dstMap := dstMsg.Mutable(fd).Map()
			srcMap := v.Map()
			srcMap.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
				if dstMap.Has(k) {
					err = xerrors.E2009(k, fd.Name())
					return false
				}
				return true
			})

		}
		return true
	})
	return err
}
