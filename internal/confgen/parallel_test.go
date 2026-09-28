package confgen

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"google.golang.org/protobuf/proto"
)

// newTestSheetParser builds a sheet parser suitable for the parallel-related
// tests below. Parallel confgen is now applied automatically by
// tableParser.Parse whenever the sheet is eligible, so there is no longer a
// per-call toggle: tests that need to compare against the serial reference
// must call tp.parse() directly to bypass the auto-parallel entry point.
func newTestSheetParser(t *testing.T) *sheetParser {
	t.Helper()
	bookOpts := book.MetabookOptions()
	sheetOpts := book.MetasheetOptions(context.Background())
	return NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
		bookOpts, sheetOpts,
		&SheetParserExtInfo{
			InputDir:       "",
			SubdirRewrites: map[string]string{},
			BookFormat:     format.CSV,
		})
}

// parseSerial parses `sheet` strictly through the serial path, bypassing the
// auto-parallel dispatch in tableParser.Parse. Used by parallel-vs-serial
// equivalence tests as the reference output.
func parseSerial(t *testing.T, sp *sheetParser, msg proto.Message, sheet *book.Sheet) {
	t.Helper()
	tp := &tableParser{sheetParser: sp}
	require.NoError(t, tp.parse(msg, sheet.Table))
}

// parseAuto parses `sheet` through the public Parse entry point, which now
// auto-selects parallel mode whenever the sheet is eligible. This is the
// production code path that all other regression tests should exercise.
func parseAuto(t *testing.T, sp *sheetParser, msg proto.Message, sheet *book.Sheet) {
	t.Helper()
	require.NoError(t, sp.Parse(msg, sheet))
}

func makeItemConfRows(n int) [][]string {
	rows := [][]string{
		{"ID", "Num"},
	}
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{fmt.Sprintf("%d", i), fmt.Sprintf("%d", i*10)})
	}
	return rows
}

// TestParallelTableParser_SimpleMapEqualsSerial verifies that a sheet
// eligible for parallel mode produces the exact same proto message as the
// serial path, even when the row count exceeds the parallel threshold and
// triggers actual goroutine sharding.
//
// Because parallel mode is now auto-enabled, the "parallel" output is
// produced via the ordinary Parse entry point, while the "serial" reference
// is produced by calling tp.parse() directly. A regression that broke either
// the auto-dispatch wiring or the partitioning would surface here as an
// inequality rather than a panic, since both paths consume the same input.
func TestParallelTableParser_SimpleMapEqualsSerial(t *testing.T) {
	rows := makeItemConfRows(minParallelRows + 64)
	sheet := book.NewTableSheet("ItemConf", rows)

	serial := &unittestpb.ItemConf{}
	parseSerial(t, newTestSheetParser(t), serial, sheet)

	parallel := &unittestpb.ItemConf{}
	parseAuto(t, newTestSheetParser(t), parallel, sheet)

	require.True(t, proto.Equal(serial, parallel),
		"parallel result diverges from serial: serial=%v parallel=%v", serial, parallel)
	require.Equal(t, len(rows)-1, len(parallel.GetItemMap()))
}

// TestParallelTableParser_DuplicateKeyIsReported verifies that duplicate map
// keys for a single-level vertical map (deduceKeyUnique=true) are surfaced
// as an error rather than silently overwritten.
//
// Under hash-by-key sharding, the duplicate ItemID lands in the same worker
// as its original, so the worker's serial parseVerticalMapField path is the
// one that detects the duplicate and emits E2005. This also pins down the
// invariant "auto-parallel does not relax single-key uniqueness": users
// relying on E2005 to catch typo'd config sheets must continue to see it.
func TestParallelTableParser_DuplicateKeyIsReported(t *testing.T) {
	rows := makeItemConfRows(minParallelRows + 64)
	// Inject a duplicate key. Under hash-by-key, this row hashes into the
	// same shard as the original "1" row, so the worker's serial path
	// triggers E2005. Under the legacy row-range scheme it would have
	// triggered xproto.ErrDuplicateKey at the reduce step instead.
	rows = append(rows, []string{"1", "999"})
	sheet := book.NewTableSheet("ItemConf", rows)

	got := &unittestpb.ItemConf{}
	require.Error(t, newTestSheetParser(t).Parse(got, sheet))
}

