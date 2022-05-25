package dynamic

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

func Test_GenProto(t *testing.T) {
	err := tableau.GenProto(
		"protoconf",
		"../testdata",
		"./_proto",
		options.Input(
			&options.InputOption{
				Proto: &options.InputProtoOption{
					ProtoPaths: []string{"./_proto"},
					ProtoCustomFiles: []string{
						"common/cs_dbkeyword.proto",
						"common/common.proto",
						"common/time.proto",
					},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
					Header: &options.HeaderOption{
						Namerow: 1,
						Typerow: 2,
						Noterow: 3,
						Datarow: 5,

						Nameline: 2,
						Typeline: 2,
					},
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Proto: &options.OutputProtoOption{
					FilenameSuffix:           "_conf",
					FilenameWithSubdirPrefix: false,
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/dev/protoconf",
					},
				},
			},
		),
		options.LogLevel("DEBUG"),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}

func Test_GenConf(t *testing.T) {
	err := tableau.GenConf(
		"protoconf",
		"../testdata",
		"./_conf",
		options.Input(
			&options.InputOption{
				Conf: &options.InputConfOption{
					ProtoPaths: []string{"./_proto"},
					ProtoFiles: []string{"./_proto/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Conf: &options.OutputConfOption{
					Pretty:  true,
					Formats: []format.Format{format.JSON},
				},
			},
		),
		options.LogLevel("DEBUG"),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}

func Test_Generate(t *testing.T) {
	err := tableau.Generate(
		"protoconf",
		"../testdata",
		"./_out",
		options.Input(
			&options.InputOption{
				Proto: &options.InputProtoOption{
					ProtoPaths: []string{"./_out/proto"},
					ProtoCustomFiles: []string{
						"cs_dbkeyword.proto",
						"common.proto",
						"time.proto",
					},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
					Header: &options.HeaderOption{
						Namerow: 1,
						Typerow: 2,
						Noterow: 3,
						Datarow: 5,

						Nameline: 2,
						Typeline: 2,
					},
				},

				Conf: &options.InputConfOption{
					ProtoPaths: []string{"./_out/proto"},
					ProtoFiles: []string{"./_out/proto/*.proto"},
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Proto: &options.OutputProtoOption{
					FilenameSuffix:           "_conf",
					FilenameWithSubdirPrefix: false,
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/dev/protoconf",
					},
					Subdir: "proto",
				},
				Conf: &options.OutputConfOption{
					Pretty:  true,
					Formats: []format.Format{format.JSON},
					Subdir:  "conf",
				},
			},
		),
		options.LogLevel("DEBUG"),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}
