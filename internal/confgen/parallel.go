package confgen

import (
	"context"
	"errors"
	"hash/fnv"
	"runtime"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/book/tableparser"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xpool"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// minParallelRows is the threshold below which parallelism is skipped:
// the per-goroutine setup cost is likely to outweigh the savings.
const minParallelRows = 1024

// workerSem caps the total number of concurrently running row-parsing
// workers across all sheets at GOMAXPROCS. Without it, with N sheets
// parsing in parallel the live worker count could reach N*GOMAXPROCS
// and trigger scheduler thrash.
//
// Scoped to leaf workers only -- callers above parseTableInParallel
// (e.g. gen.convert) must not share this semaphore, otherwise an outer
// goroutine holding a token would deadlock waiting for its own inner
// workers.
var workerSem = xpool.NewCPUSemaphore()

// parallelStrategy selects how parseTableInParallel partitions the data rows
// across workers. The strategy is decided once by canParallelizeSheet -- it is
// a property of the sheet's top-level field shape, not of the row values.
type parallelStrategy int

const (
	// strategyRowRange splits rows into contiguous half-open intervals. Used
	// when the top-level field is a vertical NON-keyed list of message:
	// every row produces a fresh element via List.Append, so block boundaries
	// are independent and the merge step concatenates blocks in row order.
	strategyRowRange parallelStrategy = iota
	// strategyKeyHash routes each row to a worker by hashing the raw cell
	// string of the top-level outer-key column. Used when the top-level
	// field is a vertical map OR a vertical KEYED list. Rows sharing the
	// same outer key always land in the same worker, so the worker's serial
	// path handles cross-row aggregation (multi-level vertical map nesting,
	// keyed-list row merging) exactly as it would in the non-parallel run.
	// Cross-block duplicate keys cannot occur by construction; the merge
	// step's duplicate-key check is retained as a defensive invariant.
	strategyKeyHash
)

// parallelPlan is the output of canParallelizeSheet's eligibility check. When
// the sheet is eligible (`ok == true`), `strategy` and (for key-hash mode)
// `keyColumn` describe how parseTableInParallel should shard the rows.
type parallelPlan struct {
	ok        bool
	strategy  parallelStrategy
	keyColumn string // header column name; empty when strategy != strategyKeyHash
	reason    string // human-readable rejection reason; empty when ok
}

// canParallelizeSheet reports whether the given sheet is eligible for parallel
// table parsing. The sheet must satisfy ALL of the following conditions:
//
//  1. It is a table sheet (Excel/CSV), not transposed.
//  2. AdjacentKey is not enabled (cross-row key auto-population would break
//     when the data rows are split into independent blocks).
//  3. The top-level message has exactly one field, which is one of:
//     a. A vertical map whose value is a message. Rows sharing the same
//        outer key are routed to the same worker (strategyKeyHash), so
//        the worker's serial path handles uniqueness checks AND multi-
//        level cross-row aggregation just like the non-parallel run.
//     b. A vertical KEYED list whose element is a message. Same routing
//        as (a) -- the key column drives the partition.
//     c. A vertical NON-keyed list whose element is a message. Each row
//        contributes one fresh list element (strategyRowRange); blocks
//        are concatenated in order at the merge step.
//  4. No field in the descriptor (recursively) declares Unique/Sequence/
//     Order/Aggregate properties: those validations need whole-sheet
//     visibility and would mis-report on per-shard inputs.
//
// When the function returns ok=false, `reason` is a human-readable rejection
// suitable for a debug log explaining why parallel mode was skipped.
func canParallelizeSheet(p *sheetParser, md protoreflect.MessageDescriptor) parallelPlan {
	if !p.IsTable() {
		return parallelPlan{reason: "not a table sheet"}
	}
	if p.sheetOpts.GetTranspose() {
		return parallelPlan{reason: "transpose is enabled"}
	}
	if p.sheetOpts.GetAdjacentKey() {
		return parallelPlan{reason: "adjacent_key is enabled"}
	}

	fields := md.Fields()
	if fields.Len() != 1 {
		return parallelPlan{reason: "top-level message must have exactly one field"}
	}
	topFd := fields.Get(0)
	topField := p.parseFieldDescriptor(topFd)
	defer topField.release()

	switch {
	case topFd.IsMap():
		if parseTableMapLayout(topField.opts.GetLayout()) != tableaupb.Layout_LAYOUT_VERTICAL {
			return parallelPlan{reason: "top-level map layout is not vertical"}
		}
		valueFd := topFd.MapValue()
		if valueFd.Kind() != protoreflect.MessageKind {
			return parallelPlan{reason: "top-level map value is not a message"}
		}
		valueMd := valueFd.Message()
		// We DO allow !deduceKeyUnique here: that's exactly the multi-level
		// vertical-map case where same outer key rows aggregate into one
		// entry. Hash-by-key partitioning lands those rows in the same
		// worker, where the serial path handles aggregation. We still
		// reject explicit RequireUnique sub-fields below in walkParallelSafe.
		keyName := topField.opts.GetKey()
		if keyName == "" {
			return parallelPlan{reason: "top-level vertical map has empty key"}
		}
		// opts.Key is the *column name* (e.g. "HeroID"); resolve it the
		// same way the serial path does so we get a real "key field not
		// found" rejection before we try to hash on a non-existent column.
		if p.findFieldByName(valueMd, keyName) == nil {
			return parallelPlan{reason: "map key field " + keyName + " not found in value message"}
		}
		if reason := checkParallelSafeMessage(p, valueMd); reason != "" {
			return parallelPlan{reason: reason}
		}
		return parallelPlan{ok: true, strategy: strategyKeyHash, keyColumn: keyName}

	case topFd.IsList():
		if parseTableListLayout(topField.opts.GetLayout()) != tableaupb.Layout_LAYOUT_VERTICAL {
			return parallelPlan{reason: "top-level list layout is not vertical"}
		}
		if topFd.Kind() != protoreflect.MessageKind {
			// Scalar vertical lists are not supported by the parser anyway,
			// but be explicit here.
			return parallelPlan{reason: "top-level list element is not a message"}
		}
		elemMd := topFd.Message()
		keyName := topField.opts.GetKey()
		if reason := checkParallelSafeMessage(p, elemMd); reason != "" {
			return parallelPlan{reason: reason}
		}
		if keyName == "" {
			// Non-keyed vertical list: each row produces a fresh element,
			// row-range sharding is correct and trivially preserves order.
			return parallelPlan{ok: true, strategy: strategyRowRange}
		}
		// Keyed vertical list: rows sharing the same key are merged into
		// one element (just like a vertical map). Hash by key column so
		// same-key rows land in the same worker.
		if p.findFieldByName(elemMd, keyName) == nil {
			return parallelPlan{reason: "list key field " + keyName + " not found in element message"}
		}
		return parallelPlan{ok: true, strategy: strategyKeyHash, keyColumn: keyName}

	default:
		return parallelPlan{reason: "top-level field is neither a map nor a list"}
	}
}

// checkParallelSafeMessage walks a message descriptor and returns a non-empty
// reason string if any sub-field disqualifies the sheet from parallel mode.
//
// The walk rejects whole-sheet validation properties (Unique/Sequence/Order/
// Aggregate) anywhere in the descriptor tree -- those need a global view of
// the data that sharding cannot provide. It does NOT reject nested vertical
// maps/lists: such fields aggregate rows that share an ancestor outer key,
// and the strategyKeyHash partitioning routes those rows to a single worker,
// so the serial path's aggregation logic still applies inside each shard.
func checkParallelSafeMessage(p *sheetParser, md protoreflect.MessageDescriptor) string {
	visited := map[protoreflect.FullName]bool{}
	return walkParallelSafe(p, md, visited)
}

func walkParallelSafe(p *sheetParser, md protoreflect.MessageDescriptor, visited map[protoreflect.FullName]bool) string {
	if visited[md.FullName()] {
		return ""
	}
	visited[md.FullName()] = true

	fields := md.Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		field := p.parseFieldDescriptor(fd)
		opts := field.opts
		prop := opts.GetProp()
		// Whole-sheet validations would yield wrong results when blocks are
		// merged: each shard sees only a subset of values.
		if fieldprop.RequireUnique(prop) {
			field.release()
			return "field " + string(fd.FullName()) + " requires Unique"
		}
		if fieldprop.RequireSequence(prop) {
			field.release()
			return "field " + string(fd.FullName()) + " requires Sequence"
		}
		if fieldprop.RequireOrder(prop) {
			field.release()
			return "field " + string(fd.FullName()) + " requires Order"
		}
		if prop.GetAggregate() {
			field.release()
			return "field " + string(fd.FullName()) + " enables Aggregate"
		}
		field.release()
		// Recurse into message-typed sub-fields, message list elements, and
		// message map values. Map-key types are always scalar/enum so don't
		// need recursion.
		switch {
		case fd.IsMap():
			if valueFd := fd.MapValue(); valueFd.Kind() == protoreflect.MessageKind {
				if reason := walkParallelSafe(p, valueFd.Message(), visited); reason != "" {
					return reason
				}
			}
		case fd.IsList():
			if fd.Kind() == protoreflect.MessageKind {
				if reason := walkParallelSafe(p, fd.Message(), visited); reason != "" {
					return reason
				}
			}
		case fd.Kind() == protoreflect.MessageKind:
			if reason := walkParallelSafe(p, fd.Message(), visited); reason != "" {
				return reason
			}
		}
	}
	return ""
}

