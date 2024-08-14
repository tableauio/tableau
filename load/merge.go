package load

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// merge merges patch into main message, which must be the same descriptor.
//
// 1. top none-map field (field value):
//   - `replace`: if field present in both **main** and **patch** sheet
//
// 2. top map field patch (key-value pair):
//   - `add`: if key not exists in **main** sheet
//   - `replace`: if key exists in both **main** and **patch** sheet
func merge(main, patch proto.Message) error {
	mainMsg, patchMsg := main.ProtoReflect(), patch.ProtoReflect()
	// Range iterates over every populated field in an undefined order.
	patchMsg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch {
		case fd.IsMap():
			mergeMap(mainMsg.Mutable(fd).Map(), v.Map())
		default:
			mainMsg.Set(fd, v)
		}
		return true
	})
	return nil
}

// - `add`: if key not exists in **main** sheet
// - `replace`: if key exists in both **main** and **patch** sheet
func mergeMap(main, patch protoreflect.Map) {
	patch.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		main.Set(k, v)
		return true
	})
}
