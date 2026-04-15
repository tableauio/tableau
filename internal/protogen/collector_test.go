package protogen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

// TestCollectorHierarchy_MessageLevel tests that message-level (field parsing)
// errors are collected and surfaced through the sheet collector.
func TestCollectorHierarchy_MessageLevel(t *testing.T) {
	// protogen uses: global(10) -> book(5)
	// We simulate a deeper hierarchy: global -> book -> sheet -> message.
	global := xerrors.NewCollector(maxParseErrors)
	book := global.NewChild(maxErrorsPerBook)
	sheet := book.NewChild(3) // sheet-level limit
	msg := sheet.NewChild(2)  // message-level limit

	// Simulate 2 field-level errors in one message parse.
	_ = msg.Collect(fmt.Errorf("field ID: invalid type uint32"))
	err := msg.Collect(fmt.Errorf("field Name: empty string"))

	// message collector is full (2/2).
	require.Error(t, err)
	assert.True(t, msg.IsFull())
	assert.False(t, sheet.IsFull())
	assert.False(t, book.IsFull())
	assert.False(t, global.IsFull())

	joined := msg.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] field ID: invalid type uint32
[2] field Name: empty string`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_SheetLevel tests that multiple message errors
// accumulate at the sheet level in protogen.
func TestCollectorHierarchy_SheetLevel(t *testing.T) {
	global := xerrors.NewCollector(maxParseErrors)
	book := global.NewChild(maxErrorsPerBook)
	sheet := book.NewChild(5)

	// Simulate 3 field-level errors across different messages.
	for i := 1; i <= 3; i++ {
		msg := sheet.NewChild(2)
		_ = msg.Collect(fmt.Errorf("field%d: parse error", i))
	}

	assert.False(t, sheet.IsFull())

	joined := sheet.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] field1: parse error
[2] field2: parse error
[3] field3: parse error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_BookLevel tests that errors from multiple sheets
// accumulate at the book level in protogen.
func TestCollectorHierarchy_BookLevel(t *testing.T) {
	global := xerrors.NewCollector(maxParseErrors)
	book := global.NewChild(maxErrorsPerBook)

	// Simulate 2 sheets, each with 2 errors.
	for s := 1; s <= 2; s++ {
		sheet := book.NewChild(5)
		for r := 1; r <= 2; r++ {
			_ = sheet.Collect(fmt.Errorf("sheet%d_field%d: error", s, r))
		}
	}

	assert.False(t, book.IsFull())

	joined := book.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] sheet1_field1: error
[2] sheet1_field2: error
[3] sheet2_field1: error
[4] sheet2_field2: error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_BookLevelFull tests that the book collector
// becomes full when its limit is reached, and the total stored errors
// across all child sheets are capped by the book limit.
func TestCollectorHierarchy_BookLevelFull(t *testing.T) {
	global := xerrors.NewCollector(maxParseErrors)
	book := global.NewChild(maxErrorsPerBook) // limit = 5

	// Simulate 3 sheets, each with 2 errors = 6 total (exceeds book limit of 5).
	for s := 1; s <= 3; s++ {
		sheet := book.NewChild(10)
		for r := 1; r <= 2; r++ {
			_ = sheet.Collect(fmt.Errorf("sheet%d_field%d: error", s, r))
		}
	}

	assert.True(t, book.IsFull())
	assert.False(t, global.IsFull())

	// Only the first 5 errors are stored (book limit = 5).
	// The 6th error is counted but not stored.
	joined := book.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] sheet1_field1: error
[2] sheet1_field2: error
[3] sheet2_field1: error
[4] sheet2_field2: error
[5] sheet3_field1: error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_GlobalLevel tests that errors from multiple books
// accumulate at the global level in protogen.
func TestCollectorHierarchy_GlobalLevel(t *testing.T) {
	global := xerrors.NewCollector(maxParseErrors)

	// Simulate 3 books, each with 1 sheet, each with 1 error.
	for b := 1; b <= 3; b++ {
		book := global.NewChild(maxErrorsPerBook)
		sheet := book.NewChild(5)
		_ = sheet.Collect(fmt.Errorf("book%d: schema error", b))
	}

	assert.False(t, global.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] book1: schema error
[2] book2: schema error
[3] book3: schema error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_GlobalLevelFull tests that the global collector
// stops storing errors when its limit is reached across books.
func TestCollectorHierarchy_GlobalLevelFull(t *testing.T) {
	global := xerrors.NewCollector(maxParseErrors) // limit = 10

	// Simulate 4 books, each with 1 sheet, each with 3 errors = 12 total.
	for b := 1; b <= 4; b++ {
		book := global.NewChild(maxErrorsPerBook)
		sheet := book.NewChild(10)
		for r := 1; r <= 3; r++ {
			_ = sheet.Collect(fmt.Errorf("book%d_field%d: error", b, r))
		}
	}

	assert.True(t, global.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	// Only the first 10 errors are stored (global limit = 10).
	// The remaining 2 errors are counted but not stored.
	var want string
	for b := 1; b <= 4; b++ {
		for r := 1; r <= 3; r++ {
			idx := (b-1)*3 + r
			if idx > maxParseErrors {
				break
			}
			if idx > 1 {
				want += "\n"
			}
			want += fmt.Sprintf("[%d] book%d_field%d: error", idx, b, r)
		}
	}
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_FourLevelRenderedText tests the complete four-level
// hierarchy (global -> book -> sheet -> message) and verifies the final
// rendered error text with exact string comparison.
func TestCollectorHierarchy_FourLevelRenderedText(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)
	msg := sheet.NewChild(2)

	// Message level: 2 field errors.
	_ = msg.Collect(fmt.Errorf("field_x: bad type"))
	err := msg.Collect(fmt.Errorf("field_y: overflow"))
	require.Error(t, err, "message collector should be full (2/2)")

	// Sheet level: 1 more direct error.
	_ = sheet.Collect(fmt.Errorf("row3: duplicate key"))

	// Verify from global.
	// Join order: own errors first, then children's errors (flattened).
	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] row3: duplicate key
[2] field_x: bad type
[3] field_y: overflow`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_NewKV tests the four-level hierarchy
// with NewKV structured errors that render via the protogen template.
func TestCollectorHierarchy_StructuredErrors_NewKV(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)
	msg := sheet.NewChild(2)

	// Simulate a protogen-style structured error at message level.
	_ = msg.Collect(xerrors.NewKV("unknown type reference",
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "B1",
		xerrors.KeyNameCell, "Attack",
		xerrors.KeyTypeCellPos, "B2",
		xerrors.KeyTypeCell, "UnknownType",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: B1
NameCell: Attack
TypeCellPos: B2
TypeCell: UnknownType
Reason: unknown type reference
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_WrapKV tests WrapKV wrapping
// a plain error with structured fields that render via the protogen template.
func TestCollectorHierarchy_StructuredErrors_WrapKV(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)
	msg := sheet.NewChild(3)

	// Collect 2 NewKV errors at message level.
	_ = msg.Collect(xerrors.NewKV("field1 error",
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "B1",
		xerrors.KeyNameCell, "Attack",
		xerrors.KeyTypeCellPos, "B2",
		xerrors.KeyTypeCell, "int32",
	))
	_ = msg.Collect(xerrors.NewKV("field2 error",
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "C1",
		xerrors.KeyNameCell, "Defense",
		xerrors.KeyTypeCellPos, "C2",
		xerrors.KeyTypeCell, "string",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] 
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: B1
NameCell: Attack
TypeCellPos: B2
TypeCell: int32
Reason: field1 error

[2] 
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: C1
NameCell: Defense
TypeCellPos: C2
TypeCell: string
Reason: field2 error
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_WrapKV_Ecode tests WrapKV wrapping
// a concrete ecode error (E0003) with protogen-style structured fields.
func TestCollectorHierarchy_StructuredErrors_WrapKV_Ecode(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)

	// E0003: duplicate column name
	_ = sheet.Collect(xerrors.WrapKV(xerrors.E0003("ID", "A1", "B1"),
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "A1",
		xerrors.KeyNameCell, "ID",
		xerrors.KeyTypeCellPos, "A2",
		xerrors.KeyTypeCell, "int32",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `error[E0003]: duplicate column name
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: A1
NameCell: ID
TypeCellPos: A2
TypeCell: int32
Reason: duplicate column name "ID" in both "A1" and "B1"
Help: rename column name and keep sure it is unique in name row
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_MixedErrors tests a mix of plain, NewKV, and Wrapf
// errors collected at different levels.
func TestCollectorHierarchy_MixedErrors(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(5)
	msg := sheet.NewChild(3)

	// Message level: 1 NewKV + 1 WrapKV(ecode).
	_ = msg.Collect(xerrors.NewKV("invalid integer",
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "B1",
		xerrors.KeyNameCell, "Attack",
		xerrors.KeyTypeCellPos, "B2",
		xerrors.KeyTypeCell, "int32",
	))
	// E0003: duplicate column name
	_ = msg.Collect(xerrors.WrapKV(xerrors.E0003("Score", "C1", "D1"),
		xerrors.KeyModule, xerrors.ModuleProto,
		xerrors.KeyBookName, "Hero.csv",
		xerrors.KeySheetName, "HeroConf",
		xerrors.KeyNameCellPos, "C1",
		xerrors.KeyNameCell, "Score",
		xerrors.KeyTypeCellPos, "C2",
		xerrors.KeyTypeCell, "int32",
	))

	// Sheet level: 1 plain error.
	_ = sheet.Collect(fmt.Errorf("row5: duplicate key"))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	// Order: sheet own errors first, then children (msg) errors flattened.
	want := `[1] row5: duplicate key
[2] 
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: B1
NameCell: Attack
TypeCellPos: B2
TypeCell: int32
Reason: invalid integer

[3] error[E0003]: duplicate column name
Workbook: Hero.csv 
Worksheet: HeroConf 
NameCellPos: C1
NameCell: Score
TypeCellPos: C2
TypeCell: int32
Reason: duplicate column name "Score" in both "C1" and "D1"
Help: rename column name and keep sure it is unique in name row
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_MidLevelFullStopsSibling tests that when a
// mid-level (book) collector is full, sibling sheets detect it.
func TestCollectorHierarchy_MidLevelFullStopsSibling(t *testing.T) {
	global := xerrors.NewCollector(100)
	book := global.NewChild(3) // tight book limit

	// Sheet1: 2 errors.
	sheet1 := book.NewChild(10)
	_ = sheet1.Collect(fmt.Errorf("sheet1: err1"))
	_ = sheet1.Collect(fmt.Errorf("sheet1: err2"))
	assert.False(t, book.IsFull())

	// Sheet2: 1 error triggers book full.
	sheet2 := book.NewChild(10)
	err := sheet2.Collect(fmt.Errorf("sheet2: err1"))
	require.Error(t, err)
	assert.True(t, book.IsFull())
	assert.True(t, sheet2.IsFull())
	assert.False(t, global.IsFull())

	// Verify rendered text.
	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] sheet1: err1
[2] sheet1: err2
[3] sheet2: err1`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_NumberedListOutput tests that multiple errors
// produce a numbered list in the output.
func TestCollectorHierarchy_NumberedListOutput(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(10)
	sheet := book.NewChild(10)

	_ = sheet.Collect(fmt.Errorf("error one"))
	_ = sheet.Collect(fmt.Errorf("error two"))
	_ = sheet.Collect(fmt.Errorf("error three"))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).Stringify(false)
	want := `[1] error one
[2] error two
[3] error three`
	assert.Equal(t, want, got)
}
