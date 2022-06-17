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
					ImportedProtoFiles: []string{
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
					Subdirs: []string{
						// "xml/match",
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
		options.Log(
			&options.LogOption{
				Level: "INFO",
				Mode:  "FULL",
			},
		),
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
					ProtoPaths: []string{"./_proto", "."},
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
					Pretty:          true,
					Formats:         []format.Format{format.JSON},
					EmitUnpopulated: true,
				},
			},
		),
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
					ImportedProtoFiles: []string{
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
					ProtoPaths: []string{"./_out/proto", "."},
					ProtoFiles: []string{"_out/proto/*.proto"},
					ExcludedProtoFiles: []string{
						"_out/proto/cs_dbkeyword.proto",
						"_out/proto/common.proto",
						"_out/proto/time.proto",
					},
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
		options.Log(
			&options.LogOption{
				Level: "DEBUG",
				Mode:  "FULL",
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}

func Test_GenConf1(t *testing.T) {
	err := tableau.GenConf(
		"protoconf",
		"../testdata",
		"./_out/conf",
		options.Input(
			&options.InputOption{
				Conf: &options.InputConfOption{
					ProtoPaths: []string{"./_out/proto", "."},
					ProtoFiles: []string{"_out/proto/*.proto"},
					ExcludedProtoFiles: []string{
						"_out/proto/cs_dbkeyword.proto",
						"_out/proto/common.proto",
						"_out/proto/time.proto",
					},
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
		options.Log(
			&options.LogOption{
				Level: "DEBUG",
				Mode:  "FULL",
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}
