# Confgen profiling

This guide summarizes the production-sized confgen profile, measured changes,
and the workflow for future performance work. The editable analysis is in the
Codex Canvas `confgen-profile-analysis.canvas.tsx`; open it in Codex to view it.

## Run and inspect profiles

The measurements below used Windows, the Tencent configuration, and the
generated protos from the same input snapshot:

```powershell
tableauc --mode conf `
  --config D:\Tencent\SVNTest\trunk\BuildDataConfig\ConfBuddyTools\LocalCache\tableau\config.local.yaml `
  --profiling `
  --proto-package protoconf `
  --indir . `
  --outdir .
```

The command ran from `D:\Tencent\SVNTest\trunk`. Profiles are written under
the configured output directory as `confgen-cpu.pprof`, `confgen-mem.pprof`,
and `confgen-block.pprof`. Profiling also reports sheet parser metrics.

Useful commands from the profile output directory:

```powershell
go tool pprof -top .\confgen-cpu.pprof
go tool pprof -tags .\confgen-cpu.pprof
go tool pprof -top -tagfocus='work=sheet_decode' .\confgen-cpu.pprof
go tool pprof -top -tagfocus='sheet_key=<book#sheet (message)>' .\confgen-cpu.pprof
go tool pprof -top -sample_index=alloc_space .\confgen-mem.pprof
go tool pprof -top -sample_index=delay .\confgen-block.pprof
```

Use normal-mode runs for end-to-end comparisons; profiling adds overhead.

## Results

### Selective XLSX reader

The reader loads requested worksheet data directly from OOXML and falls back to
Excelize for unsupported constructs or worksheets larger than 512 MB. It
handles shared and inline strings, XML entities, Excel escapes, formulas,
sparse and empty rows, and common cell types.

Two interleaved comparisons against the existing reader showed consistent
improvement despite host-load variation:

| Comparison | Existing median | New median | Faster |
| --- | ---: | ---: | ---: |
| First run | 4.460 s | 2.052 s | 2.17x |
| Hardened reader | 5.698 s | 2.364 s | 2.41x |

The hardened profile reduced CPU samples from 21.83 to 11.30 CPU-s (48.2%),
allocation from 6.57 to 2.38 GB (63.8%), and profiled wall time by 57.3%.
`source_open` CPU fell 98.2%, `sheet_decode` 78.0%, and `sheet_parse` 24.4%.
No production workbook used the Excelize fallback.

### Parser attribution

Pprof labels replace per-sheet OS-thread timing and identify generator, phase,
format, reader, source, book, sheet, message, parse pass, and refer key. A warm
run recorded 11.58 CPU-s across 2.23 seconds of profile time. Its main labeled
work was 1.88 CPU-s in `sheet_parse`, 1.85 CPU-s in `sheet_decode`, and 0.06
CPU-s in `source_open`. The importer handled 408 requests using 203 source
imports and decoded 544 sheets.

The highest per-sheet CPU totals were:

| Sheet | CPU |
| --- | ---: |
| `AiAccountPvpStat.xlsx#AiAccountPvpStatConf` | 690 ms |
| `AiAccountDisplay.xlsx#AiAccountDisplayConf` | 220 ms |
| `Skill_Monster.xlsx#SkillGroupLocal` | 180 ms |
| `Skill_Player.xlsx#SkillGroupLocal` | 100 ms |
| `AssistSkill.xlsx#AssistSkill` | 70 ms |

`AssistSkill` used 873 ms of parser wall time but only 70 ms of sampled CPU.
Wall time includes periods when a goroutine is waiting or unscheduled; use the
`sheet_key` CPU labels to choose parser optimization targets. `Skill_Monster`
used 180 ms of CPU for 7,553 rows and 604,240 cells.

The profile's 3.28 CPU-s attributed beneath `storeMessage` came from cumulative
Windows system-call residency while threads were inside native calls. A focused
write reproduction took about 0.10 seconds. Treat this as sampled native-call
residency, not processor time spent writing files.

### JSON output

The first output change skipped the JSON tree rewrite for documents without
timestamps and removed a redundant compact pass. It improved the normal-mode
median from 1.942 to 1.828 seconds and reduced total allocation from 2,433.36
to 1,765.75 MB. `MarshalToJSON` allocation fell from about 1.01 GB to 397 MB.

The follow-up presents Timestamp values to the official `protojson` encoder as
localized strings through a read-only reflection view. `protojson` retains
ownership of protobuf JSON behavior; `json.Compact` and `json.Indent` provide
stable formatting. This removed the JSON tree and dedicated timestamp rewrite.
The public `store/jsonparser` package remains available.

In paired production runs, the adapter improved the median another 0.7–1.4%.
`MarshalToJSON` allocation fell from 392.14 to 313.86 MB (20.0%), and total
allocation fell 3.3%. All 458 JSON and 23 binary outputs were byte-identical.
A copied local protobuf encoder was rejected: it was 6.4% slower in the
production comparison and added more than 800 lines to maintain. Direct
protojson indentation was also slower than `json.Indent`.

## Remaining work

- Consider compiled field actions or cached column indexes only if a new CPU
  profile shows repeated descriptor lookups or string construction. Keep the
  change only if normal-mode median improves by at least 5% and layout fixtures
  remain correct.
- Keep referred data loading lazy. Preloading improved the median by only 0.8%
  and increased refer wait; the profile does not show refer work as a CPU
  hotspot.
- Further XLSX scanner work is lower priority. Remaining costs include XML tag
  scanning, row parsing, attribute parsing, byte search, and DEFLATE decoding.
- Keep generated-file write behavior unchanged. Limiting concurrent writes
  saved about 20–28 ms in isolation; unchanged-file detection improved the
  end-to-end median by only about 2% while reading the full output set.

## Benchmark workflow

1. Use the same input snapshot, generated protos, power state, and logging.
2. Build baseline and candidate executables, then warm each once.
3. Run at least five alternating normal-mode pairs; report raw times, median,
   and p95.
4. Collect CPU, allocation, in-use heap, and block profiles for diagnosis.
5. Compare all generated files byte-for-byte, then run focused tests,
   `go test ./...`, and `go vet ./...`.

On Windows, do not use `-race`: this Go installation has CGO disabled. Keep a
change only when end-to-end time improves and output correctness is preserved.
