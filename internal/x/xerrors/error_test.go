package xerrors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWrapKVSourceDefaults verifies generic metadata wrapping fills missing
// sources while retaining precise sources and original causes in joined errors.
func TestWrapKVSourceDefaults(t *testing.T) {
	cause := errors.New("condition missing")
	wrapped := WrapKV(cause, KeyBookName, "Main.xlsx", KeySheetName, "MainSheet")
	require.ErrorIs(t, wrapped, cause)
	serr := Inspect(wrapped)
	require.Len(t, serr.Details, 1)
	require.ErrorIs(t, serr, cause)
	assert.Equal(t, &SourceLocation{Workbook: "Main.xlsx", Worksheet: "MainSheet"}, serr.Details[0].Source)
	assert.Equal(t, "Workbook: Main.xlsx\nWorksheet: MainSheet\nReason: condition missing\n", serr.Error())

	precise := &Error{Details: []*ErrorDetail{{
		Message: "invalid value",
		Source: &SourceLocation{
			Workbook: "Shard.xlsx", Worksheet: "ShardSheet",
			Cell: &CellLocation{Position: "B4", Data: "invalid"},
		},
	}}}
	joined := Inspect(WrapKV(errors.Join(cause, precise), KeyBookName, "Main.xlsx", KeySheetName, "MainSheet"))
	require.Len(t, joined.Details, 2)
	require.ErrorIs(t, joined, cause)
	require.ErrorIs(t, joined, precise)
	assert.Equal(t, serr.Details[0].Source, joined.Details[0].Source)
	assert.Equal(t, precise.Details[0].Source, joined.Details[1].Source)
	assert.Equal(t, "invalid value", joined.Details[1].Message)
	assert.Equal(t, "Shard.xlsx", precise.Details[0].Source.Workbook)
}

func TestErrorGetValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		err  *Error
		key  string
		want any
	}{
		{name: "nil error", key: KeyBookName},
		{name: "no details", err: &Error{}, key: KeyBookName},
		{name: "nil first detail", err: &Error{Details: []*ErrorDetail{nil}}, key: KeyBookName},
		{
			name: "source metadata",
			err:  Inspect(NewKV("failure", KeyBookName, "First.xlsx")),
			key:  KeyBookName, want: "First.xlsx",
		},
		{
			name: "error parameter",
			err:  Inspect(NewKV("failure", "FieldName", "Item")),
			key:  "FieldName", want: "Item",
		},
		{
			name: "first failure in aggregate",
			err: Inspect(errors.Join(
				NewKV("first", KeyBookName, "First.xlsx"),
				NewKV("second", KeyBookName, "Second.xlsx"))),
			key: KeyBookName, want: "First.xlsx",
		},
		{
			name: "absent from first failure",
			err: Inspect(errors.Join(
				NewKV("first", KeySheetName, "FirstSheet"),
				NewKV("second", KeyBookName, "Second.xlsx"))),
			key: KeyBookName,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.err.GetValue(tt.key))
		})
	}
}

func TestInspect(t *testing.T) {
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
			got := Inspect(tt.args.err)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}

func TestInspectPlainError(t *testing.T) {
	err := fmt.Errorf("plain error")
	serr := Inspect(err)
	require.NotNil(t, serr)
	assert.Equal(t, "plain error", serr.stringify(false))
}

func TestInspectNil(t *testing.T) {
	assert.Nil(t, Inspect(nil))
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
			serr := Inspect(tt.err)
			require.NotNil(t, serr)
			assert.Equal(t, tt.wantModule, detailFields(serr)[KeyModule])
		})
	}
}

func TestInspectAllNilJoin(t *testing.T) {
	joined := errors.Join(nil, nil)
	assert.Nil(t, Inspect(joined))
}

