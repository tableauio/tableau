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
		options.Input(
			&options.InputOption{
				ImportPaths: []string{
					"../proto",
				},
				ProtoFiles: []string{"../proto/*.proto"},
				Formats: []format.Format{
					// format.Excel,
					format.CSV,
					format.XML,
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Pretty:  true,
				Formats: []format.Format{format.JSON},
			},
		),
		options.LogLevel("DEBUG"),
	)
}

func Test_Generate(t *testing.T) {
	tableau.Generate(
		"protoconf",
		"../testdata",
		"./_out",
		options.Input(
			&options.InputOption{
				// FIXME: this is not working
				ImportPaths: []string{
					"./_out/proto",
					// "./_out",
					// "../proto/common", // FIXME: this is not working yet for standalone common dir.
				},
				ImportFiles: []string{
					"cs_dbkeyword.proto",
					"common.proto",
					"time.proto",
				},
				ProtoFiles: []string{"./_out/proto/*.proto"},
				// ProtoFiles: []string{"./_out/*.proto"},
				Formats: []format.Format{
					// format.Excel,
					format.CSV,
					format.XML,
				},
			},
		),
		options.Output(
			&options.OutputOption{
				ProtoSubdir:                   "proto",
				ConfSubdir:                    "conf",
				ProtoFilenameSuffix:           "_conf",
				ProtoFilenameWithSubdirPrefix: false,
				Pretty:                        true,
				Formats:                       []format.Format{format.JSON},
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
		options.LogLevel("DEBUG"),
	)
}
