package parallel

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/options"
)

func init() {
	atom.InitZap("debug")
}

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
		goPackage,
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
		options.Imports(
			"common_conf.proto",
			"cs_dbkeyword_conf.proto",
		),
		options.LogLevel("info"),
		// options.Input(
		// 	&options.InputOption{
		// 		Subdirs:        subdirs,
		// 		SubdirRewrites: rewrites,
		// 	},
		// ),
		options.Output(
			&options.OutputOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
			},
		),
		options.LogLevel("info"),
	)
}
