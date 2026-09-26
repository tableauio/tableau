# Confgen Profiling and Optimization Plan

This document explains how to view the performance report, records the measured
confgen optimizations, and identifies the remaining work from the latest profile.
Measurements use the production-sized configuration workspace described below.

## View the report in a browser

The editable report is a Codex Canvas:

```text
C:\Users\wenchyzhu\.cursor\projects\d-GitHub-tableauio-tableau\canvases\confgen-profile-analysis.canvas.tsx
```

The file imports `cursor/canvas`, a virtual component library supplied by the
Codex app. A normal browser cannot resolve that module, so it cannot render the
`.canvas.tsx` source directly.

Use one of these views:

1. Open the Canvas file in Codex for the native interactive view.
2. Open the self-contained browser export:
   [`confgen-profile-report.html`](./confgen-profile-report.html).

On Windows, open the browser export from the repository root:

```powershell
Start-Process .\docs\profile\confgen-profile-report.html
```

If the browser restricts local `file:` pages, serve the directory:

```powershell
python -m http.server 8000 --directory .\docs\profile
```

Then open <http://localhost:8000/confgen-profile-report.html>.

## Measurement setup

- Branch: `codex/confgen-sheet-perf`
- Platform: Windows, Go built with CGO disabled
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

Normal mode is the source of truth for user-visible performance. CPU, allocation,
and block profiles explain the result but add instrumentation overhead.

## Measured results

### P0: logging, profiling attribution, and parser hot paths

The consolidated P0 candidate is commit `3ba7114`. It contains:

- Logger synchronization at command boundaries instead of after every record.
- CPU labels for generator, work phase, format, source, reader, book, sheet,
  message, parse pass, and canonical refer key.
- Separate `refer_load` and `refer_wait` labels plus a block profile.
- Pprof labels instead of per-sheet OS-thread pinning on every platform.
- Dense row cell slices, cached optional fields and map key descriptors, and
  reduced scalar prefix allocation.

A clean six-pair interleaved comparison against commit `58bcdc7` measured:

| Build | Runs (seconds) | Median |
| --- | --- | ---: |
| `58bcdc7` | 4.859, 5.139, 5.074, 5.395, 5.515, 5.713 | 5.267 s |
| `3ba7114` | 4.648, 5.106, 5.199, 5.225, 5.423, 5.426 | 5.212 s |

The consolidated wall-time gain is 1.0%. Earlier sequential measurements that
suggested a larger parser gain did not reproduce under interleaving. The change
still reduced profiled allocation from 7.01 GB to 6.57 GB, about 6.3%, and the
logging change prevents the much larger regression observed with buffered Zap
logging.

### P1: selective raw XLSX reader

The retained P1 implementation reads raw cell values directly from selected
OOXML ZIP parts during cached confgen imports. It avoids eager decompression of
unrelated workbook entries and avoids decoding each worksheet cell into
Excelize's general worksheet model.

The reader supports workbook relationships, shared strings, rich strings,
inline strings, XML entities, Excel character escapes, booleans, numbers,
errors, cached formula values, sparse rows, empty rows, and XML namespace
prefixes. Unsupported XML constructs and worksheet parts larger than 512 MB
fall back to Excelize.

Two independent six-pair interleaved comparisons against `3ba7114` measured:

| Comparison | Existing median | XLSX median | Speedup | Reduction |
| --- | ---: | ---: | ---: | ---: |
| First run | 4.460 s | 2.052 s | 2.17x | 54.0% |
| Hardened reader | 5.698 s | 2.364 s | 2.41x | 58.5% |

The absolute medians changed with machine load, while both alternating runs show
more than a 2x improvement. The hardened result satisfies the requested 2x
additional speedup and exceeds the 4x historical overall target.

### Profile comparison

| Measurement | `3ba7114` | Selective XLSX | Change |
| --- | ---: | ---: | ---: |
| Profiled wall time | 4.915 s | 2.099 s | 57.3% lower |
| CPU samples | 21.83 CPU-s | 11.30 CPU-s | 48.2% lower |
| Allocation space | 6.57 GB | 2.38 GB | 63.8% lower |
| `source_open` CPU | 2.73 s | 0.05 s | 98.2% lower |
| `sheet_decode` CPU | 9.11 s | 2.00 s | 78.0% lower |
| `sheet_parse` CPU | 2.25 s | 1.70 s | 24.4% lower |

The final CPU profile attributes 1.99 CPU-seconds to `reader=xlsx`. No production
workbook used the `reader=excelize` fallback.

### Final rerun with per-sheet CPU labels

Commit `4be1d71` replaces platform-specific thread CPU measurement with pprof
labels and keeps the XLSX reader in its standalone package. The final warm run
completed successfully with these measurements:

