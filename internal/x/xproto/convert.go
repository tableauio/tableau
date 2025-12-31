package xproto

import (
	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/protoutil"
	"github.com/bufbuild/protocompile/walk"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

func resolveGlobalFiles(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

func convert(results linker.Files) []*descriptorpb.FileDescriptorProto {
	mp := map[string]*descriptorpb.FileDescriptorProto{}
	for _, res := range results {
		convertOne(res, mp)
	}
	fdps := make([]*descriptorpb.FileDescriptorProto, 0, len(mp))
	for _, fdp := range mp {
		fdps = append(fdps, fdp)
	}
	return fdps
}

func convertOne(d protoreflect.FileDescriptor, mp map[string]*descriptorpb.FileDescriptorProto) {
	if _, ok := mp[d.Path()]; ok {
		return
	}
	fdp := protoutil.ProtoFromFileDescriptor(d)
	removeDynamicExtensionsFromProto(fdp)
	mp[d.Path()] = fdp
	imports := d.Imports()
	for i := 0; i < imports.Len(); i++ {
		convertOne(imports.Get(i).FileDescriptor, mp)
	}
}

func removeDynamicExtensionsFromProto(fd *descriptorpb.FileDescriptorProto) {
	// protocompile returns descriptors with dynamic extension fields for custom options.
	// But protoparse only used known custom options and everything else defined in the
	// sources would be stored as unrecognized fields. So to bridge the difference in
	// behavior, we need to remove custom options from the given file and add them back
	// via serializing-then-de-serializing them back into the options messages. That way,
	// statically known options will be properly typed and others will be unrecognized.
	//
	// Refer: https://github.com/jhump/protoreflect/blob/v1.17.0/desc/protoparse/parser.go#L724
	fd.Options = removeDynamicExtensionsFromOptions(fd.Options)
	_ = walk.DescriptorProtos(fd, func(_ protoreflect.FullName, msg proto.Message) error {
		switch msg := msg.(type) {
		case *descriptorpb.DescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
			for _, extr := range msg.ExtensionRange {
				extr.Options = removeDynamicExtensionsFromOptions(extr.Options)
			}
		case *descriptorpb.FieldDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.OneofDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumValueDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.ServiceDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.MethodDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		}
		return nil
	})
}

type fieldValue struct {
	fd  protoreflect.FieldDescriptor
	val protoreflect.Value
}

func removeDynamicExtensionsFromOptions[O proto.Message](opts O) O {
	removeOne(opts.ProtoReflect())
	return opts
}

func removeOne(opts protoreflect.Message) {
	dynamicOpts := opts.Type().New()
	opts.Range(func(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if fd.IsExtension() {
			dynamicOpts.Set(fd, val)
			opts.Clear(fd)
		}
		return true
	})
	// serialize only these custom options
	data, _ := proto.MarshalOptions{AllowPartial: true}.Marshal(dynamicOpts.Interface())
	// and then replace values by clearing these custom options and deserializing
	_ = proto.UnmarshalOptions{AllowPartial: true, Merge: true}.Unmarshal(data, opts.Interface())
}
