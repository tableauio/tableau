# Profiling guide

Use profiling to locate expensive work, form a targeted optimization, and
verify that it improves normal end-to-end runtime without changing outputs.

## Collect profiles

Enable profiling for a confgen or protogen run with `--profiling`:

```powershell
$ConfigFile = "<path-to-config.yaml>"
tableauc --mode conf `
  --config $ConfigFile `
  --profiling `
  --proto-package protoconf `
  --indir . `
  --outdir .

tableauc --mode proto `
  --config $ConfigFile `
  --profiling `
  --proto-package protoconf `
  --indir . `
  --outdir .
```

Set `$ConfigFile` to the configuration file for your workspace, and run from
the corresponding input directory. Profiling writes CPU, memory, and block
profiles to the configured output directory:

- `<generator>-cpu.pprof`
- `<generator>-mem.pprof`
- `<generator>-block.pprof`

The run also reports per-sheet parser metrics. Profiling adds overhead, so use
normal-mode runs to compare end-to-end performance.

## Inspect profiles

Run these commands from the directory containing the profile files. Replace
`confgen` with `protogen` for protogen profiles:

```powershell
go tool pprof -top .\confgen-cpu.pprof
go tool pprof -tags .\confgen-cpu.pprof
go tool pprof -top -tagfocus='work=sheet_parse' .\confgen-cpu.pprof
go tool pprof -top -sample_index=alloc_space .\confgen-mem.pprof
go tool pprof -top -sample_index=inuse_space .\confgen-mem.pprof
go tool pprof -top -sample_index=delay .\confgen-block.pprof
```

Use CPU profiles to find processor-heavy functions and labels. Allocation
profiles show where memory was allocated; in-use profiles show retained
memory. Block profiles help locate goroutines waiting on synchronization.
Sheet wall time includes both execution and waiting, so use labeled CPU time
when choosing a CPU optimization target.

## Benchmark changes

1. Use the same input files, generated protos, runtime settings, and machine
   conditions for each build.
2. Build baseline and candidate versions, then warm each once.
3. Alternate at least five normal-mode runs per version. Record raw times,
   median, and p95.
4. Collect profiles to explain the measured difference and identify the next
   target.
5. Compare generated outputs byte-for-byte and run relevant tests and checks.

Keep an optimization only when it improves end-to-end runtime and preserves
output correctness. A smaller profile sample alone does not establish a speedup.