- End-to-end profiled wall time: 2.683 s.
- CPU profile duration: 2.23 s with 11.58 CPU-seconds of samples.
- Allocated space: 2,433.36 MB.
- Labeled work: 1.88 CPU-seconds in `sheet_parse`, 1.85 CPU-seconds in
  `sheet_decode`, and 0.06 CPU-seconds in `source_open`.
- Importer cache: 408 requests, 203 source imports, 544 decoded sheets, and 203
  distinct paths.

The immediately preceding selective-XLSX profile recorded 11.30 CPU-seconds,
2,438.87 MB allocated, 1.70 CPU-seconds in `sheet_parse`, 2.00 CPU-seconds in
`sheet_decode`, and 0.05 CPU-seconds in `source_open`. These small differences
are within sampling and host-load variation. The pprof attribution refactor did
not introduce a measurable allocation regression.

One cold run took 6.808 seconds end to end while recording only 7.21
CPU-seconds. The warm repeat above took 2.683 seconds while recording 11.58
CPU-seconds. The inverse relationship shows that the cold result was distorted
by host scheduling or I/O contention. It must not be used as a performance
regression result.

The stable per-sheet CPU ranking from both final runs is:

1. `AiAccountPvpStat.xlsx#AiAccountPvpStatConf`: 690 ms in the warm run.
2. `AiAccountDisplay.xlsx#AiAccountDisplayConf`: 220 ms.
3. `Skill_Monster.xlsx#SkillGroupLocal`: 180 ms.
4. `Skill_Player.xlsx#SkillGroupLocal`: 100 ms.
5. `AssistSkill.xlsx#AssistSkill`: 70 ms.

`AssistSkill` accumulated 873 ms of parser wall time but only 70 ms of sampled
CPU. Its high wall rank therefore reflects time when its goroutine was not
executing, such as scheduling, synchronization, or shared resource contention.
The `sheet_key` CPU label is the correct signal for deciding which parser needs
CPU optimization. By comparison, `Skill_Monster` used 180 ms of CPU for 7,553
rows and 604,240 cells, so its larger input produces the expected higher CPU
cost.

The largest top-level sample is the Windows system-call boundary beneath
`storeMessage`. This is cumulative wall residency sampled while many threads
are inside native calls, rather than 3.28 seconds of processor work. A focused
reproduction wrote the same output set in about 0.10 seconds even though pprof
greatly inflated `runtime.cgocall`. Use wall time or Windows process times to
measure native I/O; do not treat `runtime.cgocall` as file-write CPU.

### Correctness evidence

- The selective reader matches Excelize row-for-row across all 46 XLSX fixtures
  in the repository.
- The production comparison generated 481 files from each binary with zero
  SHA-256 differences.
- `go test ./...` passes.
- `go vet ./...` passes.
- Tests were run without `-race`; this Windows Go installation has CGO disabled.

## Profile findings after the XLSX optimization

The main labeled work is now:

| Work label | CPU | Share of samples |
| --- | ---: | ---: |
| `sheet_decode` | 2.00 s | 17.70% |
| `sheet_parse` | 1.70 s | 15.04% |
| `source_open` | 0.05 s | 0.44% |

The largest remaining allocation paths are:

| Path | Allocation | Share |
| --- | ---: | ---: |
| `fastjson` value cache | 0.43 GB | 18.11% |
| Dynamic protobuf field assignment | 0.22 GB | 9.33% |
| Raw XLSX row parsing | 0.16 GB | 6.54% |
| XLSX ZIP entry buffers | 0.14 GB | 5.70% |
| Dynamic protobuf messages | 0.10 GB | 4.37% |
| JSON byte growth | 0.10 GB | 4.14% |

`store.MarshalToJSON` remains responsible for 1.01 GB of inclusive allocation,
42.51% of the new total. It is now the clearest large allocation target.

## Experiments not retained

- Concurrent sheet decoding from one Excel workbook improved the median by only
  about 0.8% and increased variance.
- Blanket asynchronous refer preloading improved the median by about 0.8% and
  increased cumulative refer wait from 40.86 to 51.20 goroutine-seconds.
- A per-field map of prefixed column names regressed the median to about
  6.3 seconds.

The final profile does not show refer work as a material CPU hotspot. The lazy
canonical refer cache remains the simpler and faster design for this workload.

## Optimization status and next work

### 1. Land and monitor the selective XLSX reader

Priority: P1, completed

The code and correctness gates are complete. Keep the Excelize fallback to
preserve compatibility with uncommon or large worksheets.

### 2. Finish parser plan work only for measured row-loop costs

Priority: P1, optional follow-up

