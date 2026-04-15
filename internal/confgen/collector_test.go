package confgen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

// TestCollectorHierarchy_MessageLevel tests that message-level errors are
// collected and surfaced through the sheet collector.
func TestCollectorHierarchy_MessageLevel(t *testing.T) {
	// global(20) -> book(10) -> sheet(5) -> message(3)
	global := xerrors.NewCollector(maxErrors)
	book := global.NewChild(maxErrorsPerBook)
	sheet := book.NewChild(maxErrorsPerSheet)
	msg := sheet.NewChild(maxErrorsPerMessage)

	// Simulate 3 field-level errors in one message parse.
	msg.Collect(fmt.Errorf("field1: invalid type"))
	msg.Collect(fmt.Errorf("field2: value out of range"))
	err := msg.Collect(fmt.Errorf("field3: missing required"))

	// message collector is full (3/3), returns joined error.
	require.Error(t, err)
	assert.True(t, msg.IsFull())
	// Ancestors are NOT full yet.
	assert.False(t, sheet.IsFull())
	assert.False(t, book.IsFull())
	assert.False(t, global.IsFull())

	// Verify rendered text via NewDesc.ErrString(false).
	joined := msg.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] field1: invalid type
[2] field2: value out of range
[3] field3: missing required`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_SheetLevel tests that multiple message errors
// accumulate at the sheet level.
func TestCollectorHierarchy_SheetLevel(t *testing.T) {
	global := xerrors.NewCollector(maxErrors)
	book := global.NewChild(maxErrorsPerBook)
	sheet := book.NewChild(maxErrorsPerSheet)

	// Simulate 2 messages, each producing 1 error.
	for i := 1; i <= 2; i++ {
		msg := sheet.NewChild(maxErrorsPerMessage)
		msg.Collect(fmt.Errorf("msg%d: parse error", i))
	}

	assert.False(t, sheet.IsFull())

	// sheet.Join() includes errors from both message children.
	joined := sheet.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] msg1: parse error
[2] msg2: parse error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_SheetLevelFull tests that the sheet collector
// stops accepting errors when its limit is reached.
func TestCollectorHierarchy_SheetLevelFull(t *testing.T) {
	global := xerrors.NewCollector(maxErrors)
	book := global.NewChild(maxErrorsPerBook)
	sheet := book.NewChild(maxErrorsPerSheet) // limit = 5

	// Simulate 6 row-level errors (exceeds sheet limit of 5).
	for i := 1; i <= 6; i++ {
		sheet.Collect(fmt.Errorf("row%d: error", i))
	}

	assert.True(t, sheet.IsFull())
	assert.False(t, book.IsFull())

	// Only the first 5 errors are stored; the 6th is counted but not stored.
	joined := sheet.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] row1: error
[2] row2: error
[3] row3: error
[4] row4: error
[5] row5: error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_BookLevel tests that errors from multiple sheets
// accumulate at the book level.
func TestCollectorHierarchy_BookLevel(t *testing.T) {
	global := xerrors.NewCollector(maxErrors)
	book := global.NewChild(maxErrorsPerBook)

	// Simulate 3 sheets, each with 1 error.
	for i := 1; i <= 3; i++ {
		sheet := book.NewChild(maxErrorsPerSheet)
		sheet.Collect(fmt.Errorf("sheet%d: header error", i))
	}

	assert.False(t, book.IsFull())

	joined := book.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] sheet1: header error
[2] sheet2: header error
[3] sheet3: header error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_BookLevelFull tests that the book collector
// becomes full when its limit is reached, and the total stored errors
// across all child sheets are capped by the book limit.
func TestCollectorHierarchy_BookLevelFull(t *testing.T) {
	global := xerrors.NewCollector(maxErrors)
	book := global.NewChild(maxErrorsPerBook) // limit = 10

	// Simulate 3 sheets, each with 4 errors = 12 total (exceeds book limit of 10).
	for s := 1; s <= 3; s++ {
		sheet := book.NewChild(maxErrorsPerSheet)
		for r := 1; r <= 4; r++ {
			sheet.Collect(fmt.Errorf("sheet%d_row%d: error", s, r))
		}
	}

	assert.True(t, book.IsFull())
	assert.False(t, global.IsFull())

	// Only the first 10 errors are stored (book limit = 10).
	// The 11th and 12th errors are counted but not stored.
	joined := book.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] sheet1_row1: error
[2] sheet1_row2: error
[3] sheet1_row3: error
[4] sheet1_row4: error
[5] sheet2_row1: error
[6] sheet2_row2: error
[7] sheet2_row3: error
[8] sheet2_row4: error
[9] sheet3_row1: error
[10] sheet3_row2: error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_GlobalLevel tests that errors from multiple books
// accumulate at the global level.
func TestCollectorHierarchy_GlobalLevel(t *testing.T) {
	global := xerrors.NewCollector(maxErrors)

	// Simulate 2 books, each with 1 sheet, each with 1 error.
	for b := 1; b <= 2; b++ {
		book := global.NewChild(maxErrorsPerBook)
		sheet := book.NewChild(maxErrorsPerSheet)
		sheet.Collect(fmt.Errorf("book%d_sheet1: error", b))
	}

	assert.False(t, global.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] book1_sheet1: error
[2] book2_sheet1: error`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_GlobalLevelFull tests that the global collector
// stops storing errors when its limit is reached across books.
func TestCollectorHierarchy_GlobalLevelFull(t *testing.T) {
	global := xerrors.NewCollector(maxErrors) // limit = 20

	// Simulate 5 books, each with 2 sheets, each with 3 errors = 30 total.
	for b := 1; b <= 5; b++ {
		book := global.NewChild(maxErrorsPerBook)
		for s := 1; s <= 2; s++ {
			sheet := book.NewChild(maxErrorsPerSheet)
			for r := 1; r <= 3; r++ {
				sheet.Collect(fmt.Errorf("book%d_sheet%d_row%d: error", b, s, r))
			}
		}
	}

	assert.True(t, global.IsFull())

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	// Only the first 20 errors are stored (global limit = 20).
	// The remaining 10 errors are counted but not stored.
	var want string
	for b := 1; b <= 5; b++ {
		for s := 1; s <= 2; s++ {
			for r := 1; r <= 3; r++ {
				idx := (b-1)*6 + (s-1)*3 + r
				if idx > maxErrors {
					break
				}
				if idx > 1 {
					want += "\n"
				}
				want += fmt.Sprintf("[%d] book%d_sheet%d_row%d: error", idx, b, s, r)
			}
		}
	}
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_FourLevelRenderedText tests the complete four-level
// hierarchy (global -> book -> sheet -> message) and verifies the final
// rendered error text with exact string comparison.
func TestCollectorHierarchy_FourLevelRenderedText(t *testing.T) {
	// Use small limits for easy verification.
	global := xerrors.NewCollector(10) // global limit
	book := global.NewChild(5)         // book limit
	sheet := book.NewChild(3)          // sheet limit
	msg := sheet.NewChild(2)           // message limit

	// Collect 2 field errors at message level.
	msg.Collect(fmt.Errorf("field_a: type mismatch"))
	err := msg.Collect(fmt.Errorf("field_b: null value"))
	require.Error(t, err, "message collector should be full (2/2)")
	assert.True(t, msg.IsFull())

	// Collect 1 more error directly at sheet level (e.g., row-level error).
	sheet.Collect(fmt.Errorf("row2: missing key"))

	// Verify the full tree from global.Join().
	// Join order: own errors first, then children's errors (flattened).
	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] row2: missing key
[2] field_a: type mismatch
[3] field_b: null value`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_NewKV tests the four-level hierarchy
// with NewKV structured errors that render via the confgen template.
func TestCollectorHierarchy_StructuredErrors_NewKV(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)
	msg := sheet.NewChild(2)

	// Simulate a confgen-style structured error at message level.
	msg.Collect(xerrors.NewKV("invalid integer value",
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "C3",
		xerrors.KeyDataCell, "abc",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: C3
DataCell: abc
Reason: invalid integer value
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_WrapKV tests WrapKV wrapping
// a collector join, merging outer fields into each child error.
func TestCollectorHierarchy_StructuredErrors_WrapKV(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)
	msg := sheet.NewChild(3)

	// Collect 2 NewKV errors at message level.
	msg.Collect(xerrors.NewKV("field1 error",
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "C3",
		xerrors.KeyDataCell, "abc",
	))
	msg.Collect(xerrors.NewKV("field2 error",
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "D3",
		xerrors.KeyDataCell, "def",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] 
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: C3
DataCell: abc
Reason: field1 error

[2] 
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: D3
DataCell: def
Reason: field2 error
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_StructuredErrors_WrapKV tests WrapKV wrapping
// a concrete ecode error (E2005) with confgen-style structured fields.
func TestCollectorHierarchy_StructuredErrors_WrapKV_Ecode(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(5)
	sheet := book.NewChild(3)

	// E2005: map or keyed-list key not unique
	sheet.Collect(xerrors.WrapKV(xerrors.E2005("dup_key"),
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "B3",
		xerrors.KeyDataCell, "dup_key",
	))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `error[E2005]: map or keyed-list key not unique
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: B3
DataCell: dup_key
Reason: map or keyed-list key "dup_key" already exists
Help: fix duplicate keys and ensure map or keyed-list key is unique
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
	msg.Collect(xerrors.NewKV("invalid integer",
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "C3",
		xerrors.KeyDataCell, "abc",
	))
	// E2000: integer overflow
	msg.Collect(xerrors.WrapKV(xerrors.E2000("int32", "999999999999", int32(-2147483648), int32(2147483647)),
		xerrors.KeyModule, xerrors.ModuleConf,
		xerrors.KeyBookName, "Items.xlsx",
		xerrors.KeySheetName, "ItemConf",
		xerrors.KeyDataCellPos, "D3",
		xerrors.KeyDataCell, "999999999999",
	))

	// Sheet level: 1 plain error.
	sheet.Collect(fmt.Errorf("row5: duplicate key"))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	// Order: sheet own errors first, then children (msg) errors flattened.
	want := `[1] row5: duplicate key
[2] 
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: C3
DataCell: abc
Reason: invalid integer

[3] error[E2000]: integer overflow
Workbook: Items.xlsx 
Worksheet: ItemConf 
DataCellPos: D3
DataCell: 999999999999
Reason: value "999999999999" is outside of range [-2147483648,2147483647] of type int32
Help: check field value and make sure it in representable range
`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_MidLevelFullStopsSibling tests that when a
// mid-level (book) collector is full, sibling sheets detect it via IsFull.
func TestCollectorHierarchy_MidLevelFullStopsSibling(t *testing.T) {
	global := xerrors.NewCollector(100)
	book := global.NewChild(3) // tight book limit

	// Sheet1: 2 errors.
	sheet1 := book.NewChild(10)
	sheet1.Collect(fmt.Errorf("sheet1: err1"))
	sheet1.Collect(fmt.Errorf("sheet1: err2"))
	assert.False(t, book.IsFull())

	// Sheet2: 1 error triggers book full.
	sheet2 := book.NewChild(10)
	err := sheet2.Collect(fmt.Errorf("sheet2: err1"))
	require.Error(t, err, "book collector should be full (3/3)")
	assert.True(t, book.IsFull())

	// Sheet2 also reports full because ancestor (book) is full.
	assert.True(t, sheet2.IsFull())

	// Global is NOT full.
	assert.False(t, global.IsFull())

	// Verify rendered text.
	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] sheet1: err1
[2] sheet1: err2
[3] sheet2: err1`
	assert.Equal(t, want, got)
}

// TestCollectorHierarchy_NumberedListOutput tests that multiple errors
// at the same level produce a numbered list in the output.
func TestCollectorHierarchy_NumberedListOutput(t *testing.T) {
	global := xerrors.NewCollector(10)
	book := global.NewChild(10)
	sheet := book.NewChild(10)

	// Collect 3 simple errors.
	sheet.Collect(fmt.Errorf("error alpha"))
	sheet.Collect(fmt.Errorf("error beta"))
	sheet.Collect(fmt.Errorf("error gamma"))

	joined := global.Join()
	require.Error(t, joined)
	got := xerrors.NewDesc(joined).ErrString(false)
	want := `[1] error alpha
[2] error beta
[3] error gamma`
	assert.Equal(t, want, got)
}
