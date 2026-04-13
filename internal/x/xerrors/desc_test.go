package xerrors

import (
	"context"
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
			d := NewDesc(tt.err)
			require.NotNil(t, d)
			assert.Equal(t, tt.wantModule, d.GetValue(KeyModule))
		})
	}
}

func TestNewDescAllNilJoin(t *testing.T) {
	joined := errors.Join(nil, nil)
	assert.Nil(t, NewDesc(joined))
}

// TestNewDescSingleChildJoin verifies errors.Join with one non-nil child → single Desc.
func TestNewDescSingleChildJoin(t *testing.T) {
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

// TestNewDescMultipleChildren verifies errors.Join with multiple children → numbered list.
func TestNewDescMultipleChildren(t *testing.T) {
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

// TestNewDescMixedErrors verifies one structured error + one plain error in a join.
func TestNewDescMixedErrors(t *testing.T) {
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

// TestNewDescWrapKVOverJoin verifies outer WrapKV fields merge into every child Desc,
// while each child's own fields (Reason) still win.
//
//	WrapKV(errors.Join(e1, e2), Module, BookName, SheetName)
//	  └── joinError
//	        ├── e1 (E2027)
//	        └── e2 (E2027)
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

// TestNewDescWrapKVOverJoinSingleChild verifies WrapKV wrapping errors.Join
// with exactly one non-nil child → single Desc (not numbered list).
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

// TestNewDescOuterFieldDoesNotOverrideInner verifies inner WrapKV value wins
// over outer WrapKV on key conflicts.
func TestNewDescOuterFieldDoesNotOverrideInner(t *testing.T) {
	inner := WrapKV(Newf("inner error"), KeyModule, ModuleConf)
	joined := errors.Join(inner)
	wrapped := WrapKV(joined, KeyModule, ModuleProto)

	d := NewDesc(wrapped)
	require.NotNil(t, d)

	want := `
Workbook: <no value> 
Worksheet: <no value> 
DataCellPos: <no value>
DataCell: <no value>
Reason: inner error

`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDescTwoLayerJoinMultiOuter verifies flattening when outer join has
// multiple children, each wrapping an inner join:
//
//	outerJoinError
//	  ├── WrapKV(innerJoin1, Sheet1)
//	  │     └── innerJoin1: {e1, e2}
//	  └── WrapKV(innerJoin2, Sheet2)
//	        └── innerJoin2: {e3, e4}
//
// All 4 leaf errors must appear as a flat numbered list.
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
	assert.Equal(t, want, md.ErrString(false))
}

// TestNewDescThreeLayerJoin verifies arbitrary-depth flattening with layered WrapKV:
//
//	WrapKV(outerJoin, Module, BookName)
//	  └── outerJoinError
//	        ├── WrapKV(innerJoin1, Sheet1)
//	        │     └── innerJoin1: {e1, e2}
//	        └── WrapKV(innerJoin2, Sheet2)
//	              └── innerJoin2: {e3, e4}
//
// All 4 leaf errors carry BookName + SheetName.
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
	assert.Equal(t, want, md.ErrString(false))
}

// TestNewDescTwoLayerJoin verifies the real-world Generator+parser collector pattern:
//
//	outerJoinError                              ← Generator collector.Join()
//	  └── WrapKV(innerJoinError, Module, Book, Sheet)
//	        └── innerJoinError                  ← parser collector.Join()
//	              ├── e1 (E2027)
//	              └── e2 (E2027)
//
// Both leaf errors appear with the outer WrapKV fields.
func TestNewDescTwoLayerJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")

	innerJoin := &joinError{errs: []error{e1, e2}}
	wrapped := WrapKV(innerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)
	outerJoin := &joinError{errs: []error{wrapped}}

	md := NewDesc(outerJoin)
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
	assert.Equal(t, wantNoDebug, md.ErrString(false))
}

// TestNewDesc_CollectedMarkerTransparent verifies that the collected wrapper
// from Collector.Join() is transparent to NewDesc.
func TestNewDesc_CollectedMarkerTransparent(t *testing.T) {
	c := NewCollector(10)
	c.Collect(E2027("score: value must be > 0", "0"))
	c.Collect(E2027("name: too long", "abcdefghijk"))

	joined := c.Join()
	require.NotNil(t, joined)

	var ce *collected
	require.True(t, errors.As(joined, &ce), "Join() must return collected-wrapped error")

	d := NewDesc(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_CollectedSingleError verifies single error in collector → single Desc (no numbered list).
func TestNewDesc_CollectedSingleError(t *testing.T) {
	c := NewCollector(10)
	c.Collect(E2027("score: value must be > 0", "0"))

	joined := c.Join()
	d := NewDesc(joined)
	require.NotNil(t, d)

	want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_TwoLevelCollectorTree verifies 2-level collector flattening:
//
//	root collector
//	  └── child collector
//	        ├── E2027
//	        └── E2005
func TestNewDesc_TwoLevelCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	child.Collect(E2027("score: must be > 0", "0"))
	child.Collect(E2005("duplicate_key"))

	joined := root.Join()
	require.NotNil(t, joined)

	d := NewDesc(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2005]: map or keyed-list key not unique
Reason: map or keyed-list key "duplicate_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_TwoLevelWithWrapKV verifies WrapKV on individual errors before Collect:
//
//	root collector
//	  └── child collector
//	        ├── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"ItemConf")
//	        └── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"Item2Conf")
func TestNewDesc_TwoLevelWithWrapKV(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	child.Collect(WrapKV(E2027("item.score: must be > 0 and <= 100", "800"),
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "ItemConf",
	))
	child.Collect(WrapKV(E2027("item.name: too long", "abcdefghijklmnop"),
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "Item2Conf",
	))

	joined := root.Join()
	d := NewDesc(joined)
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
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_TwoLevelWrapKVOnJoin verifies WrapKV applied on Join() result
// (not re-Collect'd) for direct display:
//
//	WrapKV(child.Join(), Module, Book, Sheet)
//	  └── collected(joinError)
//	        ├── E2027
//	        └── E2027
func TestNewDesc_TwoLevelWrapKVOnJoin(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	child.Collect(E2027("item.score: must be > 0 and <= 100", "800"))
	child.Collect(E2027("item.name: too long", "abcdefghijklmnop"))

	wrapped := WrapKV(child.Join(),
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "ItemConf",
	)

	d := NewDesc(wrapped)
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
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_ThreeLevelCollectorTree verifies 3-level collector flattening:
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
func TestNewDesc_ThreeLevelCollectorTree(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	grandchild1 := child.NewChild(0)
	grandchild2 := child.NewChild(0)

	grandchild1.Collect(E2027("score: must be > 0", "0"))
	grandchild1.Collect(E2027("name: too long", "abcdefghijk"))
	grandchild2.Collect(E2005("dup_key"))
	grandchild2.Collect(E2003("5", 1))

	joined := root.Join()
	require.NotNil(t, joined)

	d := NewDesc(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule

[3] error[E2005]: map or keyed-list key not unique
Reason: map or keyed-list key "dup_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique

[4] error[E2003]: illegal sequence number
Reason: value "5" does not meet sequence requirement: "sequence:1"
Help: prop "sequence:1" requires value starts from "1" and increases monotonically
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_ThreeLevelWithWrapKV verifies 3-level collector with WrapKV on
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
func TestNewDesc_ThreeLevelWithWrapKV(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	grandchild1 := child.NewChild(0)
	grandchild2 := child.NewChild(0)

	grandchild1.Collect(WrapKV(E2027("score: must be > 0", "0"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet1"))
	grandchild1.Collect(WrapKV(E2027("name: too long", "abcdefghijk"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet1"))
	grandchild2.Collect(WrapKV(E2005("dup_key"),
		KeyModule, ModuleConf, KeyBookName, "Items#*.csv", KeySheetName, "Sheet2"))

	joined := root.Join()
	d := NewDesc(joined)
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

[3] error[E2005]: map or keyed-list key not unique
Workbook: Items#*.csv 
Worksheet: Sheet2 
DataCellPos: <no value>
DataCell: <no value>
Reason: map or keyed-list key "dup_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_ThreeLevelWrapKVOnJoin verifies layered WrapKV on Join() results
// (not re-Collect'd) for direct display:
//
//	WrapKV(outerJoin, Module, Book:"Items#*.csv")
//	  └── outerJoinError
//	        ├── WrapKV(grandchild1.Join(), Sheet:"Sheet1")
//	        │     └── joinError: {E2027, E2027}
//	        └── WrapKV(grandchild2.Join(), Sheet:"Sheet2")
//	              └── joinError: {E2005}
func TestNewDesc_ThreeLevelWrapKVOnJoin(t *testing.T) {
	root := NewCollector(20)
	child := root.NewChild(0)
	grandchild1 := child.NewChild(0)
	grandchild2 := child.NewChild(0)

	grandchild1.Collect(E2027("score: must be > 0", "0"))
	grandchild1.Collect(E2027("name: too long", "abcdefghijk"))
	grandchild2.Collect(E2005("dup_key"))

	grandchild1Wrapped := WrapKV(grandchild1.Join(), KeySheetName, "Sheet1")
	grandchild2Wrapped := WrapKV(grandchild2.Join(), KeySheetName, "Sheet2")

	outerJoin := &joinError{errs: []error{grandchild1Wrapped, grandchild2Wrapped}}
	top := WrapKV(outerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
	)

	d := NewDesc(top)
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

[3] error[E2005]: map or keyed-list key not unique
Workbook: Items#*.csv 
Worksheet: Sheet2 
DataCellPos: <no value>
DataCell: <no value>
Reason: map or keyed-list key "dup_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_MixedErrorTypesInCollectorTree verifies structured ecodes + plain errors
// in a collector tree render correctly.
func TestNewDesc_MixedErrorTypesInCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	child.Collect(E2027("score: must be > 0", "0"))
	child.Collect(fmt.Errorf("unexpected EOF at row 42"))
	child.Collect(E2005("dup_key"))

	joined := root.Join()
	d := NewDesc(joined)
	require.NotNil(t, d)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] unexpected EOF at row 42
[3] error[E2005]: map or keyed-list key not unique
Reason: map or keyed-list key "dup_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique
`
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_MultipleWorkbookSiblings verifies multiple workbooks processed
// concurrently, each producing WrapKV'd errors:
//
//	root collector
//	  ├── child1 (Items.xlsx)
//	  │     ├── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  │     └── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  └── child2 (Quests.xlsx)
//	        └── WrapKV(E2003, Book:"Quests.xlsx", Sheet:"QuestConf")
func TestNewDesc_MultipleWorkbookSiblings(t *testing.T) {
	root := NewCollector(20)

	c1 := root.NewChild(0)
	c1.Collect(WrapKV(E2027("score: must be > 0", "0"),
		KeyModule, ModuleConf, KeyBookName, "Items.xlsx", KeySheetName, "ItemConf"))
	c1.Collect(WrapKV(E2027("name: too long", "abcdefghijk"),
		KeyModule, ModuleConf, KeyBookName, "Items.xlsx", KeySheetName, "ItemConf"))

	c2 := root.NewChild(0)
	c2.Collect(WrapKV(E2003("5", 1),
		KeyModule, ModuleConf, KeyBookName, "Quests.xlsx", KeySheetName, "QuestConf"))

	joined := root.Join()
	d := NewDesc(joined)
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
	assert.Equal(t, want, d.ErrString(false))
}

// TestNewDesc_SingleErrorInDeepTree verifies that a single error in a 3-level
// collector tree collapses to a single Desc (no numbered list), with subtests
// for bare error, WrapKV before Collect, and WrapKV on Join.
//
//	root collector
//	  └── child collector
//	        └── grandchild collector
//	              └── E2027 (single error)
func TestNewDesc_SingleErrorInDeepTree(t *testing.T) {
	t.Run("bare", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		grandchild.Collect(E2027("score: must be > 0", "0"))

		d := NewDesc(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.ErrString(false))
	})

	t.Run("WrapKV_before_Collect", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		grandchild.Collect(WrapKV(E2027("score: must be > 0", "0"),
			KeyModule, ModuleConf,
			KeyBookName, "Items.xlsx",
			KeySheetName, "Sheet1",
		))

		d := NewDesc(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx 
Worksheet: Sheet1 
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.ErrString(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		grandchild.Collect(E2027("score: must be > 0", "0"))

		wrapped := WrapKV(root.Join(),
			KeyModule, ModuleConf,
			KeyBookName, "Items.xlsx",
			KeySheetName, "Sheet1",
		)

		d := NewDesc(wrapped)
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx 
Worksheet: Sheet1 
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.ErrString(false))
	})
}

// TestNewDesc_InnerFieldWins verifies inner WrapKV value wins over outer on
// key conflicts, with subtests for collector tree and layered WrapKV on Join.
func TestNewDesc_InnerFieldWins(t *testing.T) {
	t.Run("collector_tree", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)

		child.Collect(WrapKV(E2027("score: must be > 0", "0"),
			KeyModule, ModuleConf,
			KeyBookName, "Test.xlsx",
			KeySheetName, "InnerSheet",
		))

		d := NewDesc(root.Join())
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx 
Worksheet: InnerSheet 
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.ErrString(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		child := NewCollector(10)
		child.Collect(E2027("score: must be > 0", "0"))

		inner := WrapKV(child.Join(), KeySheetName, "InnerSheet")
		outer := WrapKV(inner,
			KeyModule, ModuleConf,
			KeyBookName, "Test.xlsx",
			KeySheetName, "OuterSheet",
		)

		d := NewDesc(outer)
		require.NotNil(t, d)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx 
Worksheet: InnerSheet 
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, d.ErrString(false))
	})
}

// TestNewDesc_EmptyCollectorTree verifies empty collector tree → nil.
func TestNewDesc_EmptyCollectorTree(t *testing.T) {
	root := NewCollector(10)
	_ = root.NewChild(0)
	_ = root.NewChild(0)

	joined := root.Join()
	assert.NoError(t, joined)
	assert.Nil(t, NewDesc(joined))
}

// TestNewDesc_ProtogenModule verifies protogen-module errors render with the
// protogen template, with subtests for WrapKV before Collect and on Join.
func TestNewDesc_ProtogenModule(t *testing.T) {
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

		child.Collect(WrapKV(E0003("ID", "A1", "B1"),
			KeyModule, ModuleProto,
			KeyBookName, "Items.xlsx",
			KeySheetName, "ItemConf",
			KeyNameCellPos, "A1",
			KeyNameCell, "ID",
			KeyTypeCellPos, "A2",
			KeyTypeCell, "int32",
		))

		d := NewDesc(root.Join())
		require.NotNil(t, d)
		assert.Equal(t, want, d.ErrString(false))
	})

	t.Run("WrapKV_on_Join", func(t *testing.T) {
		child := NewCollector(10)
		child.Collect(E0003("ID", "A1", "B1"))

		wrapped := WrapKV(child.Join(),
			KeyModule, ModuleProto,
			KeyBookName, "Items.xlsx",
			KeySheetName, "ItemConf",
			KeyNameCellPos, "A1",
			KeyNameCell, "ID",
			KeyTypeCellPos, "A2",
			KeyTypeCell, "int32",
		)

		d := NewDesc(wrapped)
		require.NotNil(t, d)
		assert.Equal(t, want, d.ErrString(false))
	})
}

// TestNewDesc_GroupEndToEnd verifies the full pipeline using Group:
//
//	root collector
//	  └── child collector
//	        └── Group.Go() goroutines return WrapKV'd errors
//
// Goroutine order is non-deterministic, so we use Contains checks.
func TestNewDesc_GroupEndToEnd(t *testing.T) {
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

	assertGroupOutput := func(t *testing.T, d *Desc) {
		t.Helper()
		require.NotNil(t, d)
		rendered := d.ErrString(false)
		assert.Contains(t, rendered, "[1]")
		assert.Contains(t, rendered, "[2]")
		assert.Contains(t, rendered, `"800" violates rule: item.score: must be > 0 and <= 100`)
		assert.Contains(t, rendered, `"abcdefghijklmnop" violates rule: item.name: too long`)
		assert.Contains(t, rendered, "Workbook: Items#*.csv")
		assert.Contains(t, rendered, "Worksheet: ItemConf")
	}

	t.Run("via_Wait", func(t *testing.T) {
		assertGroupOutput(t, NewDesc(waitErr))
	})

	t.Run("via_root_Join", func(t *testing.T) {
		assertGroupOutput(t, NewDesc(root.Join()))
	})
}
