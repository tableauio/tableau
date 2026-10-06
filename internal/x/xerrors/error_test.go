package xerrors

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewError(t *testing.T) {
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
			got := NewError(tt.args.err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestNewErrorPlainError(t *testing.T) {
	err := fmt.Errorf("plain error")
	desc := NewError(err)
	require.NotNil(t, desc)
	assert.Equal(t, "plain error", desc.stringify(false))
}

func TestNewErrorNil(t *testing.T) {
	assert.Nil(t, NewError(nil))
}

// TestWrapKVInnermostWins verifies innermost (earliest) WrapKV value wins on key conflicts.
func TestWrapKVInnermostWins(t *testing.T) {
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
			d := NewError(tt.err)
			require.NotNil(t, d)
			assert.Equal(t, tt.wantModule, detailFields(d)[KeyModule])
		})
	}
}

func TestNewErrorAllNilJoin(t *testing.T) {
	joined := errors.Join(nil, nil)
	assert.Nil(t, NewError(joined))
}

// TestNewErrorSingleChildJoin verifies errors.Join with one non-nil child → single Error.
func TestNewErrorSingleChildJoin(t *testing.T) {
	e := E2003("1", 3)
	joined := errors.Join(nil, e)
	d := NewError(joined)
	require.NotNil(t, d)

	wantNoDebug := `error[E2003]: illegal sequence number
Reason: value "1" does not meet sequence requirement: "sequence:3"
Help: prop "sequence:3" requires value starts from "3" and increases monotonically
`
	assert.Equal(t, wantNoDebug, d.stringify(false))

	debugGot := d.stringify(true)
	assert.True(t, strings.HasPrefix(debugGot, wantNoDebug), "debug output should start with the non-debug summary")
	assert.Contains(t, debugGot, "\n--- debugging ---\n", "debug output should contain debugging header")
	assert.Regexp(t, regexp.MustCompile(`xerrors\.TestNewErrorSingleChildJoin`), debugGot, "debug output should contain stack trace")
}

// TestNewErrorMultipleChildren verifies errors.Join with multiple children → numbered list.
func TestNewErrorMultipleChildren(t *testing.T) {
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := E2027("id: must be positive", "0")
	joined := errors.Join(e1, e2)

	md := NewError(joined)
	require.NotNil(t, md)
	require.Len(t, md.Details, 2)

	for i, d := range md.Details {
		assert.Equal(t, "E2027", d.fields()[KeyErrCode], "children[%d].fields()[KeyErrCode]", i)
	}

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "0" violates rule: id: must be positive
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, md.stringify(false))
}

// TestNewErrorMixedErrors verifies one structured error + one plain error in a join.
func TestNewErrorMixedErrors(t *testing.T) {
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := fmt.Errorf("plain error")
	joined := errors.Join(e1, e2)

	md := NewError(joined)
	require.NotNil(t, md)
	require.Len(t, md.Details, 2)

	assert.Equal(t, "E2027", md.Details[0].fields()[KeyErrCode])
	assert.Nil(t, md.Details[1].fields()[KeyErrCode])
	assert.Equal(t, "plain error", md.Details[1].String())

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] plain error`
	assert.Equal(t, wantNoDebug, md.stringify(false))
}

// TestNewErrorWrapKVOverJoin verifies shared fields merge into every child Error,
// while each child's own fields (Reason) still win.
//
//	WrapKV(errors.Join(e1, e2), Module, BookName, SheetName)
//	  └── joinError
//	        ├── e1 (E2027)
//	        └── e2 (E2027)
func TestNewErrorWrapKVOverJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")
	joined := errors.Join(e1, e2)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	md := NewError(wrapped)
	require.NotNil(t, md)

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
	assert.Equal(t, wantNoDebug, md.stringify(false))
}

// TestNewErrorWrapKVOverJoinSingleChild verifies WrapKV wrapping errors.Join
// with exactly one non-nil child → single Error (not numbered list).
func TestNewErrorWrapKVOverJoinSingleChild(t *testing.T) {
	e1 := E2027("score: value must be > 0 and <= 100", "800")
	joined := errors.Join(e1)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	d := NewError(wrapped)
	require.NotNil(t, d)

	wantNoDebug := `error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: ValidateFieldLevel
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, d.stringify(false))
}

