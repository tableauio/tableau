package protoc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

const (
	testUnittestProtoPath = "tableau/protobuf/unittest/unittest.proto"
	testCommonProtoPath   = "tableau/protobuf/unittest/common.proto"
)

// genDescriptorSet compiles the unittest protos and writes them into a
// descriptor set file, mimicking what an external compiler does with
// `protoc --include_imports --descriptor_set_out`. Only the given proto paths
// are written if specified, so as to simulate an incomplete descriptor set.
func genDescriptorSet(t *testing.T, onlyProtoPaths ...string) string {
	t.Helper()
	files, err := NewFiles(
		[]string{"../../../../proto"},
		[]string{"../../../../proto/tableau/protobuf/unittest/*.proto"},
	)
	require.NoError(t, err)

	wanted := make(map[string]bool, len(onlyProtoPaths))
	for _, protoPath := range onlyProtoPaths {
		wanted[protoPath] = true
	}
	fds := &descriptorpb.FileDescriptorSet{}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		if len(wanted) == 0 || wanted[fd.Path()] {
			fds.File = append(fds.File, protodesc.ToFileDescriptorProto(fd))
		}
		return true
	})
	return writeDescriptorSet(t, fds)
}

func writeDescriptorSet(t *testing.T, fds *descriptorpb.FileDescriptorSet) string {
	t.Helper()
	data, err := proto.Marshal(fds)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), "unittest.binpb")
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func TestNewFilesFromDescriptorSet(t *testing.T) {
	t.Run("load all proto files", func(t *testing.T) {
		files, err := NewFilesFromDescriptorSet(genDescriptorSet(t))
		require.NoError(t, err)
		for _, protoPath := range []string{testUnittestProtoPath, testCommonProtoPath} {
			_, err := files.FindFileByPath(protoPath)
			assert.NoError(t, err, "proto file %s not found", protoPath)
		}
		// imports should be loaded as well
		_, err = files.FindFileByPath("google/protobuf/timestamp.proto")
		assert.NoError(t, err)
		_, err = files.FindDescriptorByName("unittest.FruitType")
		assert.NoError(t, err)
	})

	t.Run("custom options are statically typed", func(t *testing.T) {
		files, err := NewFilesFromDescriptorSet(genDescriptorSet(t))
		require.NoError(t, err)
		fd, err := files.FindFileByPath(testUnittestProtoPath)
		require.NoError(t, err)
		fileOpts, ok := fd.Options().(*descriptorpb.FileOptions)
		require.True(t, ok)
		bookOpts, ok := proto.GetExtension(fileOpts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
		require.True(t, ok)
		assert.Equal(t, "unittest/Unittest#*.csv", bookOpts.GetName())
	})

	t.Run("all imports are resolved", func(t *testing.T) {
		files, err := NewFilesFromDescriptorSet(genDescriptorSet(t))
		require.NoError(t, err)
		files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
			for i := 0; i < fd.Imports().Len(); i++ {
				imp := fd.Imports().Get(i)
				assert.False(t, imp.IsPlaceholder(), "import %s of %s is unresolved", imp.Path(), fd.Path())
			}
			return true
		})
	})

	t.Run("import not included in descriptor set", func(t *testing.T) {
		// NOTE: even the proto files already linked into the current binary
		// must be included in the descriptor set.
		partialFile := genDescriptorSet(t, testUnittestProtoPath, testCommonProtoPath)
		_, err := NewFilesFromDescriptorSet(partialFile)
		require.ErrorContains(t, err, "--include_imports")
		assert.ErrorContains(t, err, "tableau/protobuf/tableau.proto")
	})

	t.Run("descriptor set file not found", func(t *testing.T) {
		_, err := NewFilesFromDescriptorSet("not-existed.binpb")
		assert.ErrorContains(t, err, "failed to read descriptor set file")
	})

	t.Run("invalid descriptor set file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.binpb")
		require.NoError(t, os.WriteFile(path, []byte("not a descriptor set"), 0o644))
		_, err := NewFilesFromDescriptorSet(path)
		assert.ErrorContains(t, err, "failed to unmarshal descriptor set file")
	})

	t.Run("empty descriptor set file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "empty.binpb")
		require.NoError(t, os.WriteFile(path, nil, 0o644))
		_, err := NewFilesFromDescriptorSet(path)
		assert.ErrorContains(t, err, "no proto file found")
	})

	t.Run("duplicate proto file in descriptor set", func(t *testing.T) {
		fdp := &descriptorpb.FileDescriptorProto{
			Name:   proto.String("foo.proto"),
			Syntax: proto.String("proto3"),
		}
		fds := &descriptorpb.FileDescriptorSet{
			File: []*descriptorpb.FileDescriptorProto{fdp, fdp},
		}
		_, err := NewFilesFromDescriptorSet(writeDescriptorSet(t, fds))
		assert.ErrorContains(t, err, "appears multiple times")
	})
}
