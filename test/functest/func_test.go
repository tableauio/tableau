// Package main is aimed for Functional Testing.
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
package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/xerrors"
)

func Test_CompareGeneratedProto(t *testing.T) {
	err := genProto("DEBUG", "FULL")
	require.NoErrorf(t, err, "%+v\n%+v", err, xerrors.NewDesc(err))
	err = EqualTextFile(".proto", "proto", "_proto", 2)
	require.NoError(t, err)
}

func Test_CompareGeneratedJSON(t *testing.T) {
	err := genConf("DEBUG", "FULL")
	require.NoErrorf(t, err, "%+v\n%+v", err, xerrors.NewDesc(err))
	err = EqualTextFile(".json", "conf", "_conf", 1)
	require.NoError(t, err)
}

func Test_CSV2Excel(t *testing.T) {
	for _, dir := range excelDirs {
		err := xfs.RangeFilesByFormat(dir, format.CSV, func(bookPath string) error {
			// log.Printf("path: %s", bookPath)
			imp, err := importer.NewCSVImporter(bookPath, nil, nil, 0, false)
			if err != nil {
				return err
			}
			return imp.ExportExcel()
		})
		require.NoError(t, err)
	}
}

func Test_Excel2CSV(t *testing.T) {
	for _, dir := range excelDirs {
		err := xfs.RangeFilesByFormat(dir, format.Excel, func(bookPath string) error {
			// log.Printf("path: %s", bookPath)
			imp, err := importer.NewExcelImporter(bookPath, nil, nil, 0, false)
			if err != nil {
				return err
			}
			return imp.ExportCSV()
		})
		require.NoError(t, err)
	}
}

var excelDirs []string

func init() {
	excelDirs = []string{
		"./testdata/default/excel",
		"./testdata/custom/excel",
	}
}
