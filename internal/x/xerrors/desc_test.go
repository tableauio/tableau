package xerrors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		{
			name: "plain fmt.Errorf",
			args: args{
				err: fmt.Errorf("plain error"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewDesc(tt.args.err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestNewDescPlainError(t *testing.T) {
	err := fmt.Errorf("plain error")
	desc := NewDesc(err)
	require.NotNil(t, desc)
	assert.Equal(t, "plain error", desc.ErrString(false))
}

func TestNewDescNil(t *testing.T) {
	assert.Nil(t, NewDesc(nil))
}

func TestWrapKVInnermostWins(t *testing.T) {
	// innermost (earliest) WrapKV value wins on key conflicts.
	wrapFirst := WrapKV(Newf("some error"), KeyModule, "first")
	wrapSecond := WrapKV(wrapFirst, KeyModule, "second")
	wrapThird := WrapKV(wrapSecond, KeyModule, "third")

	tests := []struct {
		name       string
		err        error
		wantModule string
	}{
		{
			name:       "single WrapKV sets Module",
			err:        wrapFirst,
			wantModule: "first",
		},
		{
			name:       "second WrapKV: innermost (first) wins",
			err:        wrapSecond,
			wantModule: "first",
		},
		{
			name:       "third WrapKV: innermost (first) wins",
			err:        wrapThird,
			wantModule: "first",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDesc(tt.err)
			require.NotNil(t, d)
			assert.Equal(t, tt.wantModule, d.GetValue(KeyModule))
		})
	}
}

// ---- NewDesc with errors.Join ----

func TestNewDescAllNilJoin(t *testing.T) {
	joined := errors.Join(nil, nil)
	assert.Nil(t, NewDesc(joined))
}

func TestNewDescSingleChildJoin(t *testing.T) {
	// errors.Join with one non-nil child → *Desc, not *MultiDesc.
	e := E2003("1", 3)
	joined := errors.Join(nil, e)
	d := NewDesc(joined)
	require.NotNil(t, d)

	wantNoDebug := `error[E2003]: illegal sequence number
Reason: value "1" does not meet sequence requirement: "sequence:3"
Help: prop "sequence:3" requires value starts from "3" and increases monotonically
`
	assert.Equal(t, wantNoDebug, d.ErrString(false))

	wantDebug := `Debugging: 
	Module: default
	ErrCode: E2003
	ErrDesc: illegal sequence number
	Reason: value "1" does not meet sequence requirement: "sequence:3"
	Help: prop "sequence:3" requires value starts from "3" and increases monotonically

` + wantNoDebug
	assert.Equal(t, wantDebug, d.ErrString(true))
}

func TestNewDescMultipleChildren(t *testing.T) {
	// errors.Join with two non-nil children → *Desc with Children.
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := E2027("id: must be positive", "0")
	joined := errors.Join(e1, e2)

	md := NewDesc(joined)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 2)

	for i, d := range md.Children() {
		assert.Equal(t, "E2027", d.ErrCode(), "Children()[%d].ErrCode()", i)
	}

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "0" violates rule: id: must be positive
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, md.ErrString(false))
}

func TestNewDescMixedErrors(t *testing.T) {
	// One structured error + one plain error.
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := fmt.Errorf("plain error")
	joined := errors.Join(e1, e2)

	md := NewDesc(joined)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 2)

	assert.Equal(t, "E2027", md.Children()[0].ErrCode())
	assert.Equal(t, "", md.Children()[1].ErrCode())
	assert.Equal(t, "plain error", md.Children()[1].ErrString(false))

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] plain error`
	assert.Equal(t, wantNoDebug, md.ErrString(false))
}

func TestNewDescPerErrorMetadata(t *testing.T) {
	// Each child carries its own Reason.
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := E2027("id: must be positive", "0")
	joined := errors.Join(e1, e2)

	md := NewDesc(joined)
	require.NotNil(t, md)

	assert.Equal(t, `"toolong" violates rule: name: value length must be at most 10 characters`,
		md.Children()[0].GetValue(KeyReason))
	assert.Equal(t, `"0" violates rule: id: must be positive`,
		md.Children()[1].GetValue(KeyReason))
}

func TestNewDescErrStringNoDebug(t *testing.T) {
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := E2027("id: must be positive", "0")
	joined := errors.Join(e1, e2)

	md := NewDesc(joined)
	want := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "0" violates rule: id: must be positive
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, md.ErrString(false))
}

// TestNewDescWrapKVOverJoin covers the key scenario: outer WrapKV fields
// (Module, BookName, SheetName) are merged into every child Desc, while
// each child's own fields (Reason) still win over the outer ones.
func TestNewDescWrapKVOverJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")
	joined := errors.Join(e1, e2)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	md := NewDesc(wrapped)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 2)

	for i, d := range md.Children() {
		// Outer fields must be present in every child.
		assert.Equal(t, ModuleConf, d.GetValue(KeyModule), "child[%d] Module", i)
		assert.Equal(t, "Validate#*.csv", d.GetValue(KeyBookName), "child[%d] BookName", i)
		assert.Equal(t, "ValidateFieldLevel", d.GetValue(KeySheetName), "child[%d] SheetName", i)
		// Each child must still carry its own ErrCode and Reason.
		assert.Equal(t, "E2027", d.ErrCode(), "child[%d] ErrCode", i)
	}

	// The two children must have different Reason values.
	assert.Equal(t, `"800" violates rule: item_map[1].score: value must be > 0 and <= 100`,
		md.Children()[0].GetValue(KeyReason))
	assert.Equal(t, `"950" violates rule: item_map[2].score: value must be > 0 and <= 100`,
		md.Children()[1].GetValue(KeyReason))

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Workbook: Validate#*.csv 
Worksheet: ValidateFieldLevel 
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item_map[1].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Validate#*.csv 
Worksheet: ValidateFieldLevel 
DataCellPos: <no value>
DataCell: <no value>
Reason: "950" violates rule: item_map[2].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, md.ErrString(false))
}

// TestNewDescWrapKVOverJoinSingleChild covers the single-child path:
// WrapKV wraps an errors.Join with exactly one non-nil child → *Desc.
func TestNewDescWrapKVOverJoinSingleChild(t *testing.T) {
	e1 := E2027("score: value must be > 0 and <= 100", "800")
	joined := errors.Join(e1)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	d := NewDesc(wrapped)
	require.NotNil(t, d)

	assert.Equal(t, ModuleConf, d.GetValue(KeyModule))
	assert.Equal(t, "Validate#*.csv", d.GetValue(KeyBookName))
	assert.Equal(t, "E2027", d.ErrCode())
	assert.Equal(t, `"800" violates rule: score: value must be > 0 and <= 100`,
		d.GetValue(KeyReason))

	wantNoDebug := `error[E2027]: protovalidate violation
