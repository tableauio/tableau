// Package functest is aimed for Functional Testing.
//
// "Functional Testing" is a black box testing technique, where
// the functionality of the application is tested to generate
// the desired output on providing a certain input.
//
// Test cases basically comprise of the following parts:
//   - Test Summary
//   - Prerequisites (if any)
//   - Test case input steps
//   - Test data (if any)
//   - Expected output
//   - Notes (if any)
//
// "Requirement-Based" and "Business scenario-based" are the two
// forms of functional testing that are carried out.
//
// In Requirement based testing, test cases are created as per the
// requirement and tested accordingly. In a Business scenario based
// functional testing, testing is performed by keeping in mind all
// the scenarios from a business perspective.
package functest

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/xerrors"
)

func Test_CompareGeneratedProto(t *testing.T) {
	err := genProto("DEBUG")
	if err != nil {
		t.Errorf("%+v", err)
		t.Fatalf("%s", xerrors.NewDesc(err))
	}
	oldConfDir := "proto"
	newConfDir := "_proto"
	files, err := os.ReadDir(oldConfDir)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".proto") {
			continue
		}
		oldPath := filepath.Join(oldConfDir, file.Name())
		absOldPath, err := filepath.Abs(oldPath)
		require.NoError(t, err)
		oldfile, err := os.Open(oldPath)
		require.NoError(t, err)

		newPath := filepath.Join(newConfDir, file.Name())
		absNewPath, err := filepath.Abs(newPath)
		require.NoError(t, err)
		newfile, err := os.Open(newPath)
		require.NoError(t, err)

		oscan := bufio.NewScanner(oldfile)
		nscan := bufio.NewScanner(newfile)

		line := 0
		for {
			sok := oscan.Scan()
			dok := nscan.Scan()
			line++
			require.Equalf(t, sok, dok, "unequal line count: %s:%d -> %s:%d", absOldPath, line, absNewPath, line)
			if !sok || !dok {
				break
			}
			if line == 1 {
				// as the first line is one line comment
				// (including dynamic version number), ignore it.
				continue
			}
			require.Equalf(t, string(oscan.Bytes()), string(nscan.Bytes()), "unequal line content: %s:%d -> %s:%d", absOldPath, line, absNewPath, line)
		}
	}
}

func Test_CompareGeneratedJSON(t *testing.T) {
	err := genConf("DEBUG")
	if err != nil {
		t.Errorf("%+v", err)
		t.Fatalf("%s", xerrors.NewDesc(err))
	}

	oldConfDir := "conf"
	newConfDir := "_conf"
	files, err := os.ReadDir(oldConfDir)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}
		oldPath := filepath.Join(oldConfDir, file.Name())
		absOldPath, err := filepath.Abs(oldPath)
		require.NoError(t, err)
		newPath := filepath.Join(newConfDir, file.Name())
		absNewPath, err := filepath.Abs(newPath)
		require.NoError(t, err)
		oldData, err := os.ReadFile(oldPath)
		require.NoError(t, err)
		newData, err := os.ReadFile(newPath)
		require.NoError(t, err)

		t.Logf("compare json file: %s\n", file.Name())
		require.JSONEqf(t, string(oldData), string(newData), "%s -> %s content not same.", absOldPath, absNewPath)
	}
}

func Test_Excel2CSV(t *testing.T) {
	err := fs.RangeFilesByFormat("./testdata/excel", format.Excel, func(bookPath string) error {
		// log.Printf("path: %s", bookPath)
		imp, err := importer.NewExcelImporter(bookPath, nil, nil, 0, false)
		if err != nil {
			return err
		}
		return imp.ExportCSV()
	})
	require.NoError(t, err)
}

func Test_CSV2Excel(t *testing.T) {
	err := fs.RangeFilesByFormat("./testdata/excel", format.CSV, func(bookPath string) error {
		// log.Printf("path: %s", bookPath)
		imp, err := importer.NewCSVImporter(bookPath, nil, nil, 0, false)
		if err != nil {
			return err
		}
		return imp.ExportExcel()
	})
	require.NoError(t, err)
}