// TestNewErrorOuterFieldDoesNotOverrideInner verifies inner WrapKV value wins
// over outer WrapKV on key conflicts.
func TestNewErrorOuterFieldDoesNotOverrideInner(t *testing.T) {
	inner := WrapKV(Newf("inner error"), KeyModule, ModuleConf)
	joined := errors.Join(inner)
	wrapped := WrapKV(joined, KeyModule, ModuleProto)

	d := NewError(wrapped)
	require.NotNil(t, d)

	want := `error[E0004]: unknown error
Workbook: <no value>
Worksheet: <no value>
DataCellPos: <no value>
DataCell: <no value>
Reason: inner error
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewErrorTwoLayerJoinMultiOuter verifies flattening when outer join has
// multiple children, each wrapping an inner join:
//
//	outerJoinError
//	  ├── WrapKV(innerJoin1, Sheet1)
//	  │     └── innerJoin1: {e1, e2}
//	  └── WrapKV(innerJoin2, Sheet2)
//	        └── innerJoin2: {e3, e4}
//
// All 4 leaf errors must appear as a flat numbered list.
func TestNewErrorTwoLayerJoinMultiOuter(t *testing.T) {
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

	md := NewError(outerJoin)
	require.NotNil(t, md)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item_map[1].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "950" violates rule: item_map[2].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: item_map[3].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[4] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: "-1" violates rule: item_map[4].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, md.stringify(false))
}

// TestNewErrorThreeLayerJoin verifies arbitrary-depth flattening with layered scopes:
//
//	WrapKV(outerJoin, Module, BookName)
//	  └── outerJoinError
//	        ├── WrapKV(innerJoin1, Sheet1)
//	        │     └── innerJoin1: {e1, e2}
//	        └── WrapKV(innerJoin2, Sheet2)
//	              └── innerJoin2: {e3, e4}
//
// All 4 leaf errors carry BookName + SheetName.
func TestNewErrorThreeLayerJoin(t *testing.T) {
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

	md := NewError(top)
	require.NotNil(t, md)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item_map[1].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "950" violates rule: item_map[2].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: item_map[3].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[4] error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: "-1" violates rule: item_map[4].score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, md.stringify(false))
	assert.Equal(t, want, top.Error())
	assert.Equal(t, want, fmt.Sprintf("%s", top))
	assert.Equal(t, want, fmt.Sprintf("%v", top))

	// Verify debug output: each child should have its own stack trace.
	debugGot := md.stringify(true)
	t.Log(debugGot)
	for i := 1; i <= 4; i++ {
		assert.Contains(t, debugGot, fmt.Sprintf("[%d] error[E2027]", i), "child %d should have error header", i)
	}
	assert.Equal(t, 4, strings.Count(debugGot, "\n--- debugging ---\n"), "debug output should contain exactly 4 debugging headers")
	assert.Equal(t, 4, strings.Count(debugGot, "xerrors.TestNewErrorThreeLayerJoin"), "debug output should contain exactly 4 stack traces")
}

// TestNewErrorTwoLayerJoin verifies a regular join containing a wrapped inner join:
//
//	outerJoinError                              ← Generator collector.Join()
//	  └── WrapKV(innerJoinError, Module, Book, Sheet)
//	        └── innerJoinError                  ← parser collector.Join()
//	              ├── e1 (E2027)
//	              └── e2 (E2027)
//
// Both leaf errors appear with the outer WrapKV fields.
func TestNewErrorTwoLayerJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")

	innerJoin := &joinError{errs: []error{e1, e2}}
	wrapped := WrapKV(innerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)
	outerJoin := &joinError{errs: []error{wrapped}}

	md := NewError(outerJoin)
	require.NotNil(t, md)

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
	assert.Equal(t, wantNoDebug, md.stringify(false))
}

// TestNewError_CollectedMarkerTransparent verifies that the collected wrapper
// from Collector.Join() is transparent to NewError.
func TestNewError_CollectedMarkerTransparent(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(E2027("score: value must be > 0", "0"))
	_ = c.Collect(E2027("name: too long", "abcdefghijk"))

	joined := c.Join()
	require.NotNil(t, joined)

	var ce *collected
	require.True(t, errors.As(joined, &ce), "Join() must return collected-wrapped error")

	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_CollectedSingleError verifies single error in collector → single Error (no numbered list).
func TestNewError_CollectedSingleError(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(E2027("score: value must be > 0", "0"))

	joined := c.Join()
	d := NewError(joined)
	require.NotNil(t, d)

	want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_TwoLevelCollectorTree verifies 2-level collector flattening:
//
//	root collector
//	  └── child collector
//	        ├── E2027
//	        └── E2005
func TestNewError_TwoLevelCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	_ = child.Collect(E2027("score: must be > 0", "0"))
	_ = child.Collect(E2005("duplicate_key"))

	joined := root.Join()
	require.NotNil(t, joined)

	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2005]: map key not unique
Reason: map key "duplicate_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_TwoLevelWithWrapKV verifies WrapKV on individual errors before Collect:
//
//	root collector
//	  └── child collector
//	        ├── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"ItemConf")
//	        └── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"Item2Conf")
func TestNewError_TwoLevelWithWrapKV(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	_ = child.Collect(WrapKV(E2027("item.score: must be > 0 and <= 100", "800"),
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "ItemConf",
	))
	_ = child.Collect(WrapKV(E2027("item.name: too long", "abcdefghijklmnop"),
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "Item2Conf",
	))

	joined := root.Join()
	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: ItemConf
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item.score: must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: Item2Conf
DataCellPos: <no value>
DataCell: <no value>
Reason: "abcdefghijklmnop" violates rule: item.name: too long
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_TwoLevelCollectorScope verifies that a child's scope reaches
// every error stored under it.
func TestNewError_TwoLevelCollectorScope(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0,
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "ItemConf",
	)
	_ = child.Collect(E2027("item.score: must be > 0 and <= 100", "800"))
	_ = child.Collect(E2027("item.name: too long", "abcdefghijklmnop"))

	d := NewError(root.Join())
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: ItemConf
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: item.score: must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: ItemConf
DataCellPos: <no value>
DataCell: <no value>
Reason: "abcdefghijklmnop" violates rule: item.name: too long
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_ThreeLevelCollectorTree verifies 3-level collector flattening:
//
//	root collector (Generator)
//	  └── child collector (workbook)
//	        ├── grandchild1 (sheet1)
//	        │     ├── E2027
//	        │     └── E2027
//	        └── grandchild2 (sheet2)
//	              ├── E2005
//	              └── E2003
//
// All 4 leaf errors flatten into a single numbered list.
func TestNewError_ThreeLevelCollectorTree(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	grandchild1 := child.NewChild(0)
	grandchild2 := child.NewChild(0)

	_ = grandchild1.Collect(E2027("score: must be > 0", "0"))
	_ = grandchild1.Collect(E2027("name: too long", "abcdefghijk"))
	_ = grandchild2.Collect(E2005("dup_key"))
	_ = grandchild2.Collect(E2003("5", 1))

	joined := root.Join()
	require.NotNil(t, joined)

	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2005]: map key not unique
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique

[4] error[E2003]: illegal sequence number
Reason: value "5" does not meet sequence requirement: "sequence:1"
Help: prop "sequence:1" requires value starts from "1" and increases monotonically
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_ThreeLevelWithWrapKV verifies 3-level collector with WrapKV on
// individual errors before Collect:
//
//	root collector (Generator)
//	  └── child collector (workbook)
//	        ├── grandchild1
//	        │     ├── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"Sheet1")
//	        │     └── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"Sheet1")
//	        └── grandchild2
//	              └── WrapKV(E2005, Module, Book:"Items#*.csv", Sheet:"Sheet2")
//
// All 3 leaf errors carry BookName + SheetName from their WrapKV layers.
func TestNewError_ThreeLevelWithWrapKV(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	grandchild1 := child.NewChild(0)
	grandchild2 := child.NewChild(0)

	_ = grandchild1.Collect(WrapKV(E2027("score: must be > 0", "0"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet1"))
	_ = grandchild1.Collect(WrapKV(E2027("name: too long", "abcdefghijk"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet1"))
	_ = grandchild2.Collect(WrapKV(E2005("dup_key"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet2"))

	joined := root.Join()
	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2005]: map key not unique
Workbook: Items#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_ThreeLevelCollectorScopes verifies that book and sheet fields
// inherit through the collector hierarchy without wrapping Join results.
func TestNewError_ThreeLevelCollectorScopes(t *testing.T) {
	root := NewCollector(20)
	book := root.NewChild(0, KeyModule, ModuleConf, KeyBookName, "Items#*.csv")
	sheet1 := book.NewChild(0, KeySheetName, "Sheet1")
	sheet2 := book.NewChild(0, KeySheetName, "Sheet2")

	_ = sheet1.Collect(E2027("score: must be > 0", "0"))
	_ = sheet1.Collect(E2027("name: too long", "abcdefghijk"))
	_ = sheet2.Collect(E2005("dup_key"))

	d := NewError(root.Join())
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Items#*.csv
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2005]: map key not unique
Workbook: Items#*.csv
Worksheet: Sheet2
DataCellPos: <no value>
DataCell: <no value>
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_MixedErrorTypesInCollectorTree verifies structured ecodes + plain errors
// in a collector tree render correctly.
func TestNewError_MixedErrorTypesInCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	_ = child.Collect(E2027("score: must be > 0", "0"))
	_ = child.Collect(fmt.Errorf("unexpected EOF at row 42"))
	_ = child.Collect(E2005("dup_key"))

	joined := root.Join()
	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] unexpected EOF at row 42
[3] error[E2005]: map key not unique
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_MultipleWorkbookSiblings verifies multiple workbooks processed
// concurrently, each producing WrapKV'd errors:
//
//	root collector
//	  ├── child1 (Items.xlsx)
//	  │     ├── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  │     └── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  └── child2 (Quests.xlsx)
//	        └── WrapKV(E2003, Book:"Quests.xlsx", Sheet:"QuestConf")
func TestNewError_MultipleWorkbookSiblings(t *testing.T) {
	root := NewCollector(20)

	c1 := root.NewChild(0)
	_ = c1.Collect(WrapKV(E2027("score: must be > 0", "0"),
		KeyModule, ModuleConf, KeyBookName, "Items.xlsx", KeySheetName, "ItemConf"))
	_ = c1.Collect(WrapKV(E2027("name: too long", "abcdefghijk"),
		KeyModule, ModuleConf, KeyBookName, "Items.xlsx", KeySheetName, "ItemConf"))

	c2 := root.NewChild(0)
	_ = c2.Collect(WrapKV(E2003("5", 1),
		KeyModule, ModuleConf, KeyBookName, "Quests.xlsx", KeySheetName, "QuestConf"))

	joined := root.Join()
	d := NewError(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: ItemConf
DataCellPos: <no value>
DataCell: <no value>
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2003]: illegal sequence number
Workbook: Quests.xlsx
Worksheet: QuestConf
DataCellPos: <no value>
DataCell: <no value>
Reason: value "5" does not meet sequence requirement: "sequence:1"
Help: prop "sequence:1" requires value starts from "1" and increases monotonically
`
	assert.Equal(t, want, d.stringify(false))
}