Workbook: Validate#*.csv 
Worksheet: ValidateFieldLevel 
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, d.ErrString(false))
}

// TestNewDescOuterFieldDoesNotOverrideInner ensures that when both the outer
// WrapKV and an inner child carry the same key, the inner (child) value wins.
func TestNewDescOuterFieldDoesNotOverrideInner(t *testing.T) {
	// Child carries Module=confgen via its own WrapKV.
	inner := WrapKV(Newf("inner error"), KeyModule, ModuleConf)
	joined := errors.Join(inner)
	// Outer tries to set Module=protogen – inner must win.
	wrapped := WrapKV(joined, KeyModule, ModuleProto)

	d := NewDesc(wrapped)
	require.NotNil(t, d)
	assert.Equal(t, ModuleConf, d.GetValue(KeyModule), "inner Module should win over outer")
}

// TestNewDescTwoLayerJoinMultiOuter covers the scenario where the outer join
// has multiple children, each of which wraps an inner join:
//
//	outerJoinError                              ← Generator collector (multiple sheets)
//	  ├── errs[0]: WrapKV(innerJoin1, ...)      ← sheet1 errors
//	  │     └── innerJoin1: {err1, err2}
//	  └── errs[1]: WrapKV(innerJoin2, ...)      ← sheet2 errors
//	        └── innerJoin2: {err3, err4}
//
// All four leaf errors must appear as a flat numbered list.
func TestNewDescTwoLayerJoinMultiOuter(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")
	e3 := E2027("item_map[3].score: value must be > 0 and <= 100", "0")
	e4 := E2027("item_map[4].score: value must be > 0 and <= 100", "-1")

	innerJoin1 := &joinError{errs: []error{e1, e2}}
	wrapped1 := WrapKV(innerJoin1,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "Sheet1",
	)
	innerJoin2 := &joinError{errs: []error{e3, e4}}
	wrapped2 := WrapKV(innerJoin2,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "Sheet2",
	)
	outerJoin := &joinError{errs: []error{wrapped1, wrapped2}}

	md := NewDesc(outerJoin)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 4, "all four leaf errors must be flattened as children")

	assert.Equal(t, "Sheet1", md.Children()[0].GetValue(KeySheetName))
	assert.Equal(t, "Sheet1", md.Children()[1].GetValue(KeySheetName))
	assert.Equal(t, "Sheet2", md.Children()[2].GetValue(KeySheetName))
	assert.Equal(t, "Sheet2", md.Children()[3].GetValue(KeySheetName))

	for i, d := range md.Children() {
		assert.Equal(t, ModuleConf, d.GetValue(KeyModule), "child[%d] Module", i)
		assert.Equal(t, "Validate#*.csv", d.GetValue(KeyBookName), "child[%d] BookName", i)
		assert.Equal(t, "E2027", d.ErrCode(), "child[%d] ErrCode", i)
	}
}
// TestNewDescThreeLayerJoin covers 3-level nesting to verify arbitrary-depth
// flattening:
//
//	WrapKV(outerJoin, BookName)             ← top-level WrapKV
//	  └── outerJoinError                   ← multiple children
//	        ├── WrapKV(innerJoin1, Sheet1)
//	        │     └── innerJoin1: {err1, err2}
//	        └── WrapKV(innerJoin2, Sheet2)
//	              └── innerJoin2: {err3, err4}
//
// All four leaf errors must be flattened, each carrying BookName + SheetName.
func TestNewDescThreeLayerJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")
	e3 := E2027("item_map[3].score: value must be > 0 and <= 100", "0")
	e4 := E2027("item_map[4].score: value must be > 0 and <= 100", "-1")

	innerJoin1 := &joinError{errs: []error{e1, e2}}
	wrapped1 := WrapKV(innerJoin1, KeySheetName, "Sheet1")
	innerJoin2 := &joinError{errs: []error{e3, e4}}
	wrapped2 := WrapKV(innerJoin2, KeySheetName, "Sheet2")

	outerJoin := &joinError{errs: []error{wrapped1, wrapped2}}
	top := WrapKV(outerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
	)

	md := NewDesc(top)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 4, "all four leaf errors must be flattened")

	assert.Equal(t, "Sheet1", md.Children()[0].GetValue(KeySheetName))
	assert.Equal(t, "Sheet1", md.Children()[1].GetValue(KeySheetName))
	assert.Equal(t, "Sheet2", md.Children()[2].GetValue(KeySheetName))
	assert.Equal(t, "Sheet2", md.Children()[3].GetValue(KeySheetName))

	for i, d := range md.Children() {
		assert.Equal(t, ModuleConf, d.GetValue(KeyModule), "child[%d] Module", i)
		assert.Equal(t, "Validate#*.csv", d.GetValue(KeyBookName), "child[%d] BookName", i)
		assert.Equal(t, "E2027", d.ErrCode(), "child[%d] ErrCode", i)
	}
}

