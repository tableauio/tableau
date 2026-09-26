# Confgen Profiling and Optimization Plan

This document records the current confgen performance baseline, explains how to
inspect the report, and defines the order in which to optimize the pipeline.
Measurements were captured on Windows from the production-sized configuration
workspace described below.

## View the report in a browser

The original report is a Codex Canvas:

```text
C:\Users\wenchyzhu\.cursor\projects\d-GitHub-tableauio-tableau\canvases\confgen-profile-analysis.canvas.tsx
```

A `.canvas.tsx` file is TypeScript/JSX and imports the virtual `cursor/canvas`
module supplied by the Codex app. A normal browser does not provide that module,
so opening the source file directly cannot render the report.

Use the self-contained browser export instead:

```powershell
Start-Process .\docs\profile\confgen-profile-report.html
```

The HTML file has no external runtime or package dependencies. It can also be
served locally when browser security settings restrict `file:` pages:

```powershell
python -m http.server 8000 --directory docs/profile
```

Then open <http://localhost:8000/confgen-profile-report.html>. Codex can preview
a local route or file-backed page in its built-in browser as described in the
[official Browser documentation](https://learn.chatgpt.com/docs/browser?surface=app).

The Canvas remains the editable in-app source. Update the HTML export when its
measurements or conclusions change.

## Measurement setup

- Repository branch: `codex/confgen-sheet-perf`
- Profile date: 2026-09-25
- Platform: Windows
- Configuration:
  `D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\config.local.yaml`
- Working directory: `D:\Tencent\SVNTest\trunk`
- CPU profile: `conf\confgen-cpu.pprof`
- Heap profile: `conf\confgen-mem.pprof`

Representative command:

```powershell
tableauc --mode conf `
  --config D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\config.local.yaml `
  --profiling `
  --proto-package protoconf `
  --indir . `
  --outdir .
```

The comparison build removed the `logger.Sync()` call performed after every log
record. It retained the same inputs and generated outputs. That change was used
only to isolate the cost of log flushing; the optimization still needs a proper
implementation with one explicit flush at process shutdown.

## Baseline and experiment

| Measurement | Current behavior | Without per-record sync | Change |
| --- | ---: | ---: | ---: |
| Normal wall time | 14.85 s | 6.13 s | 2.42x faster |
| Profiled wall time | 32.27 s | 7.37 s | 4.38x faster |
| CPU samples | 41.17 s | 17.93 s | 56.4% lower |
| Profile capture duration | 15.99 s | 4.80 s | 70.0% lower |
| Total allocation | 7.17 GB | 7.05 GB | effectively unchanged |

The profiled build adds about 20% wall time after removing per-record sync
(6.13 s to 7.37 s). Profiling therefore remains useful for diagnosis, but normal
mode is the source of truth for user-visible speed.

## Findings

### Logging dominates the current run

`ZapDriver.Print` accounts for 22.94 CPU seconds (58.18% of samples), and
`logger.Sync` accounts for 19.72 CPU seconds (50.01%). Flushing the logger for
every record converts buffered logging into repeated filesystem synchronization.
The A/B result makes this the highest-confidence optimization.

### Remaining CPU after removing per-record sync

| Area | CPU | Share |
| --- | ---: | ---: |
| Excel `GetRows` | 6.89 s | 38.4% |
| Merger processing | 5.39 s | 30.1% |
| Sheet parsing | 3.89 s | 21.7% |
| Vertical map parsing, inclusive | 3.41 s | 19.0% |
| Store | 2.43 s | 13.6% |
| GC marking | 2.30 s | 12.8% |
| Refer value loading | 1.88 s | 10.5% |
| JSON marshaling | 1.30 s | 7.3% |

Inclusive values overlap, so their percentages must not be added together.

### Allocation pressure

| Area | Allocated | Share |
| --- | ---: | ---: |
| Excel `GetRows` | 3.55 GB | 50.3% |
| XML decoder `rawToken` | 1.92 GB | 27.2% |
| `parseMessage` | 1.87 GB | 26.5% |
| JSON marshaling | 1.05 GB | 14.9% |
| `fastjson` | 438 MB | 6.2% |
| `Row.AddCell` | 328 MB | 4.6% |

Total allocation is 7.05 GB and retained heap is about 544 MB. Allocation
shares are inclusive and may overlap.

### Importer reuse is already effective

The run issued 417 importer load requests for 210 unique source paths. The cache
reused 207 requests and decoded 561 sheets. The next Excel improvement should
reduce decoding and row materialization within each unique workbook rather than
add another path-level importer cache.

### Sheet wall time is not intrinsic parser CPU

The no-sync run reported these largest sheet labels:

| Sheet | Wall time |
| --- | ---: |
| `DiySkill` | 1.73 s |
| `AiAccountPvpStat` | 0.62 s |
| `AiAccountDisplay` | 0.20 s |
| `Skill_Monster.xlsx#SkillGroupLocal` | 0.18 s |
| `Skill_Player.xlsx#SkillGroupLocal` | 0.10 s |
| `MonsterInfo` | 0.09 s |

A sheet label can include work performed while resolving a cold refer-cache
entry, and a sheet can spend most of its elapsed time waiting for another
goroutine. For example, `AssistSkill.xlsx` took about 2.25 seconds of wall time
but contributed almost no sampled CPU; its stack ended in `waitForReferEntry`.
Rank sheet parser work by CPU samples and separately report refer load and wait
time. Do not infer parser complexity from wall time alone.

On Windows, the measured thread CPU clock has a 15.625 ms resolution. Most
individual sheets therefore report zero processor time. Locking a goroutine to
an OS thread for each profiled sheet also changes scheduling. Pprof labels are a
better source for sheet-level CPU attribution on Windows.

## Optimization plan

### 1. Flush logs once per command

Priority: P0

1. Remove `logger.Sync()` from the per-record `ZapDriver.Print` path.
2. Add an explicit `Sync` operation to the logger driver boundary.
3. Call it once when the CLI command exits, after generation and before returning
   the final status.
4. Keep immediate flushing for fatal or panic paths where the process may exit
   before normal cleanup.
5. Treat benign Windows sync errors for console handles consistently with Zap's
   documented behavior.

Acceptance gates:

- Generated configuration files are byte-identical.
- Log records remain complete on successful and failed commands.
- The median normal-mode wall time reproduces the measured improvement within
  normal run-to-run variance.
- Profiling output is still flushed before the process exits.

### 2. Separate parser work from dependency waits

Priority: P0

1. Add pprof labels for message, sheet, parse pass, and work phase.
2. Split refer metrics into `refer_load` and `refer_wait` durations.
3. Record cache hit, cache miss, and waiter counts for each canonical refer key.
4. Add a block profile to locate synchronization and dependency stalls.
5. On Windows, omit per-sheet OS-thread locking and use pprof labels for CPU
   ranking. Keep wall time, rows, columns, present cells, absent cells, and cache
   counters as independent metrics.

Acceptance gates:

- A sheet waiting on refer data no longer appears as parser CPU.
- The report can distinguish one refer loader from its waiters.
- Profiling mode does not change parser scheduling by pinning sheet goroutines.

### 3. Reduce Excel decoding and row materialization

Priority: P1

1. Build a required-sheet plan before reading each workbook.
2. Decode only sheets required by normal, merger, scatter, and refer dependencies.
3. Avoid materializing duplicate row representations between Excel and the
   importer book model.
4. Close Excel workbook resources immediately after all required sheets have
   been decoded.
5. Measure streaming row iteration against `GetRows` on representative wide and
   sparse workbooks before selecting it.

Acceptance gates:

- All supported origin formats retain identical behavior.
- Excel `GetRows` CPU and allocation bytes fall on the production workload.
- Importer cache hit rate does not regress.
- Open workbook handles stay bounded during concurrent generation.

### 4. Compile a reusable sheet parse plan

Priority: P1

1. Resolve descriptors, layouts, column indexes, prefixes, optional fields, and
   canonical refer keys once per sheet schema.
2. Reuse the plan for each row instead of repeating descriptor and layout work.
3. Replace the sparse `map[int]*Cell` representation with a dense slice when the
   sheet column range is known and bounded.
4. Reuse short-lived parser buffers only when ownership is explicit and a
   benchmark shows a reduction in allocation.

Acceptance gates:

- `parseMessage` and `Row.AddCell` allocation bytes fall.
- Sparse, transposed, vertical, incell, merger, and scatter fixtures remain
  byte-identical.
- Pool ownership remains local and every acquired pooled object has one clear
  release point.

### 5. Schedule and prewarm refer dependencies

Priority: P1

1. Extract canonical refer dependencies from the compiled parse plans.
2. Load independent refer targets concurrently before dependent sheets reach
   field validation.
3. Continue deduplicating each canonical `<fully-qualified-message>.<column>`
   parse through `ReferredCache`.
4. Detect dependency cycles and preserve the existing error context.

Acceptance gates:

- Each canonical refer key is parsed at most once per generation run.
- Wait time falls without increasing unique importer loads.
- Failure results remain cached and report the same structured error.

### 6. Collapse JSON output passes

Priority: P2

The current output path performs protobuf JSON encoding, string conversion,
`fastjson` parsing and marshaling, compaction, and indentation. Replace these
passes with one semantic transformation followed by one final encoding step.

Acceptance gates:

- JSON bytes remain identical for representative outputs, including map order,
  default values, enum formatting, and whitespace.
- JSON marshaling and `fastjson` allocation bytes fall.
- Binary and text output formats remain unaffected.

## Benchmark protocol

Use the same external configuration, input snapshot, generated proto files, and
machine power state for every comparison.

1. Build current and candidate binaries from recorded commits.
2. Run one unmeasured warm-up for each binary.
3. Run at least five sequential normal-mode measurements and report median and
   p95 wall time.
4. Repeat with `--profiling` and retain CPU, `alloc_space`, `inuse_space`, and
   block profiles.
5. Record importer cache requests, unique loads, decoded sheets, and cache hits.
6. Compare every generated output file byte-for-byte.
7. Run focused unit and integration tests plus `go test ./...` when practical.
   Do not use `-race` on this Windows environment because Go was built with CGO
   disabled.

Apply one optimization phase at a time. Keep a change only when normal-mode wall
time improves and correctness gates pass; pprof-only improvements are
insufficient if the user-visible run does not improve.

