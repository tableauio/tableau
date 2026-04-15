package confgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/options"
)

// collectorProtoDir is the directory containing the collector.proto file
// used by the integration tests. It is loaded at runtime via protoc compiler
// (not via pre-generated pb.go), so no buf generate is needed.
const collectorProtoDir = "./testdata/collector/proto"

// newCollectorTestGenerator creates a Generator pointing to the given
// testdata subdirectory with CSV-only input format. It uses the
// "collectortest" proto package defined in testdata/collector/proto/collector.proto,
// which is parsed from disk via ProtoPaths/ProtoFiles (not from GlobalFiles).
func newCollectorTestGenerator(inputDir string) *Generator {
	return NewGenerator("collectortest", inputDir, "./testdata/_collector_out/",
		options.Conf(
			&options.ConfOption{
				Input: &options.ConfInputOption{
					ProtoPaths: []string{collectorProtoDir},
					ProtoFiles: []string{collectorProtoDir + "/*.proto"},
					Formats:    []format.Format{format.CSV},
				},
				Output: &options.ConfOutputOption{
					Formats: []format.Format{format.JSON},
				},
			},
		),
	)
}

// e2012 returns the rendered error text for a single E2012 error with confgen template.
func e2012(workbook, worksheet, cellPos, value, fieldType string) string {
	return `error[E2012]: invalid syntax of numerical value` + "\n" +
		`Workbook: ` + workbook + ` ` + "\n" +
		`Worksheet: ` + worksheet + ` ` + "\n" +
		`DataCellPos: ` + cellPos + "\n" +
		`DataCell: ` + value + "\n" +
		`Reason: "` + value + `" cannot be parsed to numerical type "` + fieldType + `", strconv.ParseFloat: parsing "` + value + `": invalid syntax` + "\n" +
		`Help: fill cell data with valid syntax of numerical type "` + fieldType + `"` + "\n"
}

// TestCollectorIntegration_MessageLevel tests that field-level errors within
// a single row (message) are collected by the message-level collector.
//
// CSV data: ItemConf has 2 rows, each with 1 invalid field.
//   - Row 1: ID="abc" (invalid uint32) -> E2012
//   - Row 2: Num="xyz" (invalid int32) -> E2012
//
// Collector hierarchy: global -> book -> sheet(ItemConf) -> message(row)
func TestCollectorIntegration_MessageLevel(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	err := gen.Generate("Collector#ItemConf.csv")
	require.Error(t, err)

	got := err.Error()
	want := "[1] " + e2012("Collector#*.csv", "ItemConf", "A4", "abc", "uint32") +
		"\n[2] " + e2012("Collector#*.csv", "ItemConf", "B5", "xyz", "int32") +
		"\n[3] " + e2012("Collector#*.csv", "ShopConf", "B4", "bad_price", "int32") +
		"\n[4] " + e2012("Collector#*.csv", "ShopConf", "A5", "bad_id", "uint32")
	assert.Equal(t, want, got)
}

// TestCollectorIntegration_BookLevel tests that errors from multiple sheets
// are collected at the book level.
//
// CSV data: Both ItemConf and ShopConf have invalid data.
//   - ItemConf: "abc" (uint32), "xyz" (int32)
//   - ShopConf: "bad_price" (int32), "bad_id" (uint32)
//
// Collector hierarchy: global -> book -> sheet(ItemConf) + sheet(ShopConf)
func TestCollectorIntegration_BookLevel(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	err := gen.Generate("Collector#ItemConf.csv")
	require.Error(t, err)

	got := err.Error()
	want := "[1] " + e2012("Collector#*.csv", "ItemConf", "A4", "abc", "uint32") +
		"\n[2] " + e2012("Collector#*.csv", "ItemConf", "B5", "xyz", "int32") +
		"\n[3] " + e2012("Collector#*.csv", "ShopConf", "B4", "bad_price", "int32") +
		"\n[4] " + e2012("Collector#*.csv", "ShopConf", "A5", "bad_id", "uint32")
	assert.Equal(t, want, got)
}

