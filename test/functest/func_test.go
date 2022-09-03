// Package functest is aimed for Functional Testing.
//
// "Functional Testing" is a black box testing technique, where
// the functionality of the application is tested to generate
// the desired output on providing a certain input.
//
// Test cases basically comprise of the following parts:
// 	- Test Summary
//	- Prerequisites (if any)
//	- Test case input steps
//	- Test data (if any)
// 	- Expected output
// 	- Notes (if any)
//
// “Requirement-Based” and “Business scenario-based” are the two
// forms of functional testing that are carried out.
//
// In Requirement based testing, test cases are created as per the
// requirement and tested accordingly. In a Business scenario based
// functional testing, testing is performed by keeping in mind all
// the scenarios from a business perspective.
package functest

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
)

func Test_CompareGeneratedProto(t *testing.T) {
	genProto(t)

	oldConfDir := "proto"
	newConfDir := "_proto"
	files, err := os.ReadDir(oldConfDir)
	if err != nil {
		t.Errorf("failed to read dir: %s", oldConfDir)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".proto") {
			continue
		}
		oldPath := filepath.Join(oldConfDir, file.Name())
		oldfile, err := os.Open(oldPath)
		if err != nil {
			t.Error(err)
		}
		newPath := filepath.Join(newConfDir, file.Name())
		newfile, err := os.Open(newPath)
		if err != nil {
			t.Error(err)
		}

		sscan := bufio.NewScanner(oldfile)
		dscan := bufio.NewScanner(newfile)

		for line := 1; sscan.Scan(); line++ {
			dscan.Scan()
			if line == 1 {
				// as the first line is one line comment (including dynamic version number), ignore it.
				continue
			}
			require.Equalf(t, string(sscan.Bytes()), string(dscan.Bytes()), "%s -> %s content not same at line: %d", oldPath, newPath, line)
		}
	}
}

func Test_CompareGeneratedJSON(t *testing.T) {
	genConf(t)
	
	oldConfDir := "conf"
	newConfDir := "_conf"
	files, err := os.ReadDir(oldConfDir)
	if err != nil {
		t.Errorf("failed to read dir: %s", oldConfDir)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
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
		require.JSONEqf(t, string(oldData), string(newData), "%s -> %s content not same.", oldPath, newPath)
	}
}

func genProto(t *testing.T) {
	err := tableau.GenProto(
		"protoconf",
		"./testdata",
		"./_proto",
		options.Input(
			&options.InputOption{
				Proto: &options.InputProtoOption{
					ProtoPaths: []string{"./_proto"},
					// ImportedProtoFiles: []string{
					// 	"common/cs_dbkeyword.proto",
					// 	"common/common.proto",
					// 	"common/time.proto",
					// },
					Formats: []format.Format{
						// format.Excel,
						format.CSV,
						format.XML,
					},
					Header: &options.HeaderOption{
						Namerow: 1,
						Typerow: 2,
						Noterow: 3,
						Datarow: 4,
					},
				},
			},
		),
		options.Output(
			&options.OutputOption{
				Proto: &options.OutputProtoOption{
					FilenameWithSubdirPrefix: false,
					FileOptions: map[string]string{
						"go_package": "github.com/tableauio/tableau/test/functest/protoconf",
					},
				},
			},
		),
		options.Log(
			&log.Options{
				Level: "DEBUG",
				Mode:  "FULL",
			},
		),
	)
	if err != nil {
		t.Errorf("%+v", err)
	}
}

func genConf(t *testing.T) {
	err := tableau.GenConf(
		"protoconf",
		"./testdata",
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

// func Test_Excel2CSV(t *testing.T) {
// 	paths := []string{
// 		"./testdata/excel/map/Map.xlsx",
// 		"./testdata/excel/metasheet/Metasheet.xlsx",
// 		"./testdata/excel/nesting/NestedInMap.xlsx",
// 	}
// 	for _, path := range paths {
// 		imp, err := importer.NewExcelImporter(path, nil, nil, 0)
// 		if err != nil {
// 			t.Errorf("%+v", err)
// 		}
// 		if err := imp.ExportCSV(); err != nil {
// 			t.Errorf("%+v", err)
// 		}
// 	}
// }

// func Test_CSV2Excel(t *testing.T) {
// 	paths := []string{
// 		"./testdata/excel/map/Map#*.csv",
// 		"./testdata/excel/metasheet/Metasheet#*.csv",
// 		"./testdata/excel/nesting/NestedInMap#*.csv",
// 	}
// 	for _, path := range paths {
// 		imp, err := importer.NewCSVImporter(path, nil, nil)
// 		if err != nil {
// 			t.Errorf("%+v", err)
// 		}
// 		if err := imp.ExportExcel(); err != nil {
// 			t.Errorf("%+v", err)
// 		}
// 	}
// }