// parseTableInParallel parses the table by sharding its data rows across
// `runtime.GOMAXPROCS(0)` goroutines, then merges the partial messages with
// xproto.Merge.
//
// Caller must have verified eligibility via canParallelizeSheet first and
// pass the resulting plan in.
func (p *tableParser) parseTableInParallel(protomsg proto.Message, table book.Tabler, plan parallelPlan) error {
	header := tableparser.NewHeader(p.sheetOpts, p.bookOpts, nil)
	beginRow, endRow := tableparser.DataRowRange(table, header)
	totalRows := endRow - beginRow
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 || totalRows < minParallelRows {
		return p.parse(protomsg, table)
	}
	// Cap workers by row count to avoid empty blocks.
	if workers > totalRows {
		workers = totalRows
	}
	log.Debugf("parallel confgen: sheet=%s rows=%d workers=%d strategy=%d",
		p.sheetOpts.GetName(), totalRows, workers, plan.strategy)

	shards, err := planShards(table, header, plan, beginRow, endRow, workers)
	if err != nil {
		return err
	}
	// `planShards` may return fewer entries than `workers` (key-hash mode
	// can leave some buckets empty under skew). Skip empty shards entirely.
	activeShards := make([]shardWork, 0, len(shards))
	for _, s := range shards {
		if len(s.indices) > 0 || s.kind == shardKindRange {
			activeShards = append(activeShards, s)
		}
	}
	if len(activeShards) <= 1 {
		// Either every row hashed into a single bucket, or only one
		// non-empty range exists. Either way, parallelism would buy nothing
		// and the overhead is pure cost. Fall back to the serial path.
		return p.parse(protomsg, table)
	}

	// Each worker produces its own message, so the parsers operate on
	// disjoint state and need no synchronization. We use proto.Clone on the
	// destination message rather than dynamicpb.NewMessage, so partials share
	// the concrete Go type with protomsg and can be merged via xproto.Merge
	// without runtime type errors when the caller passes a typed message.
	partials := make([]proto.Message, len(activeShards))
	parsers := make([]*tableParser, len(activeShards))
	for i := range parsers {
		// Clone the underlying sheetParser per worker; tableParser holds
		// per-sheet caches (cards, sheetCollector child) that are not
		// goroutine-safe.
		sp := newWorkerSheetParser(p.sheetParser)
		parsers[i] = &tableParser{sheetParser: sp}
		partials[i] = proto.Clone(protomsg)
		// proto.Clone copies fields; reset to ensure each worker starts empty.
		proto.Reset(partials[i])
	}

	g := p.sheetCollector.NewGroup(context.Background())
	for i := range activeShards {
		idx := i
		shard := activeShards[idx]
		g.Go(func(ctx context.Context) error {
			// Gate the heavy row-parsing phase on the global worker budget;
			// dispatch is unaffected since each shard still gets its own goroutine.
			if err := workerSem.Acquire(ctx); err != nil {
				return err
			}
			defer workerSem.Release()
			worker := parsers[idx]
			partial := partials[idx]
			msg := partial.ProtoReflect()
			rowFn := func(r *book.Row) error {
				if worker.sheetCollector.IsFull() {
					return worker.sheetCollector.Join()
				}
				_, rowErr := worker.parseMessage(nil, msg, r, "", "")
				if rowErr != nil {
					if cerr := worker.sheetCollector.Collect(rowErr); cerr != nil {
						return cerr
					}
				}
				return nil
			}
			var err error
			switch shard.kind {
			case shardKindRange:
				err = tableparser.RangeDataRowsInBlock(table, header, shard.beginRow, shard.endRow, rowFn)
			case shardKindIndices:
				err = tableparser.RangeDataRowsByIndices(table, header, shard.indices, rowFn)
			}
			if err != nil {
				return err
			}
			if worker.sheetCollector.HasErrors() {
				return worker.sheetCollector.Join()
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Reduce partials by ascending shard index so the merged result matches
	// serial row order:
	//   - list  : elements appended shard-by-shard. For row-range strategy
	//             this preserves sheet order. For key-hash strategy the
	//             order is "all rows of one bucket, then the next" -- for
	//             keyed lists the per-key element identity (and per-element
	//             append order within each shard) is preserved, which is
	//             what callers care about.
	//   - map   : same-key rows always landed in the same shard, so cross-
	//             shard duplicate keys are an invariant violation, not a
	//             user-facing error. xproto.mergeMap's existing duplicate-
	//             key check is retained as a defensive assertion.
	// Note: g.Wait() is a barrier above, so this reduction is independent of
	// the goroutines' completion order.
	for _, partial := range partials {
		if err := xproto.Merge(protomsg, partial); err != nil {
			if errors.Is(err, xproto.ErrDuplicateKey) {
				// This indicates a partitioning bug (same outer key reached
				// two shards), NOT a user data error. Surface it as an
				// internal error rather than the user-facing E2005 so it's
				// not mistaken for "user wrote duplicate key on a unique
				// field" -- the user-facing duplicate is already handled by
				// the worker's serial path on its own shard.
				return xerrors.Newf("internal: duplicate map key across parallel shards (partition bug)")
			}
			return err
		}
	}
	return nil
}

// shardKind tags the row-iteration mode for a single shard.
type shardKind int

const (
	shardKindRange shardKind = iota
	shardKindIndices
)

// shardWork describes the rows a single worker should process.
type shardWork struct {
	kind shardKind
	// Range mode (kind == shardKindRange):
	beginRow, endRow int
	// Indices mode (kind == shardKindIndices): explicit list of row numbers
	// in ascending order.
	indices []int
}

// planShards partitions the data rows according to the parallel plan.
//
// For strategyRowRange, rows are divided into `workers` contiguous half-open
// intervals; the leading `remainder` shards each absorb one extra row so the
// total row count is exactly preserved.
//
// For strategyKeyHash, the planner first reads the outer-key column for every
// row, then hashes each non-empty key with FNV-1a and routes the row to
// `hash % workers`. Empty keys (blank cells) deterministically go to bucket 0
// so that "two blank-key rows under a !deduceKeyUnique map" still merge in
// one worker rather than splitting and tripping the defensive E2005. Rows
// within a bucket retain their ascending sheet order, which is the precondition
// the serial parseVerticalMapField/parseVerticalListField rely on for last-
// write-wins on scalar sub-fields and append-order on inner vertical lists.
func planShards(table book.Tabler, header *tableparser.Header, plan parallelPlan, beginRow, endRow, workers int) ([]shardWork, error) {
	totalRows := endRow - beginRow
	switch plan.strategy {
	case strategyRowRange:
		shards := make([]shardWork, workers)
		blockSize := totalRows / workers
		remainder := totalRows % workers
		cursor := beginRow
		for i := 0; i < workers; i++ {
			size := blockSize
			if i < remainder {
				size++
			}
			shards[i] = shardWork{
				kind:     shardKindRange,
				beginRow: cursor,
				endRow:   cursor + size,
			}
			cursor += size
		}
		return shards, nil

	case strategyKeyHash:
		keys, err := tableparser.ScanColumn(table, header, plan.keyColumn, beginRow, endRow)
		if err != nil {
			return nil, err
		}
		shards := make([]shardWork, workers)
		for i := range shards {
			shards[i] = shardWork{kind: shardKindIndices}
		}
		// Pre-allocate roughly fairly so we don't pay len(rows) reallocs
		// when the bucket distribution is balanced.
		guess := totalRows/workers + 4
		for i := range shards {
			shards[i].indices = make([]int, 0, guess)
		}
		for offset, key := range keys {
			bucket := bucketForKey(key, workers)
			shards[bucket].indices = append(shards[bucket].indices, beginRow+offset)
		}
		return shards, nil
	}
	return nil, xerrors.Newf("internal: unknown parallel strategy %d", plan.strategy)
}

// bucketForKey deterministically maps a raw cell string to a worker index.
// Empty strings collapse into bucket 0 so that all blank-key rows share a
// single shard (otherwise hash(empty) would still pick one bucket but it's
// clearer to make the policy explicit).
//
// KNOWN RISK -- string-level vs type-level key equivalence:
// We hash the raw cell string, not the typed key the parser will eventually
// decode. So two cells that are EQUAL after type parsing but DIFFERENT as
// strings are routed to different buckets. Examples:
//   - int64 key:  "1" vs "01" vs "+1" vs " 1"  -> all decode to int64(1)
//   - bool key:   "true" vs "True" vs "TRUE" vs "1"
//   - float key:  "1" vs "1.0" vs "1e0"
//   - enum key:   "Item_NONE" vs "0" vs "NONE"
//
// When the same logical key is split across workers, vertical map / vertical
// list invariants can break:
//   - last-write-wins / append-order are no longer well-defined (depends on
//     worker completion order during merge);
//   - duplicate-key detection for !deduceKeyUnique maps (E2005) silently
//     misses the conflict because each worker only sees one variant.
// Whether two variants collide also depends on the dynamic worker count, so
// the bug can hide on one machine and surface on another.
//
// We accept this risk for now because real configs rarely rely on such
// variants for the SAME logical key within one sheet. Two follow-ups when
// it actually bites:
//  1. Hash the typed key: have ScanColumn decode each cell with the outer-
//     key field descriptor and hash a canonical byte form, aligning bucket
//     equivalence with merge-time equivalence.
//  2. Defensive merge-time check: after g.Wait, validate uniqueness/order
//     by typed key across shards so correctness degrades gracefully (at
//     worst toward serial cost) even if planning routed wrong.
func bucketForKey(key string, workers int) int {
	if key == "" {
		return 0
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return int(h.Sum32() % uint32(workers))
}

// newWorkerSheetParser shallow-clones a sheetParser for use in a parallel
// worker. The clone shares the same context, options, and extInfo (read-only
// shared state), but gets a fresh cards map and a private sheet-level error
// collector so collectors and caches do not race.
func newWorkerSheetParser(src *sheetParser) *sheetParser {
	sp := &sheetParser{
		ProtoPackage:   src.ProtoPackage,
		LocationName:   src.LocationName,
		ctx:            src.ctx,
		bookOpts:       src.bookOpts,
		sheetOpts:      src.sheetOpts,
		extInfo:        src.extInfo,
		sheetCollector: src.sheetCollector.NewChild(maxErrorsPerSheet),
	}
	sp.reset()
	return sp
}

// parallelEligibilityCache was considered, but eligibility depends on
// per-sheet WorksheetOptions (transpose, adjacent_key), so a descriptor-only
// cache is not safe. The walk itself is O(fields) and runs once per sheet, so
// caching is unnecessary.
