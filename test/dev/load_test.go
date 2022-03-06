package dev

import (
	"testing"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/load"
	"github.com/tableauio/tableau/test/dev/protoconf"
)

func Test_LoadJSON(t *testing.T) {
	msg := &protoconf.Activity{}
	err := load.Load(msg, "./_conf/", format.JSON)
	if err != nil {
		t.Error(err)
	}
}

func Test_LoadExcel(t *testing.T) {
	msg := &protoconf.Hero{}
	err := load.Load(msg, "./testdata/", format.Excel)
	if err != nil {
		t.Errorf("%+v, %s", err, msg)
	}
}