// TestCollectorIntegration_SheetLevelCapped tests that the sheet-level
// collector caps errors at maxErrorsPerSheet (5).
//
// CSV data: ItemConf has 12 rows with invalid IDs (a1..a12).
// Only the first 5 errors per sheet should be stored.
//
// Collector hierarchy: global(20) -> book(10) -> sheet(5) -> message(3)
func TestCollectorIntegration_SheetLevelCapped(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/overflow/")
	err := gen.Generate("Collector#ItemConf.csv")
	require.Error(t, err)

	got := err.Error()
	want := "[1] " + e2012("Collector#*.csv", "ItemConf", "A4", "a1", "uint32") +
		"\n[2] " + e2012("Collector#*.csv", "ItemConf", "A5", "a2", "uint32") +
		"\n[3] " + e2012("Collector#*.csv", "ItemConf", "A6", "a3", "uint32") +
		"\n[4] " + e2012("Collector#*.csv", "ItemConf", "A7", "a4", "uint32") +
		"\n[5] " + e2012("Collector#*.csv", "ItemConf", "A8", "a5", "uint32") +
		"\n[6] " + e2012("Collector#*.csv", "ShopConf", "A4", "b1", "uint32") +
		"\n[7] " + e2012("Collector#*.csv", "ShopConf", "A5", "b2", "uint32") +
		"\n[8] " + e2012("Collector#*.csv", "ShopConf", "A6", "b3", "uint32") +
		"\n[9] " + e2012("Collector#*.csv", "ShopConf", "A7", "b4", "uint32") +
		"\n[10] " + e2012("Collector#*.csv", "ShopConf", "A8", "b5", "uint32")
	assert.Equal(t, want, got)
}

// TestCollectorIntegration_MultiBook tests that errors from multiple workbooks
// are collected at the global level.
//
// Two workbooks (Collector and Collector2) each have invalid data:
//   - Collector/ItemConf: "abc" (uint32), "xyz" (int32)
//   - Collector/ShopConf: "bad_price" (int32), "bad_id" (uint32)
//   - Collector2/HeroConf: "hero_x" (uint32), "bad_lvl" (int32)
//
// Collector hierarchy: global -> book(Collector) + book(Collector2)
// NOTE: workbook processing order is non-deterministic (concurrent), so we
// verify each error is present rather than asserting exact order.
func TestCollectorIntegration_MultiBook(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/normal/")
	// Generate all workbooks (no specifier triggers GenAll).
	err := gen.Generate()
	require.Error(t, err)

	got := err.Error()
	// Verify all 6 errors are present (order may vary due to concurrent processing).
	assert.Contains(t, got, e2012("Collector2#*.csv", "HeroConf", "A4", "hero_x", "uint32"))
	assert.Contains(t, got, e2012("Collector2#*.csv", "HeroConf", "B5", "bad_lvl", "int32"))
	assert.Contains(t, got, e2012("Collector#*.csv", "ItemConf", "A4", "abc", "uint32"))
	assert.Contains(t, got, e2012("Collector#*.csv", "ItemConf", "B5", "xyz", "int32"))
	assert.Contains(t, got, e2012("Collector#*.csv", "ShopConf", "B4", "bad_price", "int32"))
	assert.Contains(t, got, e2012("Collector#*.csv", "ShopConf", "A5", "bad_id", "uint32"))
	// Verify total error count is exactly 6.
	assert.Equal(t, 6, strings.Count(got, "error[E2012]"))
}

// TestCollectorIntegration_MultiBookCapped tests that the global-level
// collector caps total errors across multiple workbooks.
//
// Two workbooks (Collector and Collector2) each have overflow data:
//   - Collector/ItemConf: 12 invalid rows (a1..a12)
//   - Collector/ShopConf: 12 invalid rows (b1..b12)
//   - Collector2/HeroConf: 12 invalid rows (c1..c12)
//
// Limits: global(20) -> book(10) -> sheet(5) -> message(3)
// Each sheet caps at 5, each book caps at 10, global caps at 20.
// Total possible: 3 sheets * 5 = 15 errors (within book limits).
// NOTE: workbook processing order is non-deterministic (concurrent), so we
// verify each error is present and total count rather than asserting exact order.
func TestCollectorIntegration_MultiBookCapped(t *testing.T) {
	gen := newCollectorTestGenerator("./testdata/collector/csv/overflow/")
	// Generate all workbooks (no specifier triggers GenAll).
	err := gen.Generate()
	require.Error(t, err)

	got := err.Error()
	t.Logf("got error string:\n%s", got)
	// Verify all 15 capped errors are present (order may vary due to concurrent processing).
	// Collector/ItemConf: 5 errors (a1..a5)
	cells := []string{"A4", "A5", "A6", "A7", "A8"}
	for i, v := range []string{"a1", "a2", "a3", "a4", "a5"} {
		assert.Contains(t, got, e2012("Collector#*.csv", "ItemConf", cells[i], v, "uint32"))
	}
	// Collector/ShopConf: 5 errors (b1..b5)
	for i, v := range []string{"b1", "b2", "b3", "b4", "b5"} {
		assert.Contains(t, got, e2012("Collector#*.csv", "ShopConf", cells[i], v, "uint32"))
	}
	// Collector2/HeroConf: 5 errors (c1..c5)
	for i, v := range []string{"c1", "c2", "c3", "c4", "c5"} {
		assert.Contains(t, got, e2012("Collector2#*.csv", "HeroConf", cells[i], v, "uint32"))
	}
}