// TestNewError_SingleErrorInDeepTree verifies that a single error in a 3-level
// collector tree collapses to a single Error (no numbered list), with subtests
// for bare error, WrapKV before Collect, and WrapKV on Join.
//
//	root collector
//	  └── child collector
//	        └── grandchild collector
//	              └── E2027 (single error)
func TestNewError_SingleErrorInDeepTree(t *testing.T) {
	t.Run("bare", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		_ = grandchild.Collect(E2027("score: must be > 0", "0"))

		d := NewError(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.stringify(false))
	})

	t.Run("WrapKV_before_Collect", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		_ = grandchild.Collect(WrapKV(E2027("score: must be > 0", "0"),
			KeyModule, ModuleConf,
			KeyBookName, "Items.xlsx",
			KeySheetName, "Sheet1",
		))

		d := NewError(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.stringify(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		_ = grandchild.Collect(E2027("score: must be > 0", "0"))

		wrapped := WrapKV(root.Join(),
			KeyModule, ModuleConf,
			KeyBookName, "Items.xlsx",
			KeySheetName, "Sheet1",
		)

		d := NewError(wrapped)
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.stringify(false))
	})
}

// TestNewError_InnerFieldWins verifies inner WrapKV value wins over outer on
// key conflicts, with subtests for collector tree and layered WrapKV on Join.
func TestNewError_InnerFieldWins(t *testing.T) {
	t.Run("collector_tree", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)

		_ = child.Collect(WrapKV(E2027("score: must be > 0", "0"),
			KeyModule, ModuleConf,
			KeyBookName, "Test.xlsx",
			KeySheetName, "InnerSheet",
		))

		d := NewError(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx
Worksheet: InnerSheet
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.stringify(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		child := NewCollector(10)
		_ = child.Collect(E2027("score: must be > 0", "0"))

		inner := WrapKV(child.Join(), KeySheetName, "InnerSheet")
		outer := WrapKV(inner,
			KeyModule, ModuleConf,
			KeyBookName, "Test.xlsx",
			KeySheetName, "OuterSheet",
		)

		d := NewError(outer)
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx
Worksheet: InnerSheet
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.stringify(false))
	})
}

