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
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
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

func Test_Excel2CSV(t *testing.T) {
	err := rangeFilesByFormat("./testdata", format.Excel, func(bookPath string) error {
		log.Printf("path: %s", bookPath)
		imp, err := importer.NewExcelImporter(bookPath, nil, nil, 0)
		if err != nil {
			return err
		}
		return imp.ExportCSV()
	})
	if err != nil {
		t.Errorf("%+v", err)
	}
}

func Test_CSV2Excel(t *testing.T) {
	err := rangeFilesByFormat("./testdata", format.CSV, func(bookPath string) error {
		log.Printf("path: %s", bookPath)
		imp, err := importer.NewCSVImporter(bookPath, nil, nil)
		if err != nil {
			t.Errorf("%+v", err)
		}
		return imp.ExportExcel()
	})
	if err != nil {
		t.Errorf("%+v", err)
	}
}
