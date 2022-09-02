package dev

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	_ "github.com/tableauio/tableau/test/dev/protoconf"
	"github.com/tableauio/tableau/xerrors"
)

func Test_GenProto(t *testing.T) {
	err := tableau.GenProto(
		"protoconf",
		"./testdata",
		"./proto",
		options.InputProto(
			&options.InputProtoOption{
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
				// Formats: []format.Format{format.CSV},
				// Subdirs: []string{`xml/`},
				// SubdirRewrites: map[string]string{
				// 	`excel/`: ``,
				// },
				Header: &options.HeaderOption{
					Namerow: 1,
					Typerow: 2,
					Noterow: 3,
					Datarow: 5,

					Nameline: 2,
					Typeline: 2,
				},
			},
		),
		options.OutputProto(
			&options.OutputProtoOption{
				FilenameSuffix:           "_conf",
				FilenameWithSubdirPrefix: false,
				FileOptions: map[string]string{
					"go_package": "github.com/tableauio/tableau/test/dev/protoconf",
				},
			},
		),
		options.Log(
			&log.Options{
				Level: "INFO",
				Mode:  "FULL",
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
		// t.Errorf("%v", err)
		// t.Errorf("%s", xerrors.NewDesc(err).String())
		// t.Errorf("%s", xerrors.NewDesc(err).StringZh())

		if log.Mode() == log.ModeFull {
			log.Errorf("generate conf failed: %+v", err)
		}
		if log.Lang() == log.LangEn {
			log.Errorf("%s", xerrors.NewDesc(err).String())
		} else {
			log.Errorf("%s", xerrors.NewDesc(err).StringZh())
		}
	}
}

func Test_GenConf(t *testing.T) {
	err := tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.OutputConf(
			&options.OutputConfOption{
				Pretty:  true,
				Formats: []format.Format{format.JSON},
				// EmitUnpopulated: true,
			},
		),
		options.Log(
			&log.Options{
				Level: "INFO",
				Mode:  "SIMPLE",
				Lang:  "zh",
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
		// t.Errorf("%v", err)
		// t.Errorf("%s", xerrors.NewDesc(err).String())
		// t.Errorf("%s", xerrors.NewDesc(err).StringZh())

		if log.Mode() == log.ModeFull {
			log.Errorf("generate conf failed: %+v", err)
		}
		if log.Lang() == log.LangEn {
			log.Errorf("%s", xerrors.NewDesc(err).String())
		} else {
			log.Errorf("%s", xerrors.NewDesc(err).StringZh())
		}
	}
}

func Test_CompareJSON(t *testing.T) {
	newConfDir := "_conf"
	// oldConfDir := "_old_conf"
	oldConfDir := "dynamic/_out/conf"
	files, err := os.ReadDir(newConfDir)
	if err != nil {
		t.Errorf("failed to read dir: %s", newConfDir)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		// if file.Name() == "Loader.json"{
		// 	continue
		// }
		newPath := filepath.Join(newConfDir, file.Name())
		oldPath := filepath.Join(oldConfDir, file.Name())
		newData, err := os.ReadFile(newPath)
		if err != nil {
			t.Error(err)
		}
		oldData, err := os.ReadFile(oldPath)
		if err != nil {
			t.Error(err)
		}
		fmt.Printf("compare json file: %s\n", file.Name())
		require.JSONEq(t, string(oldData), string(newData))
	}
}

func Test_Excel2CSV(t *testing.T) {
	paths := []string{
		"./testdata/excel/Test.xlsx",
		"./testdata/excel/hero/Hero.xlsx",
		"./testdata/excel/hero/HeroA.xlsx",
		"./testdata/excel/hero/HeroB.xlsx",
		"./testdata/excel/list/List.xlsx",
		"./testdata/excel/map/Map.xlsx",
		"./testdata/excel/metasheet/Metasheet.xlsx",
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
		"./testdata/excel/list/List#*.csv",
		"./testdata/excel/map/Map#*.csv",
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
	err := tableau.GenConf(
		"protoconf",
		"./testdata",
		"./_conf",
		options.InputConf(
			&options.InputConfOption{
				Formats: []format.Format{format.XML},
				// Subdirs: []string{`excel/`},
				// SubdirRewrites: map[string]string{
				// 	`excel/`: ``,
				// },
			},
		),
		options.OutputConf(
			&options.OutputConfOption{
				Formats: []format.Format{format.JSON},
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}