// TestInspectSingleChildJoin verifies errors.Join with one non-nil child → single Error.
func TestInspectSingleChildJoin(t *testing.T) {
	e := E2003("1", 3)
	joined := errors.Join(nil, e)
	serr := Inspect(joined)
	require.NotNil(t, serr)

	wantNoDebug := `error[E2003]: illegal sequence number
Reason: value "1" does not meet sequence requirement: "sequence:3"
Help: prop "sequence:3" requires value starts from "3" and increases monotonically
`
	assert.Equal(t, wantNoDebug, serr.stringify(false))

	debugGot := serr.stringify(true)
	assert.True(t, strings.HasPrefix(debugGot, wantNoDebug), "debug output should start with the non-debug summary")
	assert.Contains(t, debugGot, "\n--- debugging ---\n", "debug output should contain debugging header")
	assert.Regexp(t, regexp.MustCompile(`xerrors\.TestInspectSingleChildJoin`), debugGot, "debug output should contain stack trace")
}

// TestInspectMultipleChildren verifies errors.Join with multiple children → numbered list.
func TestInspectMultipleChildren(t *testing.T) {
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := E2027("id: must be positive", "0")
	joined := errors.Join(e1, e2)

	serr := Inspect(joined)
	require.NotNil(t, serr)
	require.Len(t, serr.Details, 2)

	for i, detail := range serr.Details {
		assert.Equal(t, "E2027", detail.Code, "details[%d].Code", i)
	}

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "0" violates rule: id: must be positive
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, serr.stringify(false))
}

// TestInspectMixedErrors verifies one structured error + one plain error in a join.
func TestInspectMixedErrors(t *testing.T) {
	e1 := E2027("name: value length must be at most 10 characters", "toolong")
	e2 := fmt.Errorf("plain error")
	joined := errors.Join(e1, e2)

	serr := Inspect(joined)
	require.NotNil(t, serr)
	require.Len(t, serr.Details, 2)

	assert.Equal(t, "E2027", serr.Details[0].Code)
	assert.Empty(t, serr.Details[1].Code)
	assert.Equal(t, "plain error", serr.Details[1].String())

	wantNoDebug := `[1] error[E2027]: protovalidate violation
Reason: "toolong" violates rule: name: value length must be at most 10 characters
Help: fix the field value to satisfy the protovalidate rule

[2] plain error`
	assert.Equal(t, wantNoDebug, serr.stringify(false))
}

// TestInspectWrapKVOverJoin verifies shared fields merge into every ErrorDetail,
// while each child's own fields (Reason) still win.
//
//	WrapKV(errors.Join(e1, e2), Module, BookName, SheetName)
//	  └── joinError
//	        ├── e1 (E2027)
//	        └── e2 (E2027)
func TestInspectWrapKVOverJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")
	joined := errors.Join(e1, e2)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	serr := Inspect(wrapped)
	require.NotNil(t, serr)

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
	assert.Equal(t, wantNoDebug, serr.stringify(false))
}

