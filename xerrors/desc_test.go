package xerrors

import (
	"testing"
)

func TestNewDesc(t *testing.T) {
	type args struct {
		err error
	}
	tests := []struct {
		name    string
		args    args
		wantNil bool
	}{
		{
			name: "nil error",
			args: args{
				err: nil,
			},
			wantNil: true,
		},
		{
			name: "general error",
			args: args{
				err: ErrorKV("some error",
					KeyPBFieldType, "Item",
					KeyPBFieldOpts, "{unique: true}"),
			},
		},
		{
			name: "ecode",
			args: args{
				err: E0001("Item", "Item.xlsx"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDesc(tt.args.err)
			if (got == nil) != tt.wantNil {
				t.Errorf("NewDesc() *Desc == nil: %v, wantNil %v", got == nil, tt.wantNil)
			}
			if got != nil {
				t.Logf("%s: %s: %s", tt.name, got.ErrCode(), got)
			}
		})
	}
}
