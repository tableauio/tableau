package xproto

import (
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// PatchMessage patches src into dst, which must be a message with the same descriptor.
//
// # Patch option "PATCH_MERGE"
//   - scalar: Populated scalar fields in src are copied to dst.
//   - message: Populated singular messages in src are merged into dst by
//     recursively calling [xproto.PatchMessage].
//   - list: The elements of every list field in src are appended to the
//     corresponded list fields in dst.
//   - map: The entries of every map field in src are MERGED (different from
//     the behavior of proto.Merge) into the corresponding map field in dst.
//
// # Patch option "PATCH_REPLACE"
//   - scalar: Same with "PATCH_MERGE".
//   - message: Clear message firstly, then copy the source message to dst.
//   - list: Clear list firstly, and then all elements of this list field
//     in src are appended to the corresponded list fields in dst.
//   - map: Clear list firstly, and then all entries of this map field in src
//     are copied into the corresponding map field in dst.
//
// [proto.Merge]: https://pkg.go.dev/google.golang.org/protobuf/proto#Merge
func PatchMessage(dst, src proto.Message, patch tableaupb.Patch) error {
	if patch == tableaupb.Patch_PATCH_NONE {
		return errors.Errorf("patch type none is invalid")
	}
	dstMsg, srcMsg := dst.ProtoReflect(), src.ProtoReflect()
	if dstMsg.Descriptor().FullName() != srcMsg.Descriptor().FullName() {
		return errors.Errorf("dst %s and src %s are not messages with the same descriptor",
			dstMsg.Descriptor().FullName(),
			srcMsg.Descriptor().FullName())
	}
	patchMessage(dstMsg, srcMsg, patch)
	return nil
}

func patchMessage(dst, src protoreflect.Message, patch tableaupb.Patch) {
	if patch == tableaupb.Patch_PATCH_REPLACE {
		dst.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
			dst.Clear(fd)
			return true
		})
	}
	// Range iterates over every populated field in an undefined order.
	src.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		opts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
		fieldPatch := opts.GetProp().GetPatch()
		if fieldPatch == tableaupb.Patch_PATCH_NONE {
			fieldPatch = patch
		}
		switch {
		case fd.IsList():
			patchList(dst.Mutable(fd).List(), v.List(), fd, fieldPatch)
		case fd.IsMap():
			patchMap(dst.Mutable(fd).Map(), v.Map(), fd.MapValue(), fieldPatch)
		case fd.Message() != nil:
			patchMessage(dst.Mutable(fd).Message(), v.Message(), fieldPatch)
		case fd.Kind() == protoreflect.BytesKind:
			dst.Set(fd, cloneBytes(v))
		default:
			dst.Set(fd, v)
		}
		return true
	})
}

func patchList(dst, src protoreflect.List, fd protoreflect.FieldDescriptor, patch tableaupb.Patch) {
	if patch == tableaupb.Patch_PATCH_REPLACE {
		dst.Truncate(0)
	}
	// Merge semantics appends to the end of the existing list.
	for i, n := 0, src.Len(); i < n; i++ {
		switch v := src.Get(i); {
		case fd.Message() != nil:
			dstv := dst.NewElement()
			patchMessage(dstv.Message(), v.Message(), patch)
			dst.Append(dstv)
		case fd.Kind() == protoreflect.BytesKind:
			dst.Append(cloneBytes(v))
		default:
			dst.Append(v)
		}
	}
}

func patchMap(dst, src protoreflect.Map, fd protoreflect.FieldDescriptor, patch tableaupb.Patch) {
	if patch == tableaupb.Patch_PATCH_REPLACE {
		dst.Range(func(mk protoreflect.MapKey, _ protoreflect.Value) bool {
			dst.Clear(mk)
			return true
		})
	}
	// Merge semantics replaces, rather than merges into existing entries.
	src.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		switch {
		case fd.Message() != nil:
			// NOTE: this behavior is different from proto.Clone
			var dstv protoreflect.Value
			if dst.Has(k) {
				dstv = dst.Mutable(k)
			} else {
				dstv = dst.NewValue()
			}
			patchMessage(dstv.Message(), v.Message(), patch)
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