// TestInspectWrapKVOverJoinSingleChild verifies WrapKV wrapping errors.Join
// with exactly one non-nil child → single Error (not numbered list).
func TestInspectWrapKVOverJoinSingleChild(t *testing.T) {
	e1 := E2027("score: value must be > 0 and <= 100", "800")
	joined := errors.Join(e1)
	wrapped := WrapKV(joined,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)

	serr := Inspect(wrapped)
	require.NotNil(t, serr)

	wantNoDebug := `error[E2027]: protovalidate violation
Workbook: Validate#*.csv
Worksheet: ValidateFieldLevel
DataCellPos: <no value>
DataCell: <no value>
Reason: "800" violates rule: score: value must be > 0 and <= 100
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, wantNoDebug, serr.stringify(false))
}

// TestInspectOuterFieldDoesNotOverrideInner verifies inner WrapKV value wins
// over outer WrapKV on key conflicts.
func TestInspectOuterFieldDoesNotOverrideInner(t *testing.T) {
	inner := WrapKV(Newf("inner error"), KeyModule, ModuleConf)
	joined := errors.Join(inner)
	wrapped := WrapKV(joined, KeyModule, ModuleProto)

	serr := Inspect(wrapped)
	require.NotNil(t, serr)

	want := `error[E0004]: unknown error
Workbook: <no value>
Worksheet: <no value>
DataCellPos: <no value>
DataCell: <no value>
Reason: inner error
`
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspectTwoLayerJoinMultiOuter verifies flattening when outer join has
// multiple children, each wrapping an inner join:
//
//	outerJoinError
//	  ├── WrapKV(innerJoin1, Sheet1)
//	  │     └── innerJoin1: {e1, e2}
//	  └── WrapKV(innerJoin2, Sheet2)
//	        └── innerJoin2: {e3, e4}
//
// All 4 leaf errors must appear as a flat numbered list.
func TestInspectTwoLayerJoinMultiOuter(t *testing.T) {
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

	serr := Inspect(outerJoin)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspectThreeLayerJoin verifies arbitrary-depth flattening with layered scopes:
//
//	WrapKV(outerJoin, Module, BookName)
//	  └── outerJoinError
//	        ├── WrapKV(innerJoin1, Sheet1)
//	        │     └── innerJoin1: {e1, e2}
//	        └── WrapKV(innerJoin2, Sheet2)
//	              └── innerJoin2: {e3, e4}
//
// All 4 leaf errors carry BookName + SheetName.
func TestInspectThreeLayerJoin(t *testing.T) {
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

	serr := Inspect(top)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
	assert.Equal(t, want, top.Error())
	assert.Equal(t, want, fmt.Sprintf("%s", top))
	assert.Equal(t, want, fmt.Sprintf("%v", top))

	// Verify debug output: each detail should have its own stack trace.
	debugGot := serr.stringify(true)
	t.Log(debugGot)
	for i := 1; i <= 4; i++ {
		assert.Contains(t, debugGot, fmt.Sprintf("[%d] error[E2027]", i), "detail %d should have error header", i)
	}
	assert.Equal(t, 4, strings.Count(debugGot, "\n--- debugging ---\n"), "debug output should contain exactly 4 debugging headers")
	assert.Equal(t, 4, strings.Count(debugGot, "xerrors.TestInspectThreeLayerJoin"), "debug output should contain exactly 4 stack traces")
}

// TestInspectTwoLayerJoin verifies a regular join containing a wrapped inner join:
//
//	outerJoinError                              ← Generator collector.Join()
//	  └── WrapKV(innerJoinError, Module, Book, Sheet)
//	        └── innerJoinError                  ← parser collector.Join()
//	              ├── e1 (E2027)
//	              └── e2 (E2027)
//
// Both leaf errors appear with the outer WrapKV fields.
func TestInspectTwoLayerJoin(t *testing.T) {
	e1 := E2027("item_map[1].score: value must be > 0 and <= 100", "800")
	e2 := E2027("item_map[2].score: value must be > 0 and <= 100", "950")

	innerJoin := &joinError{errs: []error{e1, e2}}
	wrapped := WrapKV(innerJoin,
		KeyModule, ModuleConf,
		KeyBookName, "Validate#*.csv",
		KeySheetName, "ValidateFieldLevel",
	)
	outerJoin := &joinError{errs: []error{wrapped}}

	serr := Inspect(outerJoin)
	require.NotNil(t, serr)

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
	assert.Equal(t, wantNoDebug, serr.stringify(false))
}

// TestInspect_CollectedMarkerTransparent verifies that the collected wrapper
// from Collector.Join() is transparent to Inspect.
func TestInspect_CollectedMarkerTransparent(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(E2027("score: value must be > 0", "0"))
	_ = c.Collect(E2027("name: too long", "abcdefghijk"))

	joined := c.Join()
	require.NotNil(t, joined)

	var ce *collected
	require.True(t, errors.As(joined, &ce), "Join() must return collected-wrapped error")

	serr := Inspect(joined)
	require.NotNil(t, serr)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2027]: protovalidate violation
Reason: "abcdefghijk" violates rule: name: too long
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_CollectedSingleError verifies single error in collector → single Error (no numbered list).
func TestInspect_CollectedSingleError(t *testing.T) {
	c := NewCollector(10)
	_ = c.Collect(E2027("score: value must be > 0", "0"))

	joined := c.Join()
	serr := Inspect(joined)
	require.NotNil(t, serr)

	want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: value must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_TwoLevelCollectorTree verifies 2-level collector flattening:
//
//	root collector
//	  └── child collector
//	        ├── E2027
//	        └── E2005
func TestInspect_TwoLevelCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	_ = child.Collect(E2027("score: must be > 0", "0"))
	_ = child.Collect(E2005("duplicate_key"))

	joined := root.Join()
	require.NotNil(t, joined)

	serr := Inspect(joined)
	require.NotNil(t, serr)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] error[E2005]: map key not unique
Reason: map key "duplicate_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_TwoLevelWithWrapKV verifies WrapKV on individual errors before Collect:
//
//	root collector
//	  └── child collector
//	        ├── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"ItemConf")
//	        └── WrapKV(E2027, Module, Book:"Items#*.csv", Sheet:"Item2Conf")
func TestInspect_TwoLevelWithWrapKV(t *testing.T) {
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
	serr := Inspect(joined)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_TwoLevelCollectorScope verifies that a child's scope reaches
// every error stored under it.
func TestInspect_TwoLevelCollectorScope(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0,
		KeyModule, ModuleConf,
		KeyBookName, "Items#*.csv",
		KeySheetName, "ItemConf",
	)
	_ = child.Collect(E2027("item.score: must be > 0 and <= 100", "800"))
	_ = child.Collect(E2027("item.name: too long", "abcdefghijklmnop"))

	serr := Inspect(root.Join())
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_ThreeLevelCollectorTree verifies 3-level collector flattening:
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
func TestInspect_ThreeLevelCollectorTree(t *testing.T) {
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

	serr := Inspect(joined)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_ThreeLevelWithWrapKV verifies 3-level collector with WrapKV on
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
func TestInspect_ThreeLevelWithWrapKV(t *testing.T) {
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
	serr := Inspect(joined)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_ThreeLevelCollectorScopes verifies that book and sheet fields
// inherit through the collector hierarchy without wrapping Join results.
func TestInspect_ThreeLevelCollectorScopes(t *testing.T) {
	root := NewCollector(20)
	book := root.NewChild(0, KeyModule, ModuleConf, KeyBookName, "Items#*.csv")
	sheet1 := book.NewChild(0, KeySheetName, "Sheet1")
	sheet2 := book.NewChild(0, KeySheetName, "Sheet2")

	_ = sheet1.Collect(E2027("score: must be > 0", "0"))
	_ = sheet1.Collect(E2027("name: too long", "abcdefghijk"))
	_ = sheet2.Collect(E2005("dup_key"))

	serr := Inspect(root.Join())
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_MixedErrorTypesInCollectorTree verifies structured ecodes + plain errors
// in a collector tree render correctly.
func TestInspect_MixedErrorTypesInCollectorTree(t *testing.T) {
	root := NewCollector(10)
	child := root.NewChild(0)

	_ = child.Collect(E2027("score: must be > 0", "0"))
	_ = child.Collect(fmt.Errorf("unexpected EOF at row 42"))
	_ = child.Collect(E2005("dup_key"))

	joined := root.Join()
	serr := Inspect(joined)
	require.NotNil(t, serr)

	want := `[1] error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule

[2] unexpected EOF at row 42
[3] error[E2005]: map key not unique
Reason: map key "dup_key" already exists
Help: fix duplicate keys and ensure map key is unique
`
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_MultipleWorkbookSiblings verifies multiple workbooks processed
// concurrently, each producing WrapKV'd errors:
//
//	root collector
//	  ├── child1 (Items.xlsx)
//	  │     ├── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  │     └── WrapKV(E2027, Book:"Items.xlsx", Sheet:"ItemConf")
//	  └── child2 (Quests.xlsx)
//	        └── WrapKV(E2003, Book:"Quests.xlsx", Sheet:"QuestConf")
func TestInspect_MultipleWorkbookSiblings(t *testing.T) {
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
	serr := Inspect(joined)
	require.NotNil(t, serr)

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
	assert.Equal(t, want, serr.stringify(false))
}

// TestInspect_SingleErrorInDeepTree verifies that a single error in a 3-level
// collector tree collapses to a single Error (no numbered list), with subtests
// for bare error, WrapKV before Collect, and WrapKV on Join.
//
//	root collector
//	  └── child collector
//	        └── grandchild collector
//	              └── E2027 (single error)
func TestInspect_SingleErrorInDeepTree(t *testing.T) {
	t.Run("bare", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)
		grandchild := child.NewChild(0)

		_ = grandchild.Collect(E2027("score: must be > 0", "0"))

		serr := Inspect(root.Join())
		require.NotNil(t, serr)

		want := `error[E2027]: protovalidate violation
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, serr.stringify(false))
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

		serr := Inspect(root.Join())
		require.NotNil(t, serr)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, serr.stringify(false))
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

		serr := Inspect(wrapped)
		require.NotNil(t, serr)

		want := `error[E2027]: protovalidate violation
Workbook: Items.xlsx
Worksheet: Sheet1
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, serr.stringify(false))
	})
}

// TestInspect_InnerFieldWins verifies inner WrapKV value wins over outer on
// key conflicts, with subtests for collector tree and layered WrapKV on Join.
func TestInspect_InnerFieldWins(t *testing.T) {
	t.Run("collector_tree", func(t *testing.T) {
		root := NewCollector(10)
		child := root.NewChild(0)

		_ = child.Collect(WrapKV(E2027("score: must be > 0", "0"),
			KeyModule, ModuleConf,
			KeyBookName, "Test.xlsx",
			KeySheetName, "InnerSheet",
		))

		serr := Inspect(root.Join())
		require.NotNil(t, serr)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx
Worksheet: InnerSheet
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, serr.stringify(false))
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

		serr := Inspect(outer)
		require.NotNil(t, serr)

		want := `error[E2027]: protovalidate violation
