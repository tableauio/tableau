# Confgen Profiling and Optimization Plan

This document explains how to view the performance report, records the latest
measurements, and defines the next optimization work in measurement order.

## View the report in a browser

The editable report is a Codex Canvas:

```text
C:\Users\wenchyzhu\.cursor\projects\d-GitHub-tableauio-tableau\canvases\confgen-profile-analysis.canvas.tsx
```

The file imports `cursor/canvas`, a virtual component library provided by the
Codex app. A normal browser and ordinary TypeScript tools cannot resolve that
module, so opening the `.canvas.tsx` file directly displays source code or a
blank page.

Use one of these two supported views:

1. In Codex, open the Canvas file directly. Codex compiles it and renders the
   interactive report beside the conversation.
2. In a normal browser, open the self-contained HTML export:
   [`confgen-profile-report.html`](./confgen-profile-report.html).

On Windows, open the export from the repository root with:

```powershell
Start-Process .\docs\profile\confgen-profile-report.html
```

If the browser restricts local `file:` pages, serve the directory over HTTP:

```powershell
python -m http.server 8000 --directory .\docs\profile
```

Then open <http://localhost:8000/confgen-profile-report.html>.

The HTML report intentionally has no package, build, or network dependency. It
is a browser snapshot rather than a compiled form of the Canvas. When profile
data changes, update both the Canvas data and the HTML export from the same
measurement set.

## Measurement setup

- Branch: `codex/confgen-sheet-perf`
- Platform: Windows
- Configuration:
  `D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\config.local.yaml`
- Command working directory: `D:\Tencent\SVNTest\trunk`
- Profile output directory:
  `D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\conf`

Representative profiled command:

```powershell
tableauc --mode conf `
  --config D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\config.local.yaml `
  --profiling `
  --proto-package protoconf `
  --indir . `
  --outdir .