`sheet_parse` is now close to `sheet_decode`. A follow-up should compile field
actions only where CPU profiles still show repeated descriptor lookup or string
construction. Keep it only if it reduces the end-to-end median by at least 5%
and preserves all layout fixtures.

Potential targets:

1. Precompute immutable field operations for each message and layout.
2. Reuse resolved column indexes rather than looking up names for every row.
3. Avoid repeated dynamic-message construction when ownership permits reuse.
4. Keep pool ownership local and explicit.

### 3. Keep refer loading lazy

Priority: P1, resolved by measurement

Do not add global refer preloading. Revisit scheduling only if a future labeled
profile shows a stable critical dependency chain and an end-to-end benchmark
confirms a gain.

### 4. Bypass unnecessary JSON passes

Priority: P2, completed

The production run generates 458 JSON files totaling 50.90 MB, while only 52
files contain populated protobuf timestamps. Previously, `emitTimezones`
parsed every JSON document into a fastjson tree, traversed it, marshaled it,
compacted it, and optionally indented it.

The retained implementation keeps `protojson` as the protobuf compatibility
boundary and bypasses the fastjson tree unless its serialized output can
contain a UTC Timestamp. The descriptor-aware rewrite still changes Timestamp
fields exclusively, so an ordinary string ending in `Z` can only cause an
extra traversal. Pretty output goes directly to `json.Indent` without the
redundant preceding compact pass.

An eight-pair warmed normal-mode comparison measured:

- Existing median: 1.942 seconds.
- Optimized median: 1.828 seconds.
- Median reduction: 5.9%.

The optimized profile reduced total allocation from 2,433.36 MB to 1,765.75
MB, reduced `MarshalToJSON` inclusive allocation from about 1.01 GB to 397.21
MB, and reduced fastjson value-cache allocation from about 420 MB to 45.24 MB.
Profile duration fell from 2.23 seconds to 1.81 seconds. All 458 generated
JSON files and all 23 binary files remained byte-identical.

Acceptance gates:

- JSON output remains byte-identical, including ordering, enum formatting,
  defaults, and whitespace. Completed.
- JSON allocation falls by at least 25%. Completed for total allocation and by
  about 62% for `MarshalToJSON` inclusive allocation.
- Binary and text output are unaffected.
- End-to-end median improves by at least 5%. Completed.

### 5. Keep generated-file writes unchanged

Priority: P2, resolved by measurement

An isolated round of current file writes took about 99-110 ms. Limiting writes
to four concurrent operations reduced that to about 82-84 ms. Reading and
comparing unchanged files reduced isolated write time further, but added a read
of the complete 58 MB output set and improved the end-to-end median by only
about 2%. These gains do not justify extra output policy and allocation in this
change. Revisit unchanged-file detection only when preserving modification
times is itself a product requirement.

### 6. Optimize the raw XLSX scanner after output work

Priority: P3

Within `sheet_decode`, the remaining CPU is concentrated in `xlsx.nextTag`,
`xlsx.parseRows`, byte searching, XML attribute parsing, and DEFLATE decoding.
The reader already cut end-to-end time by more than half, so further scanner
work follows the JSON and file-write targets. Prefer changes that reduce both
the roughly 312 MB allocated by row parsing and ZIP entry buffers and the
1.85 CPU-seconds attributed to sheet decoding.

## Profiling workflow

Use the same input snapshot, generated proto files, power state, and log
configuration for every comparison.

1. Build baseline and candidate executables from recorded commits.
2. Run one unmeasured warm-up for each binary.
3. Run at least five alternating normal-mode pairs and report their raw values,
   median, and p95.
4. Run each binary with `--profiling` and retain CPU, `alloc_space`,
   `inuse_space`, and block profiles.
5. Record importer cache requests, source opens, decoded sheets, reader labels,
   and refer load/wait counters.
6. Compare every generated output file byte-for-byte.
7. Run focused tests, `go test ./...`, and `go vet ./...`. Do not use `-race` on
   this Windows environment because Go was built with CGO disabled.

Useful commands from the profile output directory:

```powershell
go tool pprof -top .\confgen-cpu.pprof
go tool pprof -tags .\confgen-cpu.pprof
go tool pprof -top -tagfocus='work=sheet_decode' .\confgen-cpu.pprof
go tool pprof -top -tagfocus='reader=xlsx' .\confgen-cpu.pprof
go tool pprof -top -tagfocus='sheet_key=<book#sheet (message)>' .\confgen-cpu.pprof
go tool pprof -top -sample_index=alloc_space .\confgen-mem.pprof
go tool pprof -top -sample_index=delay .\confgen-block.pprof
```

Keep a change only when normal-mode wall time improves and correctness gates
pass. A smaller pprof number without an end-to-end gain is insufficient.
