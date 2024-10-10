package xproto

import (
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PatchMessage patches src into dst, which must be a message with the same descriptor.
//
// # Default PatchMessage mechanism
//   - scalar: Populated scalar fields in src are copied to dst.
//   - message: Populated singular messages in src are merged into dst by
//     recursively calling [xproto.PatchMessage], or replace dst message if
//     "PATCH_REPLACE" is specified for this field.
//   - list: The elements of every list field in src are appended to the
//     corresponded list fields in dst, or replace dst list if "PATCH_REPLACE"
//     is specified for this field.
//   - map: The entries of every map field in src are MERGED (different from
//     the behavior of proto.Merge) into the corresponding map field in dst,
//     or replace dst map if "PATCH_REPLACE" is specified for this field.
//   - unknown: The unknown fields of src are appended to the unknown
//     fields of dst (TODO: untested).
//
// [proto.Merge]: https://pkg.go.dev/google.golang.org/protobuf/proto#Merge
func PatchMessage(dst, src proto.Message) error {
	dstMsg, srcMsg := dst.ProtoReflect(), src.ProtoReflect()
	if dstMsg.Descriptor().FullName() != srcMsg.Descriptor().FullName() {
		return xerrors.Errorf("dst %s and src %s are not messages with the same descriptor",
			dstMsg.Descriptor().FullName(),
			srcMsg.Descriptor().FullName())
	}
	patchMessage(dstMsg, srcMsg)
	return nil
}

func patchMessage(dst, src protoreflect.Message) {
	// Range iterates over every populated field in an undefined order.
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		opts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
		fieldPatch := opts.GetProp().GetPatch()
		if fieldPatch == tableaupb.Patch_PATCH_REPLACE {
			dst.Clear(fd)
		}
		switch {
		case fd.IsList():
			patchList(dst.Mutable(fd).List(), v.List(), fd)
		case fd.IsMap():
			patchMap(dst.Mutable(fd).Map(), v.Map(), fd.MapValue())
		case fd.Message() != nil:
			patchMessage(dst.Mutable(fd).Message(), v.Message())
		case fd.Kind() == protoreflect.BytesKind:
			dst.Set(fd, cloneBytes(v))
		default:
			dst.Set(fd, v)
		}
		return true
	})

	if len(src.GetUnknown()) > 0 {
		dst.SetUnknown(append(dst.GetUnknown(), src.GetUnknown()...))
	}
}

func patchList(dst, src protoreflect.List, fd protoreflect.FieldDescriptor) {
	// Merge semantics appends to the end of the existing list.
	for i, n := 0, src.Len(); i < n; i++ {
		switch v := src.Get(i); {
		case fd.Message() != nil:
			dstv := dst.NewElement()
			patchMessage(dstv.Message(), v.Message())
			dst.Append(dstv)
		case fd.Kind() == protoreflect.BytesKind:
			dst.Append(cloneBytes(v))
		default:
			dst.Append(v)
		}
	}
}

func patchMap(dst, src protoreflect.Map, fd protoreflect.FieldDescriptor) {
	// Merge semantics MERGES INFO, rather than REPLACES existing entries.
	src.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch {
		case fd.Message() != nil:
			// NOTE: this behavior is different from [proto.Clone]
			var dstv protoreflect.Value
			if dst.Has(k) {
				dstv = dst.Mutable(k)
			} else {
				dstv = dst.NewValue()
			}
			patchMessage(dstv.Message(), v.Message())
			dst.Set(k, dstv)
		case fd.Kind() == protoreflect.BytesKind:
			dst.Set(k, cloneBytes(v))
		default:
			dst.Set(k, v)
		}
		return true
	})
}

func cloneBytes(v protoreflect.Value) protoreflect.Value {
	return protoreflect.ValueOfBytes(append([]byte{}, v.Bytes()...))
}
