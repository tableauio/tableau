package xerrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorf(t *testing.T) {
	type args struct {
		code int
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "with stack",
			args: args{
				code: -1,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// err := Errorf(tt.args.code, "add some msg %d", 111)
			err := Errorf("add some msg %d", 111)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
		})
	}
}

func TestWrapf(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "with stack",
			args: args{
				err: Errorf("some error %d", 111),
			},
		},
		{
			name: "fmt.Errorf",
			args: args{
				err: Wrapf(fmt.Errorf("fmt.Errorf"), "wrapf"),
			},
		},
		{
			name: "Errorf",
			args: args{
				err: Wrapf(Wrapf(Errorf("Errorf"), "Wrapf"), "wrapf"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrapf(tt.args.err, "add some msg %d", 111)
			t.Logf("err: %+v", err)
			t.Logf("err: %s", err)
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
	assertEcode(e2003, "E2003", `illegal sequence number`, `value "1" does not meet sequence requirement: "sequence:3"`, `prop "sequence:3" requires value starts from "3" and increases monotonically`)
	assert.True(t, errors.Is(WrapKV(e2003, "key", "value"), ErrE2003))
}

func TestEcodeStack(t *testing.T) {
	// TODO: add test for stacktrace levels
	// t.Logf("%+v", ErrorKV("test", "key", "value"))
	// t.Logf("%+v", Errorf("test: %s:%s", "key", "value"))
	// t.Logf("%+v", WrapKV(ErrE2003, "key", "value"))
	t.Logf("%+v", Wrapf(Wrapf(ErrE2003, "msg1"), "msg2"))
}

func Test_abc(t *testing.T) {
	err := E2003("1", 3)
	t.Logf("err: %+v\n", err)

	err2 := WrapKV(err,
		KeyModule, ModuleConf,
		KeyBookName, "mybook",
		KeySheetName, "mysheet",
		KeyPBMessage, "mymessage",
	)
	t.Logf("err2: %v\n", err2)
	t.Logf("err2: %+v\n", err2)

	err3 := Wrapf(err2,
		"Test_abc failed",
	)
	t.Logf("err3: %v\n", err3)
	t.Logf("err3: %+v\n", err3)
}
