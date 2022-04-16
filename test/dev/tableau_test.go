package dev

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/options"
	_ "github.com/tableauio/tableau/test/dev/protoconf"
)

func init() {
	atom.InitZap("DEBUG")
}

func Test_GenProto(t *testing.T) {
	tableau.GenProto(
		"protoconf",
		"./testdata",
		"./proto",
		options.Input(
			&options.InputOption{
				// ImportPaths: []string{
				// 	"../../proto",
				// },
				ImportFiles: []string{
					"cs_dbkeyword.proto",
					"common.proto",
					"time.proto",
				},
				Formats: []format.Format{
					// format.Excel,
					format.CSV,
					format.XML,
				},
				// Formats: []format.Format{format.CSV},
				// Subdirs: []string{`excel/`},
				// SubdirRewrites: map[string]string{
				// 	`excel/`: ``,
				// },
			},
		),
		options.Output(
			&options.OutputOption{
				ProtoFilenameSuffix:           "_conf",
				ProtoFilenameWithSubdirPrefix: false,
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

func Test_GenJSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.LogLevel("DEBUG"),
		options.Output(
			&options.OutputOption{
				Pretty:  true,
				Formats: []format.Format{format.JSON},
			},
		),
	)
}

func Test_Excel2JSON_Select(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.Input(
			&options.InputOption{
				Formats: []format.Format{format.Excel},
			},
		),
		options.LogLevel("DEBUG"),
		// options.Workbook("hero/Hero.xlsx"),
		// options.Workbook("./hero/Hero.xlsx"),
		options.Workbook(".\\excel\\hero\\Hero.xlsx"),
		options.Worksheet("Hero"),
	)
}

func Test_Excel2CSV(t *testing.T) {
	paths := []string{
		"./testdata/excel/Test.xlsx",
		"./testdata/excel/hero/Hero.xlsx",
		"./testdata/excel/hero/HeroA.xlsx",
		"./testdata/excel/hero/HeroB.xlsx",
	}
	for _, path := range paths {
		imp, err := importer.NewExcelImporter(path, nil, nil, 0)
		if err != nil {
			t.Errorf("%+v", err)
		}
		if err := imp.ExportCSV(); err != nil {
			t.Errorf("%+v", err)
		}
	}
}

func Test_CSV2Excel(t *testing.T) {
	paths := []string{
		"./testdata/excel/Test#*.csv",
		"./testdata/excel/hero/Hero#*.csv",
		"./testdata/excel/hero/HeroA#*.csv",
		"./testdata/excel/hero/HeroB#*.csv",
	}
	for _, path := range paths {
		imp, err := importer.NewCSVImporter(path, nil, nil)
		if err != nil {
			t.Errorf("%+v", err)
		}
		if err := imp.ExportExcel(); err != nil {
			t.Errorf("%+v", err)
		}
	}
}

func Test_GenJSON_Subdir(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.Input(
			&options.InputOption{
				// Formats: []format.Format{format.CSV},
				Subdirs: []string{`excel/`},
				SubdirRewrites: map[string]string{
					`excel/`: ``,
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Formats: []format.Format{format.JSON},
			},
		),
		options.LogLevel("DEBUG"),
	)
}
