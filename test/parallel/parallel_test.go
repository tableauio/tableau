package parallel

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/options"
)

const (
	protoPackage    = "protoconf"
	goPackage       = "github.com/tableauio/tableau/test/parallel/protoconf"
	defaultProtoDir = "protoconf/proto"
	defaultInputDir = "conf"
	defaultJsonDir  = "golang/cloud/_output/json"
	defaultFormat   = "json"
	inputDir        = "./excel"
	outputDir       = "./proto"
)

func Test_GenProto(t *testing.T) {

	tableau.GenProto(
		protoPackage,
		inputDir,
		outputDir,
		options.Header(
			&options.HeaderOption{
				Namerow:  1,
				Typerow:  2,
				Noterow:  3,
				Datarow:  5,
				Nameline: 2,
				Typeline: 2,
			}),
		options.Input(
			&options.InputOption{
				ImportPaths: []string{defaultProtoDir},
				ImportFiles: []string{
					"common_conf.proto",
					"cs_dbkeyword_conf.proto",
				},
			},
		),
		options.Output(
			&options.OutputOption{
				ProtoFilenameSuffix:           "_conf",
				ProtoFilenameWithSubdirPrefix: false,
				ProtoFileOptions: map[string]string{
					"go_package": goPackage,
				},
			},
		),
		options.LogLevel("INFO"),
	)
}