// TestParallelTableParser_BelowThresholdFallsBack verifies that small sheets
// fall back to the serial path even when eligible, so the per-goroutine
// setup cost does not regress small-sheet performance.
func TestParallelTableParser_BelowThresholdFallsBack(t *testing.T) {
	rows := makeItemConfRows(8) // far below minParallelRows
	sheet := book.NewTableSheet("ItemConf", rows)

	got := &unittestpb.ItemConf{}
	parseAuto(t, newTestSheetParser(t), got, sheet)
	require.Len(t, got.GetItemMap(), 8)
}

// TestParallelTableParser_ListPreservesRowOrder pins down the contract that
// a vertical list of message, when sharded across workers, ends up with its
// elements in the exact same order as the input rows. Concretely:
//
//   - parseTableInParallel divides rows into contiguous, ascending blocks;
//   - each worker's partial list reflects its block's row order;
//   - xproto.mergeList appends partial lists in slice index order
//     (i.e. block index order) at the final reduction step.
//
// We bypass canParallelizeSheet here because the only readily-available
// vertical-list-of-message sheet in unittestpb has Unique props (which the
// eligibility check rejects). The Unique check itself is a per-row sheet
// validator that still works under sharding as long as input keys are
// globally unique, which they are by construction below.
func TestParallelTableParser_ListPreservesRowOrder(t *testing.T) {
	const n = minParallelRows + 64
	rows := [][]string{
		{"ID", "Name", "Desc"},
	}
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{
			fmt.Sprintf("%d", i),
			fmt.Sprintf("name-%d", i),
			fmt.Sprintf("desc-%d", i),
		})
	}
	sheet := book.NewTableSheet("UniqueFieldInVerticalStructList", rows)

	sp := newTestSheetParser(t)
	tp := &tableParser{sheetParser: sp}
	got := &unittestpb.UniqueFieldInVerticalStructList{}
	require.NoError(t, tp.parseTableInParallel(got, sheet.Table, parallelPlan{
		ok:       true,
		strategy: strategyRowRange,
	}))

	items := got.GetItemList()
	require.Len(t, items, n)
	for i, it := range items {
		require.Equalf(t, uint32(i+1), it.GetId(),
			"list element %d out of order: got id=%d", i, it.GetId())
	}
}

// TestCanParallelizeSheet exhaustively walks the eligibility predicate.
func TestCanParallelizeSheet(t *testing.T) {
	tests := []struct {
		name         string
		setup        func(p *sheetParser)
		md           proto.Message
		wantOK       bool
		wantStrategy parallelStrategy // checked only when wantOK
		wantSub      string           // substring of the rejection reason; empty when wantOK
	}{
		{
			name:         "simple vertical map sheet is eligible (key-hash)",
			md:           &unittestpb.ItemConf{},
			wantOK:       true,
			wantStrategy: strategyKeyHash,
		},
		{
			name: "transpose disqualifies",
			setup: func(p *sheetParser) {
				p.sheetOpts.Transpose = true
			},
			md:      &unittestpb.ItemConf{},
			wantOK:  false,
			wantSub: "transpose",
		},
		{
			name: "adjacent_key disqualifies",
			setup: func(p *sheetParser) {
				p.sheetOpts.AdjacentKey = true
			},
			md:      &unittestpb.ItemConf{},
			wantOK:  false,
			wantSub: "adjacent_key",
		},
		{
			// MallConf has shop_map (vertical) -> goods_map (vertical).
			// This is the canonical multi-level vertical-map case: same
			// outer ShopID across different rows aggregates inner goods.
			// Hash-by-key partitioning routes same-key rows to the same
			// worker, so the worker's serial path runs the same
			// aggregation it would in the non-parallel run. No whole-sheet
			// validation kicks in (no Unique/Sequence/Order/Aggregate),
			// so the sheet is now eligible.
			name:         "nested vertical map sheet is eligible (key-hash)",
			md:           &unittestpb.MallConf{},
			wantOK:       true,
			wantStrategy: strategyKeyHash,
		},
		{
			// Top-level vertical map whose value contains a nested HORIZONTAL
			// map. Horizontal maps live entirely within one row, so they
			// don't even need cross-row aggregation; either partitioning
			// strategy would work, but we still pick key-hash because the
			// top is a vertical map.
			name:         "nested horizontal map is eligible (key-hash)",
			md:           &unittestpb.RewardConf{}, // reward_map (vertical) -> item_map (horizontal)
			wantOK:       true,
			wantStrategy: strategyKeyHash,
		},
		{
			// HorizontalAggregateMap has aggregate:true on its nested
			// horizontal map. Aggregate is a whole-sheet validation that
			// would mis-report on per-shard inputs, so this remains
			// rejected even after the unique check is loosened.
			name:    "aggregate field disqualifies",
			md:      &unittestpb.HorizontalAggregateMap{},
			wantOK:  false,
			wantSub: "Aggregate",
		},
		{
			// Top-level vertical list of message is shardable in principle;
			// the walk still rejects this particular sheet because its inner
			// fields enable Unique. The point of the case is to ensure the
			// list branch is actually taken (i.e. not short-circuited by the
			// "not a map" check) and that the per-field rejection reason
			// flows through.
			name:    "vertical list with unique sub-field disqualifies",
			md:      &unittestpb.UniqueFieldInVerticalStructList{},
			wantOK:  false,
			wantSub: "Unique",
		},
		{
			// SequenceKeyInVerticalKeyedList is a vertical KEYED list whose
			// item id has prop.sequence set. The sequence prop is whole-
			// sheet (it asserts the global "key == seq + index"), so we
			// reject it. Without the sequence prop, a vertical keyed list
			// would now be eligible via key-hash partitioning.
			name:    "vertical keyed list with sequence prop disqualifies",
			md:      &unittestpb.SequenceKeyInVerticalKeyedList{},
			wantOK:  false,
			wantSub: "Sequence",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := newTestSheetParser(t)
			if tt.setup != nil {
				tt.setup(p)
			}
			plan := canParallelizeSheet(p, tt.md.ProtoReflect().Descriptor())
			if tt.wantOK {
				require.True(t, plan.ok, "want eligible but rejected: %s", plan.reason)
				assert.Equal(t, tt.wantStrategy, plan.strategy, "wrong partitioning strategy")
				return
			}
			require.False(t, plan.ok)
			assert.Contains(t, plan.reason, tt.wantSub)
		})
	}
}

