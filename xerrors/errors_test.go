package xerrors

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)
var nilEcode *ecode

func assertError(t *testing.T, err error, errstr string) {
	t.Helper()
	require.EqualValues(t, errstr, err.Error())
	require.EqualValues(t, errstr, fmt.Sprintf("%s", err))
	require.EqualValues(t, fmt.Sprintf("%q", errstr), fmt.Sprintf("%q", err))
	// %+v should start with NewDesc(err).String(), followed by the stack trace.
	plusV := fmt.Sprintf("%+v", err)
	d := NewDesc(err)
	var descStr string
	if d != nil {
		descStr = d.String()
	}
	require.True(t, strings.HasPrefix(plusV, descStr),
		"%+v should start with NewDesc(err).String():\ngot:  %q\nwant prefix: %q", plusV, descStr)
}

func TestErrorf(t *testing.T) {
	err := Newf("msg %d", 111)
	assertError(t, err, "msg 111")
}

func TestErrorKV(t *testing.T) {
	err := NewKV("msg", "key", "val", "key2", "val2")
	// Error() returns the clean message; KV fields are metadata only.
	assertError(t, err, "msg")
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
			errstr: "fmt.Errorf",
		},
		{
			name:   "fmt.Errorf with two Wrap",
			err:    Wrap(Wrap(fmt.Errorf("fmt.Errorf"))),
			errstr: "fmt.Errorf",
		},
		{
			name:   "Errof",
			err:    Wrap(Newf("Errorf")),
			errstr: "Errorf",
		},
		{
			name:   "Errorf with two Wrap",
			err:    Wrap(Wrap(Newf("Errorf"))),
			errstr: "Errorf",
		},
		{
			name:   "ErrorKV",
			err:    Wrap(NewKV("ErrorKV 1", "key", "val")),
			errstr: "ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr)
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
			errstr: "Wrapf 2: fmt.Errorf 1",
		},
		{
			name:   "fmt.Errorf with two Wrapf",
			err:    Wrapf(Wrapf(fmt.Errorf("fmt.Errorf %d", 1), "Wrapf %d", 2), "Wrapf %d", 3),
			errstr: "Wrapf 3: Wrapf 2: fmt.Errorf 1",
		},
		{
			name:   "Errof",
			err:    Wrapf(Newf("Errorf 1"), "Wrapf 2"),
			errstr: "Wrapf 2: Errorf 1",
		},
		{
			name:   "Errorf with two Wrapf",
			err:    Wrapf(Wrapf(Newf("Errorf 1"), "Wrapf 2"), "Wrapf 3"),
			errstr: "Wrapf 3: Wrapf 2: Errorf 1",
		},
		{
			name:   "ErrorKV",
			err:    Wrapf(NewKV("ErrorKV 1", "key", "val"), "Wrapf %d", 2),
			errstr: "Wrapf 2: ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr)
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
			// WrapKV adds metadata only; Error() delegates to cause.
			name:   "fmt.Errorf",
			err:    WrapKV(fmt.Errorf("fmt.Errorf %d", 1), "key", "val"),
			errstr: "fmt.Errorf 1",
		},
		{
			name:   "fmt.Errorf with two WrapKV",
			err:    WrapKV(WrapKV(fmt.Errorf("fmt.Errorf %d", 1), "key", "val"), "key2", "val2"),
			errstr: "fmt.Errorf 1",
		},
		{
			name:   "Errof",
			err:    WrapKV(Newf("Errorf 1"), "key", "val"),
			errstr: "Errorf 1",
		},
		{
			name:   "Errorf with two WrapKV",
			err:    WrapKV(WrapKV(Newf("Errorf 1"), "key", "val"), "key2", "val2"),
			errstr: "Errorf 1",
		},
		{
			name:   "ErrorKV",
			err:    WrapKV(NewKV("ErrorKV 1", "key", "val"), "key2", "val2"),
			errstr: "ErrorKV 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, tt.err, tt.errstr)
		})
	}
}

func TestEcode(t *testing.T) {
	assertEcode := func(err error, ecode string, desc, text, help string) {
		d := NewDesc(err)
		t.Log(err)
		t.Logf("%+v", err)
		t.Log(d.String())
		assert.Equal(t, ecode, d.GetValue(keyErrCode))
		assert.Equal(t, desc, d.GetValue(keyErrDesc))
		assert.Equal(t, text, d.GetValue(KeyReason))
		assert.Equal(t, help, d.GetValue(keyHelp))
	}
	e2003 := E2003("1", 3)
	assert.ErrorIs(t, e2003, newEcode("E2003", "desc"))
	assert.ErrorIs(t, e2003, ErrE2003)
	assertEcode(e2003, "E2003", `illegal sequence number`, `value "1" does not meet sequence requirement: "sequence:3"`, `prop "sequence:3" requires value starts from "3" and increases monotonically`)
	assert.ErrorIs(t, WrapKV(e2003, "key", "val"), ErrE2003)
}