// TestNewError_EmptyCollectorTree verifies empty collector tree → nil.
func TestNewError_EmptyCollectorTree(t *testing.T) {
	root := NewCollector(10)
	_ = root.NewChild(0)
	_ = root.NewChild(0)

	joined := root.Join()
	assert.NoError(t, joined)
	assert.Nil(t, NewError(joined))
}

// TestNewError_ProtogenModule verifies protogen-module errors render with the
// protogen template, with subtests for WrapKV before Collect and on Join.
func TestNewError_ProtogenModule(t *testing.T) {
	want := `error[E0003]: duplicate column name
Workbook: Items.xlsx
Worksheet: ItemConf
NameCellPos: A1
NameCell: ID
TypeCellPos: A2
TypeCell: int32
Reason: duplicate column name "ID" in both "A1" and "B1"
Help: rename column name and keep sure it is unique in name row
`

	t.Run("WrapKV_before_Collect", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)

		_ = child.Collect(WrapKV(E0003("ID", "A1", "B1"),
			KeyModule, ModuleProto,
			KeyBookName, "Items.xlsx",
			KeySheetName, "ItemConf",
			KeyNameCellPos, "A1",
			KeyNameCell, "ID",
			KeyTypeCellPos, "A2",
			KeyTypeCell, "int32",
		))

		d := NewError(root.Join())
		require.NotNil(t, d)
		assert.Equal(t, want, d.stringify(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		child := NewCollector(10)
		_ = child.Collect(E0003("ID", "A1", "B1"))

		wrapped := WrapKV(child.Join(),
			KeyModule, ModuleProto,
			KeyBookName, "Items.xlsx",
			KeySheetName, "ItemConf",
			KeyNameCellPos, "A1",
			KeyNameCell, "ID",
			KeyTypeCellPos, "A2",
			KeyTypeCell, "int32",
		)

		d := NewError(wrapped)
		require.NotNil(t, d)
		assert.Equal(t, want, d.stringify(false))
	})
}