// makeMallConfRows builds rows for MallConf (vertical map -> vertical map).
// The returned layout is interleaved: shops do NOT appear in contiguous
// blocks. This pins down the property that hash-by-key partitioning preserves
// "same outer-key rows aggregate into one entry" regardless of input ordering.
//
//	cols = [ShopID, GoodsID, Price]
//	for shop in 1..numShops:
//	  for goods in 1..goodsPerShop:
//	    row(shop, goods, shop*1000+goods)
//	then shuffle by interleaving shops at every step:
//	  (s=1,g=1), (s=2,g=1), ..., (s=N,g=1),
//	  (s=1,g=2), (s=2,g=2), ..., (s=N,g=2), ...
//
// This guarantees that for every shop, its goods rows land in NON-contiguous
// sheet positions, which is precisely the case where row-range sharding would
// break (if we still used it) and where key-hash sharding must succeed.
func makeMallConfRows(numShops, goodsPerShop int) [][]string {
	rows := [][]string{
		{"ShopID", "GoodsID", "Price"},
	}
	for g := 1; g <= goodsPerShop; g++ {
		for s := 1; s <= numShops; s++ {
			rows = append(rows, []string{
				fmt.Sprintf("%d", s),
				fmt.Sprintf("%d", g),
				fmt.Sprintf("%d", s*1000+g),
			})
		}
	}
	return rows
}

