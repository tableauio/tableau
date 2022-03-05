package main

import (
	"testing"

	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/test/dev/testpb"
)

func init() {
	atom.InitZap("debug")
}

func Test_Excel2Proto(t *testing.T) {
	tableau.GenProto(
		"test",
		"github.com/tableauio/tableau/cmd/test/testpb",
		"./testdata",
		"./protoconf",
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
			[]string{
				"cs_dbkeyword.proto",
				"common.proto",
				"time.proto",
			},
		),
		options.Input(
			&options.InputOption{
				Format:  format.Excel,
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
		"test",
		"./testdata",
		"./_output/json/",
		options.Input(
			&options.InputOption{
				Format: format.Excel,
			},
		),
		options.LogLevel("debug"),
	)
}

func Test_Excel2JSON_Select(t *testing.T) {
	tableau.GenConf(
		"test",
		"./testdata",
		"./_output/json/",
		options.Input(
			&options.InputOption{
				Format: format.Excel,
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
		imp := importer.NewExcelImporter(path, nil, nil, true)
		err := imp.ExportCSV()
		if err != nil {
			t.Error(err)
		}
	}
}

func Test_CSV2Excel(t *testing.T) {
	paths := []string{
		"./testdata/excel/Test#*.csv",
		"./testdata/excel/hero/Hero#*.csv",
	}
	for _, path := range paths {
		imp := importer.NewCSVImporter(path, nil, nil)
		err := imp.ExportExcel()
		if err != nil {
			t.Errorf("%+v", err)
		}
	}
}

func Test_XML2Proto(t *testing.T) {
	tableau.GenProto(
		"test",
		"github.com/tableauio/tableau/cmd/test/testpb",
		"./testdata",
		"./protoconf",
		options.Imports(
			[]string{
				"cs_dbkeyword.proto",
				"common.proto",
				"time.proto",
			},
		),
		options.Input(
			&options.InputOption{
				Format: format.XML,
			},
		),
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
		"test",
		"./testdata",
		"./_output/json",
		options.LogLevel("debug"),
		options.Input(
			&options.InputOption{
				Format: format.XML,
			},
		),
	)
	// tableau.Generate("test", "./testdata/", "./_output/xml/")
}

func Test_LoadJSON(t *testing.T) {
	msg := &testpb.Activity{}
	err := load.Load(msg, "./_output/json/", format.JSON)
	if err != nil {
		t.Error(err)
	}
}

func Test_LoadExcel(t *testing.T) {
	msg := &testpb.Hero{}
	err := load.Load(msg, "./testdata/", format.Excel)
	if err != nil {
		t.Error(err)
	}
}

func Test_GenProto(t *testing.T) {
	tableau.GenProto(
		"test",
		"github.com/tableauio/tableau/cmd/test/testpb",
		"./testdata",
		"./protoconf",
		options.Imports(
			[]string{
				"cs_dbkeyword.proto",
				"common.proto",
				"time.proto",
			},
		),
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
		"test",
		"./testdata",
		"./_output/json",
		options.LogLevel("debug"),
	)
}
