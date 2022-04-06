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
	atom.InitZap("debug")
}

func Test_Excel2Proto(t *testing.T) {
	tableau.GenProto(
		"protoconf",
		"github.com/tableauio/tableau/test/dev/protoconf",
		"./testdata",
		"./proto",
		options.Header(
			&options.HeaderOption{
				Namerow: 1,
				Typerow: 2,
				Noterow: 3,
				Datarow: 5,

				Nameline: 2,
				Typeline: 2,
			}),
		options.Imports(
			"cs_dbkeyword.proto",
			"common.proto",
			"time.proto",
		),
		options.Input(
			&options.InputOption{
				Formats: []format.Format{format.Excel},
				Subdirs: []string{`./`},
			},
		),
		options.Output(
			&options.OutputOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
			},
		),
		options.LogLevel("debug"),
	)
}

func Test_Excel2JSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.InputFormats(format.Excel),
		options.LogLevel("debug"),
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
		options.LogLevel("debug"),
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
	}
	for _, path := range paths {
		imp, err := importer.NewExcelImporter(path, nil, nil)
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

func Test_XML2Proto(t *testing.T) {
	tableau.GenProto(
		"protoconf",
		"github.com/tableauio/tableau/test/dev/protoconf",
		"./testdata",
		"./proto",
		options.Imports(
			"cs_dbkeyword.proto",
			"common.proto",
			"time.proto",
		),
		options.InputFormats(format.XML),
		options.Output(
			&options.OutputOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
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
	)
}

func Test_XML2JSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.LogLevel("debug"),
		options.InputFormats(format.XML),
	)
	// tableau.Generate("protoconf", "./testdata/", "./_output/xml/")
}

func Test_GenProto(t *testing.T) {
	tableau.GenProto(
		"protoconf",
		"github.com/tableauio/tableau/test/dev/protoconf",
		"./testdata",
		"./proto",
		// options.ImportPaths(
		// 	"../../proto",
		// ),
		options.Imports(
			"cs_dbkeyword.proto",
			"common.proto",
			"time.proto",
		),
		options.InputFormats(format.CSV, format.XML),
		// options.Input(
		// 	&options.InputOption{
		// 		// Formats: []format.Format{format.CSV},
		// 		// Subdirs: []string{`excel/`},
		// 		SubdirRewrites: map[string]string{
		// 			`excel/`: ``,
		// 		},
		// 	},
		// ),
		options.Output(
			&options.OutputOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
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

func Test_GenJSON(t *testing.T) {
	tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.LogLevel("debug"),
		options.OutputFormats(format.JSON),
	)
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
		options.LogLevel("debug"),
		options.OutputFormats(format.JSON),
	)
}