```

Normal mode is the source of truth for user-visible performance. CPU, heap, and
block profiles explain the result but add instrumentation overhead.

## Current measurements

The earlier report used a configuration where per-record Zap synchronization
dominated execution. Removing that synchronization reduced normal wall time
from 14.85 seconds to 6.13 seconds. The current production configuration uses
simple INFO logging, so a fresh baseline was captured before evaluating parser
changes.

### Fresh baseline

- Normal runs: 8.15, 5.96, 5.81, 5.31, and 5.64 seconds.
- Five-run median: 5.81 seconds. The first 8.15-second run was cold; the median
  of the four subsequent runs was 5.73 seconds.
- Profiled wall time: 5.524 seconds.
- CPU samples: 21.92 CPU-seconds over a 5.10-second capture.
- Allocation space: 6.92 GB.

The main baseline CPU paths were:

| Area | CPU | Share |
| --- | ---: | ---: |
| Excel row decoding | 7.85 s | 38.84% |
| Merger processing | 6.56 s | 32.46% |
| Sheet parsing | 3.84 s | 19.00% |
| Store output | 3.08 s | 15.24% |
| Garbage collection | 2.94 s | 14.55% |
| Refer loading | 1.54 s | 7.62% |

Inclusive paths overlap and their percentages must not be added.

The largest allocation sources were XML tokenization at 1.92 GB, Excel row XML
handling at 0.78 GB, XML token creation at 0.73 GB, `fastjson` at 0.44 GB,
`Row.AddCell` at 0.27 GB, `bytes.growSlice` at 0.27 GB, `bytes.Replace` at
0.26 GB, and dynamic protobuf field assignment at 0.26 GB.

### Retained changes

The current candidate contains these measured changes:

- Logger synchronization moved from every log record to the command boundary.
  On the current simple-console configuration this changed the five-run median
  from 5.81 to 5.72 seconds. The larger earlier A/B result justifies fixing the
  logger lifecycle.
- CPU labels now separate source opening, sheet decoding, sheet parsing, refer
  loading, and refer waiting. Profiling also writes a block profile.
- Windows no longer pins each measured sheet goroutine to an OS thread. CPU
  pprof labels provide useful attribution without changing scheduling.
- Rows use a bounded dense cell slice instead of a map keyed by column index.
- Parser fields cache their optional variant and map key descriptor.
- Scalar parsing avoids constructing unused cardinality prefixes, and top-level
  column names avoid redundant concatenation.

After the dense row and parser-plan changes, the five-run median was 5.21 seconds
and allocation space was 6.66 GB. Avoiding scalar prefix allocation reduced the
five-run median to 4.88 seconds and allocation space to 6.48 GB. This is about 16%
faster and 6.4% less allocation than the fresh 5.81-second, 6.92-GB baseline.

The final column-name helper showed a win in four of five interleaved A/B pairs,
with an approximately 0.27-second median paired improvement. Absolute sequential
runs remained noisy, so this result should be reproduced after the current
changes are consolidated into one candidate binary.

### Profiling attribution

In the labeled profile, the main work categories were:

| Work label | CPU | Share |
| --- | ---: | ---: |
| `sheet_decode` | 9.07 s | 42.82% |
| `source_open` | 2.86 s | 13.50% |
| `sheet_parse` | 2.19 s | 10.34% |

The XLSX format accounted for 56.2% of sampled CPU. The largest source labels
were `AiAccountPvpStat` at 3.29 CPU-seconds, `Skill_Monster` at 1.10,
`AiAccountDisplay` at 1.00, and `Skill_Player` at 0.72.

The block profile attributed about 40.86 aggregate goroutine-seconds to refer
waits. This is cumulative blocked time across goroutines, not elapsed runtime.
It explains why a small sheet can report a large wall duration while consuming
little CPU.

## Experiments not retained

These experiments did not meet the normal-mode performance gate:

- Concurrent decoding of sheets from the same Excel workbook improved the
  median by only about 0.8% and increased variance.
- Asynchronous refer preloading improved the median by only about 0.8% and
  increased cumulative refer wait time from 40.86 to 51.20 goroutine-seconds.
- A per-field map caching every prefixed column name regressed the median to
  about 6.3 seconds because lookup and synchronization costs exceeded saved
  string allocation.

The lazy canonical refer cache, serialized per-workbook Excel access, and the
simple column-name helper remain in place.

## Optimization plan

The original 14.85-second run is now about 3.0 times slower than the current
4.88-second candidate. Reaching a 4x improvement from that original run requires
a five-run median at or below 3.71 seconds, about 24% below the current candidate.
Reaching 2x from the fresh 5.81-second baseline would require 2.91 seconds and
likely needs a deeper XLSX reader change plus output-path improvements.

### 1. Consolidate and validate the current candidate

Priority: P0

1. Build one candidate containing the logger lifecycle, profiling labels, dense
   rows, descriptor caches, and prefix changes.
2. Compare it with the recorded baseline using interleaved runs to reduce drift
   from filesystem cache, antivirus scanning, and system load.
3. Compare every generated configuration file byte-for-byte.
4. Run focused tests and the full Go test suite without `-race` on Windows.

Acceptance gates:

- At least five measured warm runs for each binary.
- The candidate improves median wall time and does not materially regress p95.
- Generated output is identical.
- CPU, memory, and block profiles are readable and use the expected labels.

### 2. Reduce XLSX source opening and sheet decoding

Priority: P1

Excel opening and row decoding are the largest remaining measured cost. The
current Excel library opens the ZIP container and prepares workbook data before
the importer decodes requested sheets. Optimize this path in measured steps:

1. Record each workbook's ZIP entry sizes, requested sheet count, total sheet
   count, shared-string size, open time, and decode time.
2. Build the complete required-sheet set before opening a workbook, including
   normal, merger, scatter, and refer dependencies.
3. Decode each required sheet exactly once and release workbook resources after
   the final required sheet.
4. Prototype a read-only selective XLSX decoder behind the importer boundary if
   the library still inflates or parses substantial unrequested data. It must
   handle shared strings, inline strings, booleans, numbers, cached formula
   values, empty cells, relationships, and XML namespaces before replacing the
   current path.
5. Retain the prototype only if it improves end-to-end wall time on the
   production workload and passes all XLSX fixtures.

Acceptance gates:

- `source_open` plus `sheet_decode` CPU falls by at least 25%.
- Total allocation falls by at least 20% from the fresh baseline.
- End-to-end warm median improves by at least 15% from the consolidated
  candidate.
- Open file handles and temporary files remain bounded.
- CSV, XML, and YAML behavior is unchanged.

### 3. Finish a reusable sheet parse plan

Priority: P1

The retained caches remove several repeated row-loop operations. Continue only
where profiles still show repeated reflection or allocation:

1. Compile field actions for each message and layout before parsing data rows.
2. Store resolved descriptors, column indexes, optional behavior, map keys,
   layouts, and canonical refer keys in that plan.
3. Execute the plan per row without descriptor walks or static string building.
4. Keep pooled ownership explicit. Add pooling only when a benchmark shows a
   reduction in allocation and every acquisition has one release point.

Acceptance gates:

- `sheet_parse` CPU improves by at least 10%.
- End-to-end warm median improves by at least 5%.
- Sparse, transposed, vertical, incell, merger, scatter, and patch fixtures
  produce identical output.

### 4. Shorten refer critical paths without eager global preloading

Priority: P1

The blanket asynchronous preloader increased contention, so retain lazy
single-load behavior. Use the labeled CPU and block profiles to optimize only
refer keys on the generation critical path:

1. Record load count, cache hit count, waiter count, load duration, and wait
   duration per canonical `<fully-qualified-message>.<column>` key.
2. Identify dependency chains that block output progress.
3. If a stable dependency graph exists, schedule only independent critical
   refer targets before their consumers with bounded concurrency.
4. Cache successful values and failures exactly once per generation run.

Acceptance gates:

- Unique importer loads do not increase.
- Aggregate and critical-path refer wait both decrease.
- Each canonical key is parsed at most once.
- The end-to-end warm median improves; reduced blocked time alone is
  insufficient.

### 5. Collapse JSON output passes

Priority: P2

The JSON path currently builds protobuf JSON, performs a second parse for
timezone rewriting, marshals again, compacts, and optionally indents. Replace
the repeated object and byte transformations with one semantic traversal and
one final encoding step.

Acceptance gates:

- Representative JSON output is byte-identical, including map order, enum
  formatting, default values, and whitespace.
- JSON CPU and allocation both fall by at least 25%.
- Binary and text output paths are unaffected.
- End-to-end warm median improves by at least 5% on this workload.

### 6. Revisit concurrency after reducing per-task memory

Priority: P2

More concurrency did not help while each workbook decode allocates heavily.
After the XLSX and parser changes, use a bounded global scheduler for source and
sheet work. Set the bound from measurements rather than CPU count alone because
decompression, XML parsing, garbage collection, and storage compete for memory
bandwidth.

Keep a scheduler change only when it improves both median and p95 without
raising peak memory or open file handles beyond an agreed bound.

## Profiling workflow

Use the same input snapshot, generated proto files, machine power state, and log
configuration for every comparison.

1. Build baseline and candidate executables from recorded commits.
2. Run one unmeasured warm-up for each binary.
3. Run at least five interleaved normal-mode measurements. Report median and
   p95 wall time.
4. Run each binary with `--profiling`. Retain CPU, `alloc_space`, `inuse_space`,
   and block profiles.
5. Record importer cache requests, unique source opens, decoded sheets, cache
   hits, and refer load/wait counters.
6. Compare generated output files byte-for-byte.
7. Run focused unit and integration tests, followed by `go test ./...` when
   practical. Do not use `-race` on this Windows environment because Go was
   built with CGO disabled.

Useful profile commands from the profile output directory include:

```powershell
go tool pprof -top .\confgen-cpu.pprof
go tool pprof -tags .\confgen-cpu.pprof
go tool pprof -top -tagfocus='work=sheet_decode' .\confgen-cpu.pprof
go tool pprof -top -sample_index=alloc_space .\confgen-mem.pprof
go tool pprof -top -sample_index=delay .\confgen-block.pprof
```

Apply one optimization phase at a time. Keep a change only when normal-mode wall
time improves and correctness gates pass. A smaller pprof number by itself is
not a user-visible performance gain.

