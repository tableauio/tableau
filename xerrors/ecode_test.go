package xerrors

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertEcode(t *testing.T, err error, ecode string, desc string) {
	errdesc := NewDesc(err)
	assert.Equal(t, ecode, errdesc.GetValue(keyErrCode))
	assert.Equal(t, desc, errdesc.GetValue(keyErrDesc))
}

func TestEcode(t *testing.T) {
	assertEcode(t, E3002(fmt.Errorf("no such file or directory")), "E3002", "failed to open file")
	assertEcode(t, E3003("xxx#*.csv"), "E3003", "CSV workbook glob pattern matches no files")
}
