package protoc

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"github.com/bufbuild/protocompile/protoutil"
	"github.com/bufbuild/protocompile/walk"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"

	_ "github.com/tableauio/tableau/proto/tableaupb"
)

type fileDescriptorProtoMap map[string]*descriptorpb.FileDescriptorProto // file path -> file desc proto

// NewFiles creates a new protoregistry.Files from the provided proto paths, files, and excluded proto files.
func NewFiles(protoPaths []string, protoFiles []string, excludedProtoFiles ...string) (*protoregistry.Files, error) {
	cleanSlashProtoPaths := make([]string, len(protoPaths))
	for i, protoPath := range protoPaths {
		cleanSlashProtoPaths[i] = xfs.CleanSlashPath(protoPath)
	}
	parsedExcludedProtoFiles := make(map[string]bool)
	for _, filename := range excludedProtoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			parsedExcludedProtoFiles[cleanSlashPath] = true
		}
	}
	parsedProtoFiles := make(map[string]string) // full path -> rel path
	for _, filename := range protoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			if !parsedExcludedProtoFiles[cleanSlashPath] {
				rel := rel(cleanSlashPath, cleanSlashProtoPaths)
				parsedProtoFiles[cleanSlashPath] = rel
			}
		}
	}
	return parseProtos(protoPaths, parsedProtoFiles)
}

func rel(filename string, protoPaths []string) string {
	for _, protoPath := range protoPaths {
		if rel, err := filepath.Rel(protoPath, filename); err == nil {
			return xfs.CleanSlashPath(rel)
		}
	}
	return filename
}

// parseProtos parses the proto paths and proto files to protoregistry.Files.
func parseProtos(protoPaths []string, protoFilesMap map[string]string) (*protoregistry.Files, error) {
	log.Debugf("proto paths: %v", protoPaths)
	log.Debugf("proto files: %v", protoFilesMap)
	var protoFiles []string
	for _, path := range protoFilesMap {
		protoFiles = append(protoFiles, path)
	}
	compiler := protocompile.Compiler{
		Resolver: protocompile.CompositeResolver{
			protocompile.ResolverFunc(resolveGlobalFiles),
			&protocompile.SourceResolver{
				ImportPaths: protoPaths,
				Accessor: func(path string) (io.ReadCloser, error) {
					if _, ok := protoFilesMap[xfs.CleanSlashPath(path)]; !ok {
						return nil, fs.ErrNotExist
					}
					return os.Open(path)
				},
			},
		},
		MaxParallelism: 1,
	}
	results, err := compiler.Compile(context.Background(), protoFiles...)
	if err != nil {
		return nil, err
	}
	return protodesc.NewFiles(toFDS(results))
}

func resolveGlobalFiles(path string) (protocompile.SearchResult, error) {
	fd, err := protoregistry.GlobalFiles.FindFileByPath(path)
	if err != nil {
		return protocompile.SearchResult{}, err
	}
	return protocompile.SearchResult{Desc: fd}, nil
}

// toFDS converts linker.Files to *descriptorpb.FileDescriptorSet.
func toFDS(results linker.Files) *descriptorpb.FileDescriptorSet {
	fdpMap := make(fileDescriptorProtoMap)
	for _, res := range results {
		convertFile(res, fdpMap)
	}
	fdps := make([]*descriptorpb.FileDescriptorProto, 0, len(fdpMap))
	for _, fdp := range fdpMap {
		fdps = append(fdps, fdp)
	}
	return &descriptorpb.FileDescriptorSet{File: fdps}
}

func convertFile(d protoreflect.FileDescriptor, fdpMap fileDescriptorProtoMap) {
	if _, ok := fdpMap[d.Path()]; ok {
		// skip duplicate conversion
		return
	}
	fdp := protoutil.ProtoFromFileDescriptor(d)
	removeDynamicExtensionsFromProto(fdp)
	fdpMap[d.Path()] = fdp
	// convert imports recursively
	imports := d.Imports()
	for i := 0; i < imports.Len(); i++ {
		convertFile(imports.Get(i).FileDescriptor, fdpMap)
	}
}

func removeDynamicExtensionsFromProto(fd *descriptorpb.FileDescriptorProto) {
	// protocompile returns descriptors with dynamic extension fields for custom options.
	// But tableau only uses known custom options (*tableaupb.UnionOptions rather than
	// *dynamicpb.Message for example). So to bridge the difference in behavior, we need
	// to remove custom options from the given file and add them back via
	// serializing-then-de-serializing them back into the options messages. That way,
	// statically known options will be properly typed and others will be unrecognized.
	//
	// Refer:
	//   https://github.com/jhump/protoreflect/blob/v1.17.0/desc/protoparse/parser.go#L724
	fd.Options = removeDynamicExtensionsFromOptions(fd.Options)
	err := walk.DescriptorProtos(fd, func(_ protoreflect.FullName, msg proto.Message) error {
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
	if err != nil {
		log.Warnf("walk descriptor protos failed: %v", err)
	}
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
	data, err := proto.MarshalOptions{AllowPartial: true}.Marshal(dynamicOpts.Interface())
	if err != nil {
		// oh, well... can't fix this one
		log.Warnf("marshal dynamic options failed: %v", err)
		return
	}
	// and then replace values by clearing these custom options and deserializing
	err = proto.UnmarshalOptions{AllowPartial: true, Merge: true}.Unmarshal(data, opts.Interface())
	if err != nil {
		// oh, well... can't fix this one
		log.Warnf("unmarshal dynamic options failed: %v", err)
		return
	}
}