// TestNewError_GroupEndToEnd verifies the full pipeline using Group:
//
//	root collector
//	  └── child collector
//	        └── Group.Go() goroutines return WrapKV'd errors
//
// Goroutine order is non-deterministic, so we use Contains checks.
func TestNewError_GroupEndToEnd(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	g := child.NewGroup(context.Background())

	g.Go(func(ctx context.Context) error {
		return WrapKV(E2027("item.score: must be > 0 and <= 100", "800"),
			KeyModule, ModuleConf,
			KeyBookName, "Items#*.csv",
			KeySheetName, "ItemConf",
		)
	})
	g.Go(func(ctx context.Context) error {
		return WrapKV(E2027("item.name: too long", "abcdefghijklmnop"),
			KeyModule, ModuleConf,
			KeyBookName, "Items#*.csv",
			KeySheetName, "ItemConf",
		)
	})

	waitErr := g.Wait()
	require.Error(t, waitErr)

	assertGroupOutput := func(t *testing.T, d *Error) {
		t.Helper()
		require.NotNil(t, d)
		rendered := d.stringify(false)
		assert.Contains(t, rendered, "[1]")
		assert.Contains(t, rendered, "[2]")
		assert.Contains(t, rendered, `"800" violates rule: item.score: must be > 0 and <= 100`)
		assert.Contains(t, rendered, `"abcdefghijklmnop" violates rule: item.name: too long`)
		assert.Contains(t, rendered, "Workbook: Items#*.csv")
		assert.Contains(t, rendered, "Worksheet: ItemConf")
	}

	t.Run("via_Wait", func(t *testing.T) {
		assertGroupOutput(t, NewError(waitErr))
	})

	t.Run("via_root_Join", func(t *testing.T) {
		assertGroupOutput(t, NewError(root.Join()))
	})
}

func TestNewErrorReferBookAndSheet(t *testing.T) {
	err := WrapKV(E2013("not-a-bool", errors.New("parse")),
		KeyModule, ModuleConf,
		KeyBookName, "Affix.xlsx",
		KeySheetName, "AffixConf",
		KeyReferBookName, "AssistSkill.xlsx",
		KeyReferSheetName, "AssistSkill",
		KeyDataCellPos, "C4",
		KeyDataCell, "not-a-bool",
	)
	d := NewError(err)
	require.NotNil(t, d)
	got := d.stringify(false)
	assert.Contains(t, got, "Workbook: Affix.xlsx")
	assert.Contains(t, got, "Worksheet: AffixConf")
	assert.Contains(t, got, "ReferWorkbook: AssistSkill.xlsx")
	assert.Contains(t, got, "ReferWorksheet: AssistSkill")
	assert.Contains(t, got, "DataCellPos: C4")
}
