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
				err: NewKV("some error",
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

func TestWrapKVSameKeyNotOverwritten(t *testing.T) {
	// Base error with KeyModule set to "first"
	baseErr := WrapKV(Newf("some error"), KeyModule, "first")

	// Wrap with the same key KeyModule again
	wrappedOnce := WrapKV(baseErr, KeyModule, "second")

	// Wrap with the same key KeyModule a third time
	wrappedTwice := WrapKV(wrappedOnce, KeyModule, "third")

	tests := []struct {
		name       string
		err        error
		wantModule string
	}{
		{
			name:       "single WrapKV sets Module",
			err:        baseErr,
			wantModule: "first",
		},
		{
			name:       "second WrapKV with same key does not overwrite",
			err:        wrappedOnce,
			wantModule: "first",
		},
		{
			name:       "third WrapKV with same key does not overwrite",
			err:        wrappedTwice,
			wantModule: "first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc := NewDesc(tt.err)
			if desc == nil {
				t.Fatal("NewDesc() returned nil")
			}
			gotModule := desc.GetValue(KeyModule)
			if gotModule != tt.wantModule {
				t.Errorf("KeyModule = %v, want %v", gotModule, tt.wantModule)
			}
		})
	}
}
