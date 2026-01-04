package xerrors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var nilEcode *ecode

func assertError(t *testing.T, err error, errstr string, stackRegex string) {
	require.EqualValues(t, errstr, err.Error())
	require.EqualValues(t, errstr, fmt.Sprintf("%s", err))
	require.EqualValues(t, fmt.Sprintf("%q", errstr), fmt.Sprintf("%q", err))
	// error with stack
	errstrWithStack := fmt.Sprintf("%+v", err)
	regexErrstr := strings.ReplaceAll(errstr, `|`, `\|`) + `\|?`
	require.Regexp(t, `(?s)^`+regexErrstr+stackRegex, errstrWithStack)
}

func TestErrorf(t *testing.T) {
	err := Newf("msg %d", 111)
	assertError(t, err, "|Reason: msg 111", `\s+[^\n]*TestErrorf.*?errors_test`)
}

func TestErrorKV(t *testing.T) {
	err := NewKV("msg", "key", "val", "key2", "val2")
	assertError(t, err, "|key: val|key2: val2|Reason: msg", `\s+[^\n]*TestErrorKV.*?errors_test`)
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errstr string
	}{
		{
			name:   "nil ecode",
			err:    Wrap(nilEcode),
			errstr: "",
		},
		{
			name:   "fmt.Errorf",
			err:    Wrap(fmt.Errorf("fmt.Errorf")),
			errstr: "|fmt.Errorf",
		},
		{
			name:   "fmt.Errorf with two Wrap",
			err:    Wrap(Wrap(fmt.Errorf("fmt.Errorf"))),
			errstr: "|fmt.Errorf",
		},
		{
			name:   "Errof",
			err:    Wrap(Newf("Errorf")),
			errstr: "|Reason: Errorf",
		},
		{
			name:   "Errorf with two Wrap",
			err:    Wrap(Wrap(Newf("Errorf"))),
			errstr: "|Reason: Errorf",
		},
		{
			name:   "ErrorKV",
			err:    Wrap(NewKV("ErrorKV 1", "key", "val")),
			errstr: "|key: val|Reason: ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr, `\s+[^\n]*TestWrap.*?errors_test`)
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errstr string
	}{
		{
			name:   "fmt.Errorf",
			err:    Wrapf(fmt.Errorf("fmt.Errorf %d", 1), "Wrapf %d", 2),
			errstr: "Wrapf 2|fmt.Errorf 1",
		},
		{
			name:   "fmt.Errorf with two Wrapf",
			err:    Wrapf(Wrapf(fmt.Errorf("fmt.Errorf %d", 1), "Wrapf %d", 2), "Wrapf %d", 3),
			errstr: "Wrapf 3|Wrapf 2|fmt.Errorf 1",
		},
		{
			name:   "Errof",
			err:    Wrapf(Newf("Errorf 1"), "Wrapf 2"),
			errstr: "Wrapf 2|Reason: Errorf 1",
		},
		{
			name:   "Errorf with two Wrapf",
			err:    Wrapf(Wrapf(Newf("Errorf 1"), "Wrapf 2"), "Wrapf 3"),
			errstr: "Wrapf 3|Wrapf 2|Reason: Errorf 1",
		},
		{
			name:   "ErrorKV",
			err:    Wrapf(NewKV("ErrorKV 1", "key", "val"), "Wrapf %d", 2),
			errstr: "Wrapf 2|key: val|Reason: ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr, `\s+[^\n]*TestWrapf.*?errors_test`)
		})
	}
}

func TestWrapKV(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		errstr string
	}{
		{
			name:   "fmt.Errorf",
			err:    WrapKV(fmt.Errorf("fmt.Errorf %d", 1), "key", "val"),
			errstr: "|key: val|fmt.Errorf 1",
		},
		{
			name:   "fmt.Errorf with two WrapKV",
			err:    WrapKV(WrapKV(fmt.Errorf("fmt.Errorf %d", 1), "key", "val"), "key2", "val2"),
			errstr: "|key2: val2|key: val|fmt.Errorf 1",
		},
		{
			name:   "Errof",
			err:    WrapKV(Newf("Errorf 1"), "key", "val"),
			errstr: "|key: val|Reason: Errorf 1",
		},
		{
			name:   "Errorf with two WrapKV",
			err:    WrapKV(WrapKV(Newf("Errorf 1"), "key", "val"), "key2", "val2"),
			errstr: "|key2: val2|key: val|Reason: Errorf 1",
		},
		{
			name:   "ErrorKV",
			err:    WrapKV(NewKV("ErrorKV 1", "key", "val"), "key2", "val2"),
			errstr: "|key2: val2|key: val|Reason: ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr, `\s+[^\n]*TestWrapKV.*?errors_test`)
		})
	}
}

func TestEcode(t *testing.T) {
	assertEcode := func(err error, ecode string, desc, text, help string) {
		errdesc := NewDesc(err)
		t.Log(err)
		t.Logf("%+v", err)
		t.Log(errdesc.String())
		assert.Equal(t, ecode, errdesc.GetValue(keyErrCode))
		assert.Equal(t, desc, errdesc.GetValue(keyErrDesc))
		assert.Equal(t, text, errdesc.GetValue(KeyReason))
		assert.Equal(t, help, errdesc.GetValue(keyHelp))
	}
	e2003 := E2003("1", 3)
	assert.ErrorIs(t, e2003, newEcode("E2003", "desc"))
	assert.ErrorIs(t, e2003, ErrE2003)
	assertEcode(e2003, "E2003", `illegal sequence number`, `value "1" does not meet sequence requirement: "sequence:3"`, `prop "sequence:3" requires value starts from "3" and increases monotonically`)
	assert.ErrorIs(t, WrapKV(e2003, "key", "val"), ErrE2003)
}