Workbook: Test.xlsx
Worksheet: InnerSheet
DataCellPos: <no value>
DataCell: <no value>
Reason: "0" violates rule: score: must be > 0
Help: fix the field value to satisfy the protovalidate rule
`
		assert.Equal(t, want, serr.stringify(false))
	})
}

// TestInspect_EmptyCollectorTree verifies empty collector tree → nil.
func TestInspect_EmptyCollectorTree(t *testing.T) {
	root := NewCollector(10)
	_ = root.NewChild(0)
	_ = root.NewChild(0)

	joined := root.Join()
	assert.NoError(t, joined)
	assert.Nil(t, Inspect(joined))
}

// TestInspect_ProtogenModule verifies protogen-module errors render with the
// protogen template, with subtests for WrapKV before Collect and on Join.
func TestInspect_ProtogenModule(t *testing.T) {
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

		serr := Inspect(root.Join())
		require.NotNil(t, serr)
		assert.Equal(t, want, serr.stringify(false))
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

		serr := Inspect(wrapped)
		require.NotNil(t, serr)
		assert.Equal(t, want, serr.stringify(false))
	})
}

// TestInspect_GroupEndToEnd verifies the full pipeline using Group:
//
//	root collector
//	  └── child collector
//	        └── Group.Go() goroutines return WrapKV'd errors
//
// Goroutine order is non-deterministic, so we use Contains checks.
func TestInspect_GroupEndToEnd(t *testing.T) {
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

	assertGroupOutput := func(t *testing.T, serr *Error) {
		t.Helper()
		require.NotNil(t, serr)
		rendered := serr.stringify(false)
		assert.Contains(t, rendered, "[1]")
		assert.Contains(t, rendered, "[2]")
		assert.Contains(t, rendered, `"800" violates rule: item.score: must be > 0 and <= 100`)
		assert.Contains(t, rendered, `"abcdefghijklmnop" violates rule: item.name: too long`)
		assert.Contains(t, rendered, "Workbook: Items#*.csv")
		assert.Contains(t, rendered, "Worksheet: ItemConf")
	}

	t.Run("via_Wait", func(t *testing.T) {
		assertGroupOutput(t, Inspect(waitErr))
	})

	t.Run("via_root_Join", func(t *testing.T) {
		assertGroupOutput(t, Inspect(root.Join()))
	})
}

func TestInspectReferBookAndSheet(t *testing.T) {
	err := WrapKV(E2013("not-a-bool", errors.New("parse")),
		KeyModule, ModuleConf,
		KeyBookName, "Affix.xlsx",
		KeySheetName, "AffixConf",
		KeyReferBookName, "AssistSkill.xlsx",
		KeyReferSheetName, "AssistSkill",
		KeyDataCellPos, "C4",
		KeyDataCell, "not-a-bool",
	)
	serr := Inspect(err)
	require.NotNil(t, serr)
	got := serr.stringify(false)
	assert.Contains(t, got, "Workbook: Affix.xlsx")
	assert.Contains(t, got, "Worksheet: AffixConf")
	assert.Contains(t, got, "ReferWorkbook: AssistSkill.xlsx")
	assert.Contains(t, got, "ReferWorksheet: AssistSkill")
	assert.Contains(t, got, "DataCellPos: C4")
}

func TestNormalize(t *testing.T) {
	assert.Nil(t, Normalize(nil))
	plain := fmt.Errorf("load config: %w", errors.New("unavailable"))
	assert.Same(t, plain, Normalize(plain))

	cause := E2031("protoconf.FightTypeFilter", ";ranked;casual", 2, 3, ";")
	err := WrapKV(cause,
		KeyModule, ModuleConf,
		KeyBookName, "server/BattlePass.xlsx",
		KeyPrimaryBookName, "server/Task.xlsx",
		KeySheetName, "TaskConfig",
		KeyDataCellPos, "L6",
		KeyDataCell, ";ranked;casual",
		KeyPBMessage, "TaskConfig",
		KeyPBFieldName, "fight_type_filter")
	wrapped := Normalize(fmt.Errorf("load failed: %w", err))
	var structured *Error
	require.ErrorAs(t, wrapped, &structured)
	require.ErrorIs(t, wrapped, ErrE2031)
	assert.Same(t, structured, Normalize(structured))
	assert.Equal(t, Inspect(err).Error(), structured.Error())
	assert.NotContains(t, structured.Error(), "--- debugging ---")
	assert.Contains(t, fmt.Sprintf("%+v", structured), "--- debugging ---")
	require.Len(t, structured.Details, 1)
	detail := structured.Details[0]
	assert.Equal(t, "E2031", detail.Code)
	assert.Contains(t, detail.Message, "protoconf.FightTypeFilter")
	require.NotNil(t, detail.Source)
	assert.Equal(t, "server/BattlePass.xlsx", detail.Source.Workbook)
	assert.Equal(t, "server/Task.xlsx", detail.Source.PrimaryWorkbook)
	assert.Equal(t, "TaskConfig", detail.Source.Worksheet)
	assert.Equal(t, &CellLocation{Position: "L6", Data: ";ranked;casual"}, detail.Source.Cell)
	assert.Equal(t, "TaskConfig", detail.Field.Message)
	assert.Equal(t, "fight_type_filter", detail.Field.Name)

	b, marshalErr := json.Marshal(structured)
	require.NoError(t, marshalErr)
	var decoded Error
	require.NoError(t, json.Unmarshal(b, &decoded))
	roundtrip, marshalErr := json.Marshal(&decoded)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, string(b), string(roundtrip))
	assert.Equal(t, structured.Error(), decoded.Error())
	assert.NotContains(t, string(b), "stack")
	assert.NotContains(t, string(b), `"fields"`)
	assert.NotContains(t, string(b), `"children"`)
}

func TestErrorDetailsFromCollector(t *testing.T) {
	root := NewCollector(10)
	for _, book := range []string{"Shard1.xlsx", "Shard2.xlsx"} {
		sheet := root.NewChild(0,
			KeyModule, ModuleConf,
			KeyBookName, book,
			KeySheetName, "Tasks",
			KeyPrimaryBookName, "Main.xlsx")
		_ = sheet.Collect(WrapKV(E2005(book), KeyDataCellPos, "A4"))
	}
	plain := errors.New("connection closed")
	wrapped := Normalize(errors.Join(fmt.Errorf("load: %w", root.Join()), plain))
	var structured *Error
	require.ErrorAs(t, wrapped, &structured)
	require.Len(t, structured.Details, 3)
	for i, detail := range structured.Details[:2] {
		assert.Equal(t, "E2005", detail.Code)
		assert.Equal(t, fmt.Sprintf("Shard%d.xlsx", i+1), detail.Source.Workbook)
		assert.Equal(t, "Main.xlsx", detail.Source.PrimaryWorkbook)
		assert.Equal(t, "A4", detail.Source.Cell.Position)
	}
	assert.Equal(t, "connection closed", structured.Details[2].Message)
	assert.Nil(t, structured.Details[2].Source)
	require.ErrorIs(t, wrapped, plain)
	b, err := json.Marshal(structured)
	require.NoError(t, err)
	var decoded Error
	require.NoError(t, json.Unmarshal(b, &decoded))
	roundtrip, marshalErr := json.Marshal(&decoded)
	require.NoError(t, marshalErr)
	assert.JSONEq(t, string(b), string(roundtrip))
	assert.Equal(t, structured.Error(), decoded.Error())
}

func TestErrorDetailsNormalization(t *testing.T) {
	err := NewKV("invalid header",
		KeyModule, ModuleProto,
		KeyBookName, "Items.xlsx",
		KeyNameCellPos, "A1",
		KeyNameCell, " ID ",
		KeyTrimmedNameCell, "ID",
		KeyTypeCellPos, "A2",
		KeyTypeCell, "invalid-type",
		KeyNoteCellPos, "A3",
		KeyNoteCell, "identifier")
	var structured *Error
	require.ErrorAs(t, Normalize(err), &structured)
	require.Len(t, structured.Details, 1)
	detail := structured.Details[0]
	assert.Equal(t, "E0004", detail.Code)
	assert.Equal(t, &CellLocation{Position: "A1", Data: " ID ", TrimmedData: "ID"}, detail.Source.NameCell)
	assert.Equal(t, "A2", detail.Source.TypeCell.Position)
	assert.Equal(t, "identifier", detail.Source.NoteCell.Data)
	before, err := json.Marshal(structured)
	require.NoError(t, err)
	for range 3 {
		_ = structured.Error()
	}
	after, err := json.Marshal(structured)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "rendering must not change the serialized data")
}

// A consumer editing typed details must see the same values in text and JSON;
// there must be no stale source metadata or cached summary behind the public error.
func TestErrorDetailsDriveRendering(t *testing.T) {
	cause := WrapKV(E2005("duplicate"),
		KeyModule, ModuleConf,
		KeyBookName, "Old.xlsx", KeyDataCellPos, "A4")
	var e *Error
	require.ErrorAs(t, Normalize(cause), &e)
	e.Details[0].Source.Workbook = "New.xlsx"
	e.Details[0].Source.Cell.Position = "B6"
	e.Details[0].Message = "updated reason"
	assert.Contains(t, e.Error(), "Workbook: New.xlsx")
	assert.Contains(t, e.Error(), "DataCellPos: B6")
	assert.Contains(t, e.Error(), "Reason: updated reason")
	assert.NotContains(t, e.Error(), "Old.xlsx")
	assert.Equal(t, e.Details[0].String(), e.Error())

	// Joining an existing typed error must use those same edited details and
	// retain its original cause through standard Go error traversal.
	joined := Normalize(errors.Join(fmt.Errorf("load: %w", e), errors.New("offline")))
	var aggregate *Error
	require.ErrorAs(t, joined, &aggregate)
	require.Len(t, aggregate.Details, 2)
	assert.Contains(t, aggregate.Error(), "Workbook: New.xlsx")
	require.ErrorIs(t, joined, ErrE2005)
	encoded, err := json.Marshal(aggregate)
	require.NoError(t, err)
	var decoded Error
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, aggregate.Error(), decoded.Error())
}

func TestErrorPlainAndEmptyDetails(t *testing.T) {
	plain := Wrapf(New("inner"), "outer")
	assert.Equal(t, "outer: inner", Normalize(plain).Error())
	scoped := WrapKV(errors.New("offline"), KeyModule, ModuleConf, KeyBookName, "Main.xlsx")
	assert.Equal(t, "Workbook: Main.xlsx\nReason: offline\n", Normalize(scoped).Error())
	for _, e := range []*Error{nil, {}, {Details: []*ErrorDetail{nil}}, {Details: []*ErrorDetail{{}}}} {
		assert.Empty(t, e.Error())
		assert.NotPanics(t, func() { _ = Normalize(fmt.Errorf("wrapped: %w", e)) })
	}
}

func TestInspectPreservesInput(t *testing.T) {
	plain := errors.New("offline")
	view := Inspect(plain)
	require.NotNil(t, view)
	require.Len(t, view.Details, 1)
	require.ErrorIs(t, view, plain)
	assert.Same(t, plain, Normalize(plain))
	view.Details[0].Message = "edited view"
	assert.Equal(t, "offline", plain.Error())

	cause := NewKV("invalid cell", KeyModule, ModuleConf, KeyBookName, "Tasks.xlsx",
		KeyDataCellPos, "A4", KeyDataCell, "bad", KeyPBFieldName, "id")
	fields := cause.(fieldsCarrier).Fields()
	assert.NotContains(t, fields, KeyErrCode)
	view = Inspect(cause)
	assert.Equal(t, "E0004", view.Details[0].Code)
	_ = view.Error()
	assert.NotContains(t, fields, KeyErrCode, "inspection must not add metadata to the input chain")
	snapshot := Inspect(view)
	assert.NotSame(t, view, snapshot)
	assert.Equal(t, view.Error(), snapshot.Error())
	assert.Equal(t, fmt.Sprintf("%+v", view), fmt.Sprintf("%+v", snapshot))
	require.ErrorIs(t, snapshot, view)
	require.ErrorIs(t, snapshot, cause)
	snapshot.Details[0].Message = "edited snapshot"
	snapshot.Details[0].Source.Workbook = "Other.xlsx"
	snapshot.Details[0].Source.Cell.Position = "B6"
	snapshot.Details[0].Field.Name = "other"
	assert.Equal(t, "invalid cell", view.Details[0].Message)
	assert.Equal(t, "Tasks.xlsx", view.Details[0].Source.Workbook)
	assert.Equal(t, "A4", view.Details[0].Source.Cell.Position)
	assert.Equal(t, "id", view.Details[0].Field.Name)
}

func TestErrorDetailPlainSourceRendering(t *testing.T) {
	detail := &ErrorDetail{
		Message: "custom check failed: missing award",
		Source:  &SourceLocation{Workbook: "Test#*.csv", Worksheet: "Activity"},
	}
	assert.Equal(t, "Workbook: Test#*.csv\nWorksheet: Activity\nReason: custom check failed: missing award\n", detail.String())
	assert.NotContains(t, detail.String(), "error[]")
	encoded, err := json.Marshal(&Error{Details: []*ErrorDetail{detail}})
	require.NoError(t, err)
	var decoded Error
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, detail.String(), decoded.Details[0].String())
}

func TestInspectJoinedDetailPreservesMessage(t *testing.T) {
	cause := errors.New("offline")
	view := Inspect(cause)
	view.Details[0].Message = "load failed: offline"
	view.Details[0].Source = &SourceLocation{Workbook: "Tasks#*.csv", Worksheet: "Tasks"}
	joined := Inspect(errors.Join(view, errors.New("another failure")))
	require.Len(t, joined.Details, 2)
	assert.Equal(t, "load failed: offline", joined.Details[0].Message)
	assert.Contains(t, joined.Error(), "Workbook: Tasks#*.csv")
	assert.Contains(t, joined.Error(), "load failed: offline")
	require.ErrorIs(t, joined, cause)
	assert.Equal(t, "offline", cause.Error())
}