// TestNewDescTwoLayerJoin covers the real-world scenario produced by the
// Generator+parser collector pair:
//
//	outerJoinError                         ← Generator collector.Join()
//	  └── errs[0]: WrapKV(innerJoinError)  ← WrapKV adds Module/BookName/SheetName
//	        └── innerJoinError             ← parser collector.Join()
//	              ├── errs[0]: err1
//	              └── errs[1]: err2
//
// NewDesc must recursively expand the inner join so that both err1 and err2
// appear as separate children, each carrying the outer WrapKV fields.
func TestNewDescTwoLayerJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")

	// Inner join (parser collector)
	innerJoin := &joinError{errs: []error{e1, e2}}
	// WrapKV adds sheet-level context
	wrapped := WrapKV(innerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)
	// Outer join (Generator collector) – single child
	outerJoin := &joinError{errs: []error{wrapped}}

	md := NewDesc(outerJoin)
	require.NotNil(t, md)
	require.Len(t, md.Children(), 2, "both inner errors must be expanded as children")

	for i, d := range md.Children() {
		assert.Equal(t, ModuleConf, d.GetValue(KeyModule), "child[%d] Module", i)
		assert.Equal(t, "Validate#*.csv", d.GetValue(KeyBookName), "child[%d] BookName", i)
		assert.Equal(t, "ValidateFieldLevel", d.GetValue(KeySheetName), "child[%d] SheetName", i)
		assert.Equal(t, "E2027", d.ErrCode(), "child[%d] ErrCode", i)
	}

	assert.Equal(t, `"800" violates rule: item_map[1].score: value must be > 0 and <= 100`,
		md.Children()[0].GetValue(KeyReason))
	assert.Equal(t, `"950" violates rule: item_map[2].score: value must be > 0 and <= 100`,
		md.Children()[1].GetValue(KeyReason))

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Workbook: Validate#*.csv 
Worksheet: ValidateFieldLevel 
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item_map[1].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Validate#*.csv 
Worksheet: ValidateFieldLevel 
DataCellPos: <no value>
DataCell: <no value>
Reason: "950" violates rule: item_map[2].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, md.ErrString(false))
}
