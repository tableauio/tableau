package xerrors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertEcode(t *testing.T, err error, ecode string, desc, text, help string) {
	errdesc := NewDesc(err)
	t.Log(err)
	t.Logf("%+v", err)
	t.Log(errdesc.String())
	assert.Equal(t, ecode, errdesc.GetValue(keyErrCode))
	assert.Equal(t, desc, errdesc.GetValue(keyErrDesc))
	assert.Equal(t, text, errdesc.GetValue(KeyReason))
	assert.Equal(t, help, errdesc.GetValue(keyHelp))
}

func TestEcode(t *testing.T) {
	e2003 := E2003("1", 3)
	assertEcode(t, e2003, "E2003", `illegal sequence number`, `value "1" does not meet sequence requirement: "sequence:3"`, `prop "sequence:3" requires value starts from "3" and increases monotonically`)
	assert.True(t, errors.Is(WrapKV(e2003, "key", "value"), ErrE2003))
}
