package load

import (
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Merge merges src into dst, which must be a message with the same descriptor.
//
// # Default Merge mechanism
//   - scalar: Populated scalar fields in src are copied to dst.
//   - message: Populated singular messages in src are merged into dst by
//     recursively calling [proto.Merge].
//   - list: The elements of every list field in src are appended to the
//     corresponded list fields in dst.
//   - map: The entries of every map field in src are copied into the
//     corresponding map field in dst, possibly replacing existing entries.
//   - unknown: The unknown fields of src are appended to the unknown
//     fields of dst.
//
// # Top-field patch option "PATCH_REPLACE"
//   - list: Clear field firstly, and then all elements of this list field
//     in src are appended to the corresponded list fields in dst.
//   - map: Clear field firstly, and then all entries of this map field in src
//     are copied into the corresponding map field in dst.
//
// [proto.Merge]: https://pkg.go.dev/google.golang.org/protobuf/proto#Merge
func Merge(dst, src proto.Message) error {
	dstMsg, srcMsg := dst.ProtoReflect(), src.ProtoReflect()
	if dstMsg.Descriptor().FullName() != srcMsg.Descriptor().FullName() {
		return errors.Errorf("dst %s and src %s are not messages with the same descriptor",
			dstMsg.Descriptor().FullName(),
			srcMsg.Descriptor().FullName())
	}
	// Range iterates over every populated field in an undefined order.
	srcMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		opts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
		patch := opts.GetProp().GetPatch()
		if patch == tableaupb.Patch_PATCH_REPLACE {
			log.Debugf("patch(%s) %s's field: %s", patch, dstMsg.Descriptor().Name(), fd.Name())
			dstMsg.Clear(fd)
		}
		return true
	})
	proto.Merge(dst, src)
	return nil
}
