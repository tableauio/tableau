package dev

import (
	"fmt"
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/test/dev/protoconf"
)

func Test_LoadJSON(t *testing.T) {
	msg := &protoconf.Activity{}
	err := load.Load(msg, "./testdata/json/", format.JSON)
	if err != nil {
		t.Error(err)
	}
	fmt.Println(msg)
}

func Test_LoadCSVFailed(t *testing.T) {
	msg := &protoconf.Activity{}
	err := load.Load(msg, "./testdata/", format.CSV, load.SubdirRewrites(map[string]string{"excel": ""}))
	if err == nil {
		t.Errorf("should have failed")
	}
	// fmt.Printf("%+v\n", err)
}

func Test_LoadExcel(t *testing.T) {
	msg := &protoconf.Hero{}
	err := load.Load(msg, "./testdata/", format.Excel)
	if err != nil {
		t.Errorf("%+v", err)
	}
	fmt.Println(msg)
}

func Test_LoadExcelFailed(t *testing.T) {
	msg := &protoconf.Activity{}
	err := load.Load(msg, "./testdata/", format.Excel, load.SubdirRewrites(map[string]string{"excel": ""}))
	if err == nil {
		t.Errorf("shoud have failed")
	}
	fmt.Printf("%+v\n", err)
}
