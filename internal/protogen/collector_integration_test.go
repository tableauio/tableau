package protogen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

// newCollectorTestGenerator creates a Generator pointing to the given
// testdata subdirectory with CSV-only input format.
func newCollectorTestGenerator(inputDir string) *Generator {
	return NewGenerator("collectortest", inputDir, "./testdata/_collector_out/",
		options.Proto(
			&options.ProtoOption{
				Input: &options.ProtoInputOption{
					Formats: []format.Format{format.CSV},
				},
				Output: &options.ProtoOutputOption{},
			},
		),
	)
}

// e0003 returns the rendered error text for a single E0003 error with protogen template.
func e0003(workbook, worksheet, nameCellPos, nameCell, typeCellPos, typeCell, dupPos1, dupPos2 string) string {
	return `error[E0003]: duplicate column name` + "\n" +
		`Workbook: ` + workbook + ` ` + "\n" +
		`Worksheet: ` + worksheet + ` ` + "\n" +
		`NameCellPos: ` + nameCellPos + "\n" +
		`NameCell: ` + nameCell + "\n" +
		`TypeCellPos: ` + typeCellPos + "\n" +
		`TypeCell: ` + typeCell + "\n" +
		`Reason: duplicate column name "` + nameCell + `" in both "` + dupPos1 + `" and "` + dupPos2 + `"` + "\n" +
		`Help: rename column name and keep sure it is unique in name row` + "\n"
}

// TestCollectorIntegration_SingleSheet tests that a single sheet with a
// duplicate column name produces exactly one E0003 error.
//
// CSV data: HeroConf has "Level" duplicated in columns B1 and C1.
//
// Collector hierarchy: global -> book -> sheet(HeroConf)
func TestCollectorIntegration_SingleSheet(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	err := gen.Generate("Collector2#HeroConf.csv")
	require.Error(t, err)

	got := err.Error()
	// Single error: no numbered prefix.
	want := e0003("Collector2#*.csv", "HeroConf", "C1", "Level", "C2", "int32", "B1", "C1")
	assert.Equal(t, want, got)
}

// TestCollectorIntegration_MultiSheet tests that errors from multiple sheets
// in the same workbook are collected at the book level.
//
// CSV data: Both ItemConf and SkillConf have duplicate column names.
//   - ItemConf: "ID" duplicated in A1 and B1 -> E0003
//   - SkillConf: "Name" duplicated in B1 and C1 -> E0003
//
// Collector hierarchy: global -> book -> sheet(ItemConf) + sheet(SkillConf)
func TestCollectorIntegration_MultiSheet(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	err := gen.Generate("Collector#ItemConf.csv")
	require.Error(t, err)

	got := err.Error()
	want := "[1] " + e0003("Collector#*.csv", "ItemConf", "B1", "ID", "B2", "uint32", "A1", "B1") +
		"\n[2] " + e0003("Collector#*.csv", "SkillConf", "C1", "Name", "C2", "string", "B1", "C1")
	assert.Equal(t, want, got)
}

