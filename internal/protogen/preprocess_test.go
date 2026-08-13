package protogen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/options"
)

// prepareGeneratedProto writes a proto file into the output dir, mimicking a
// previously generated one, whose message type is absent from both the input
// workbooks and the predefined proto files.
func prepareGeneratedProto(t *testing.T, outdir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(outdir, xfs.DefaultDirPerm))
	content := `syntax = "proto3";
package protoconf;
option go_package = "github.com/tableauio/tableau/test/dev/protoconf";

import "tableau/protobuf/tableau.proto";

option (tableau.workbook) = {name:"Stale#*.csv" namerow:1 typerow:2 datarow:3};

message StaleConf {
  option (tableau.worksheet) = {name:"StaleConf"};

  map<uint32, uint32> stale_map = 1 [(tableau.field) = {key:"ID" layout:LAYOUT_VERTICAL}];
}
`
	require.NoError(t, os.WriteFile(filepath.Join(outdir, "stale_conf.proto"), []byte(content), xfs.DefaultFilePerm))
}

func newPreserveFieldNumbersGenerator(t *testing.T, subdir string) *Generator {
	t.Helper()
	outputDir := t.TempDir()
	// NOTE: the outdir must be one of the proto paths, so that the previously
	// generated protos are resolvable, as real configs do.
	outdir := filepath.Join(outputDir, subdir)
	prepareGeneratedProto(t, outdir)
	return NewGeneratorWithOptions("protoconf", "testdata", outputDir, &options.Options{
		LocationName: "Asia/Shanghai",
		Proto: &options.ProtoOption{
			Input: &options.ProtoInputOption{
				ProtoPaths: []string{"../../proto", outdir},
			},
			Output: &options.ProtoOutputOption{
				Subdir:               subdir,
				PreserveFieldNumbers: true,
			},
		},
	})
}

// Test_preprocess_preserveFieldNumbersNotPollutingTypeInfos asserts that
// preserveFieldNumbers only warms up the registry cache, without leaking the
// previously generated protos into type infos, otherwise the types only
// existing in them would be mistaken for predefined ones, which makes the
// exporter import a proto file that no longer exists.
func Test_preprocess_preserveFieldNumbersNotPollutingTypeInfos(t *testing.T) {
	gen := newPreserveFieldNumbersGenerator(t, "default")
	outdir := filepath.Join(gen.OutputDir, gen.OutputOpt.Subdir)

	require.NoError(t, gen.preprocess(false, true))

	// The message only exists in the previously generated proto, so it must not
	// be treated as a predefined type.
	assert.Nil(t, gen.typeInfos.Get("protoconf.StaleConf"),
		"type in previously generated proto should not be in type infos")

	// Meanwhile the registry including generated protos should be cached before
	// the generated protos were deleted by prepareOutdir, so that
	// preserveFieldNumbers still works afterwards.
	require.NoFileExists(t, filepath.Join(outdir, "stale_conf.proto"), "generated proto should have been deleted")
	files := gen.getProtoRegistryFilesWithGenerated()
	_, err := files.FindDescriptorByName("protoconf.StaleConf")
	assert.NoError(t, err, "registry with generated protos should be cached before deletion")
}

// Test_preprocess_useGeneratedProtosPopulatesTypeInfos asserts that the
// advanced first-pass mode still sees the previously generated protos as
// predefined types.
func Test_preprocess_useGeneratedProtosPopulatesTypeInfos(t *testing.T) {
	gen := newPreserveFieldNumbersGenerator(t, "default")

	require.NoError(t, gen.preprocess(true, false))

	assert.NotNil(t, gen.typeInfos.Get("protoconf.StaleConf"),
		"type in previously generated proto should be in type infos")
}
