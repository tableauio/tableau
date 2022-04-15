package dynamic

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func Test_GenJSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"../testdata",
		"../_conf",
		options.ImportPaths("../proto"),
		options.ProtoFiles("../proto/*.proto"),
		options.LogLevel("debug"),
		options.OutputFormats(format.JSON),
	)
}

func Test_Generate(t *testing.T) {
	tableau.Generate(
		"protoconf",
		"../testdata",
		"./_out",
		// options.ImportPaths("../proto/common"), // FIXME: this is not working
		options.ImportPaths("./_out/proto"),
		options.ProtoFiles("./_out/proto/*.proto"),
		options.Imports(
			"cs_dbkeyword.proto",
			"common.proto",
			"time.proto",
		),
		options.InputFormats(format.CSV, format.XML),
		options.Output(
			&options.OutputOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
				Pretty:                   true,
				Formats:                  []format.Format{format.JSON},
				ProtoFileOptions: map[string]string{
					"go_package": "github.com/tableauio/tableau/test/dev/protoconf",
				},
			},
		),
		options.Header(
			&options.HeaderOption{
				Namerow: 1,
				Typerow: 2,
				Noterow: 3,
				Datarow: 5,

				Nameline: 2,
				Typeline: 2,
			}),
		options.LogLevel("debug"),
	)
}