// TestCollectorIntegration_MultiBook tests that errors from multiple workbooks
// are collected at the global level.
//
// Two workbooks (Collector and Collector2) each have invalid schema:
//   - Collector/ItemConf: "ID" duplicated in A1 and B1 -> E0003
//   - Collector/SkillConf: "Name" duplicated in B1 and C1 -> E0003
//   - Collector2/HeroConf: "Level" duplicated in B1 and C1 -> E0003
//
// Collector hierarchy: global -> book(Collector) + book(Collector2)
// NOTE: workbook processing order is non-deterministic (concurrent), so we
// verify each error is present rather than asserting exact order.
func TestCollectorIntegration_MultiBook(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	err := gen.Generate()
	require.Error(t, err)

	got := err.Error()
	t.Logf("got error string:\n%s", got)
	// Verify all 3 errors are present (order may vary due to concurrent processing).
	assert.Contains(t, got, e0003("Collector#*.csv", "ItemConf", "B1", "ID", "B2", "uint32", "A1", "B1"))
	assert.Contains(t, got, e0003("Collector#*.csv", "SkillConf", "C1", "Name", "C2", "string", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector2#*.csv", "HeroConf", "C1", "Level", "C2", "int32", "B1", "C1"))
	// Verify total error count is exactly 3.
	assert.Equal(t, 3, strings.Count(got, "error[E0003]"))
}

// TestCollectorIntegration_MultiBookCapped tests that the global-level
// collector caps total errors across multiple workbooks.
//
// Two workbooks (Collector and Collector2) each have 6 sheets with 1 error each.
//   - Collector/Sheet1..Sheet6: each has 1 duplicate column name -> E0003
//   - Collector2/Sheet1..Sheet6: each has 1 duplicate column name -> E0003
//
// Limits: global(10) -> book(5)
// Each book caps at 5, so both books together produce exactly 10 errors,
// which hits the global cap.
// NOTE: workbook processing order is non-deterministic (concurrent), so we
// verify each error is present and total count rather than asserting exact order.
func TestCollectorIntegration_MultiBookCapped(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/overflow/")
	err := gen.Generate()
	require.Error(t, err)

	got := err.Error()
	t.Logf("got error string:\n%s", got)
	// Collector: first 5 sheets capped (Sheet6 is dropped).
	assert.Contains(t, got, e0003("Collector#*.csv", "Sheet1", "B1", "ID", "B2", "uint32", "A1", "B1"))
	assert.Contains(t, got, e0003("Collector#*.csv", "Sheet2", "C1", "Name", "C2", "string", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector#*.csv", "Sheet3", "C1", "Value", "C2", "int32", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector#*.csv", "Sheet4", "C1", "Type", "C2", "int32", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector#*.csv", "Sheet5", "C1", "Level", "C2", "int32", "B1", "C1"))
	// Collector2: first 5 sheets capped (Sheet6 is dropped).
	assert.Contains(t, got, e0003("Collector2#*.csv", "Sheet1", "B1", "HeroID", "B2", "uint32", "A1", "B1"))
	assert.Contains(t, got, e0003("Collector2#*.csv", "Sheet2", "C1", "Rank", "C2", "int32", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector2#*.csv", "Sheet3", "C1", "Power", "C2", "int32", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector2#*.csv", "Sheet4", "C1", "Speed", "C2", "int32", "B1", "C1"))
	assert.Contains(t, got, e0003("Collector2#*.csv", "Sheet5", "C1", "Armor", "C2", "int32", "B1", "C1"))
	// Verify total error count is exactly 10 (global cap).
	assert.Equal(t, 10, strings.Count(got, "error[E0003]"))
}

// TestCollectorIntegration_BookLevelCapped tests that the book-level
// collector caps errors at maxErrorsPerBook (5).
//
// CSV data: Collector has 6 sheets, each with 1 duplicate column name.
// Only the first 5 errors per book should be stored.
//
// Collector hierarchy: global(10) -> book(5)
func TestCollectorIntegration_BookLevelCapped(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/overflow/")
	err := gen.Generate("Collector#Sheet1.csv")
	require.Error(t, err)

	got := err.Error()
	want := "[1] " + e0003("Collector#*.csv", "Sheet1", "B1", "ID", "B2", "uint32", "A1", "B1") +
		"\n[2] " + e0003("Collector#*.csv", "Sheet2", "C1", "Name", "C2", "string", "B1", "C1") +
		"\n[3] " + e0003("Collector#*.csv", "Sheet3", "C1", "Value", "C2", "int32", "B1", "C1") +
		"\n[4] " + e0003("Collector#*.csv", "Sheet4", "C1", "Type", "C2", "int32", "B1", "C1") +
		"\n[5] " + e0003("Collector#*.csv", "Sheet5", "C1", "Level", "C2", "int32", "B1", "C1")
	assert.Equal(t, want, got)
}