// TestParallelTableParser_NestedVerticalMapEqualsSerial is the headline
// regression test for hash-by-key sharding. It feeds a multi-level vertical-
// map sheet (MallConf: shop_map.vertical -> goods_map.vertical) with an
// interleaved row order so that every ShopID's rows are spread across the
// sheet, then asserts that the parallel result is byte-equal to the serial
// result.
//
// What this test would have caught under the old "row-range" scheme:
//   - Splitting the same ShopID's rows across blocks would fire xproto's
//     ErrDuplicateKey at the reduce step, so this would have errored out
//     instead of returning equal output.
//
// What it pins down for hash-by-key:
//   - Same outer-key rows always co-locate in one shard.
//   - Inner goods_map aggregation works inside a shard using the unmodified
//     serial parseVerticalMapField path.
//   - The reduce step's mergeMap finds disjoint outer keys across shards,
//     so its duplicate-key check is a defensive no-op.
func TestParallelTableParser_NestedVerticalMapEqualsSerial(t *testing.T) {
	// Need enough rows to actually trigger sharding; with goodsPerShop large
	// enough that interleaving puts the same shop's rows far apart.
	const numShops = 64
	const goodsPerShop = 32 // 64 * 32 = 2048 data rows > minParallelRows
	rows := makeMallConfRows(numShops, goodsPerShop)
	require.GreaterOrEqual(t, len(rows)-1, minParallelRows,
		"test fixture too small to exercise sharding")
	sheet := book.NewTableSheet("MallConf", rows)

	serial := &unittestpb.MallConf{}
	parseSerial(t, newTestSheetParser(t), serial, sheet)

	parallel := &unittestpb.MallConf{}
	parseAuto(t, newTestSheetParser(t), parallel, sheet)

	require.True(t, proto.Equal(serial, parallel),
		"parallel result diverges from serial:\nserial=%v\nparallel=%v", serial, parallel)
	// Sanity-check the cardinality so a silent "both sides empty" can't
	// pass: every ShopID must appear, every shop must have all its goods.
	require.Len(t, parallel.GetShopMap(), numShops)
	for shopID, shop := range parallel.GetShopMap() {
		require.Lenf(t, shop.GetGoodsMap(), goodsPerShop,
			"shop %d has wrong goods count", shopID)
	}
}

// TestParallelTableParser_NestedVerticalMapSkewed feeds MallConf with one
// dominant ShopID owning ~all rows, plus a handful of singleton shops. This
// pins down two properties:
//
//  1. Correctness under skew: the dominant shop's goods all land in one
//     shard, the singletons hash into other shards (or the same one --
//     either way the result must equal the serial output).
//  2. The "<= 1 active shard" fallback in parseTableInParallel: when every
//     row hashes into a single bucket (e.g. only one outer key exists), the
//     parallel path must transparently fall back to serial rather than
//     spinning up a single-worker goroutine for no benefit. We don't assert
//     on the fallback path directly here, but a regression in it would
//     surface as a result-mismatch via the proto.Equal check.
func TestParallelTableParser_NestedVerticalMapSkewed(t *testing.T) {
	rows := [][]string{
		{"ShopID", "GoodsID", "Price"},
	}
	// One dominant shop with minParallelRows worth of goods.
	const dominantGoods = minParallelRows + 8
	for g := 1; g <= dominantGoods; g++ {
		rows = append(rows, []string{"1", fmt.Sprintf("%d", g), fmt.Sprintf("%d", 1000+g)})
	}
	// A handful of singleton shops, sprinkled at the tail.
	for s := 2; s <= 5; s++ {
		rows = append(rows, []string{fmt.Sprintf("%d", s), "1", fmt.Sprintf("%d", s*1000+1)})
	}
	sheet := book.NewTableSheet("MallConf", rows)

	serial := &unittestpb.MallConf{}
	parseSerial(t, newTestSheetParser(t), serial, sheet)

	parallel := &unittestpb.MallConf{}
	parseAuto(t, newTestSheetParser(t), parallel, sheet)

	require.True(t, proto.Equal(serial, parallel),
		"parallel result diverges from serial under skew:\nserial=%v\nparallel=%v", serial, parallel)
	require.Len(t, parallel.GetShopMap(), 5)
	require.Len(t, parallel.GetShopMap()[1].GetGoodsMap(), dominantGoods)
}

// TestBucketForKey is a tiny but load-bearing unit test: it pins down the
// invariant "two equal raw key strings always map to the same bucket". A
// regression here (e.g. swapping in a non-deterministic hash) would silently
// fail TestParallelTableParser_NestedVerticalMapEqualsSerial under load, so
// it's worth a dedicated tight check.
func TestBucketForKey(t *testing.T) {
	const workers = 8
	for _, key := range []string{"", "1", "42", "ShopABC", "中文键", "1\t"} {
		a := bucketForKey(key, workers)
		b := bucketForKey(key, workers)
		require.Equalf(t, a, b, "bucketForKey(%q) is non-deterministic: %d vs %d", key, a, b)
		require.GreaterOrEqualf(t, a, 0, "bucketForKey(%q) negative: %d", key, a)
		require.Lessf(t, a, workers, "bucketForKey(%q) out of range: %d", key, a)
	}
	// Empty key must always go to bucket 0 by policy.
	require.Equal(t, 0, bucketForKey("", workers))
}
