# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Tableau is a Go-based configuration converter that transforms Excel/CSV/XML/YAML files into protobuf-defined configuration files (JSON, Text, Bin). It uses Protocol Buffers (proto3) to define the structure of input data, extended with custom tableau options (field numbers 50000-99999 on `google.protobuf.*Options`).

## Common Commands

### Build & Run
```bash
go build ./...
go install github.com/tableauio/tableau/cmd/tableauc@latest
```

### Testing
```bash
# Run all unit tests
go test -v -timeout 30m -race ./...

# Run a single test
go test -v -run TestFunctionName ./path/to/package/

# Run functional tests (from repo root) — builds coverage-instrumented binary, runs it, compares golden files
./test/functest/run.sh

# Run benchmarks (profiling)
go test -bench=. ./test/bench/
go test -run ^Test_genConf$ -cpuprofile=cpu.prof ./test/bench/
go tool pprof -http :8888 cpu.prof
```

### Vet & Lint
```bash
go vet ./...

# Full lint (CI uses golangci-lint v2.2.1)
golangci-lint run

# Buf proto linting & build
buf lint
buf build
buf generate            # Regenerate Go code from proto files
buf dep update          # Update dependencies in buf.lock
```

### Error Code Generation
```bash
# Regenerate ecode_generated.go from i18n config (via go:generate directive in internal/tools/generate.go)
go generate ./internal/tools/
# Or regenerate all generated code
go generate ./...
```

## Architecture

### Two-Phase Generation Pipeline

The core pipeline has two phases, both orchestrated from `tableau.go`:

1. **protogen** (`internal/protogen/`): Reads Excel/CSV/XML/YAML workbooks and generates `.proto` files. Parses sheet headers (name row, type row, note row) to infer protobuf message structure with tableau-specific annotations.

2. **confgen** (`internal/confgen/`): Reads the same workbooks using the generated `.proto` definitions and produces output configuration files (JSON/Text/Bin). Validates data using `buf.build/go/protovalidate`.

Both generators use **concurrent processing** with hierarchical error collectors that cap errors at each level:
- protogen: generator(10) -> book(5) -> sheet(3)
- confgen: generator(20) -> book(10) -> sheet(5) -> message(3)

### Public API (`tableau.go`)

```go
// Full pipeline: proto + conf
Generate(protoPackage, indir, outdir string, setters ...options.Option) error

// Individual phases
GenProto(protoPackage, indir, outdir string, setters ...options.Option) error
GenConf(protoPackage, indir, outdir string, setters ...options.Option) error

// Generator constructors (for programmatic use)
NewProtoGenerator(protoPackage, indir, outdir string, options ...options.Option) *protogen.Generator
NewConfGenerator(protoPackage, indir, outdir string, options ...options.Option) *confgen.Generator

// Utilities
SetLang(lang string) error
NewImporter(workbookPath string) (importer.Importer, error)
GetVersionInfo() *VersionInfo
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `cmd/tableauc/` | CLI tool (cobra). Modes: `default`, `proto`, `conf`. Supports YAML config file (`-c`). |
| `internal/protogen/` | Generates `.proto` from workbook structure. Table parser + document parser. |
| `internal/confgen/` | Generates config from workbooks + proto descriptors. Table parser + document parser. |
| `internal/confgen/fieldprop/` | Field property validation: unique, sequence, order, range, refer, presence. |
| `internal/importer/` | `Importer` interface with Excel/CSV/XML/YAML implementations. |
| `internal/importer/book/` | In-memory workbook/sheet/table/row/node representation. |
| `internal/importer/book/tableparser/` | Table header parsing (name/type/note rows, data rows). |
| `internal/importer/metasheet/` | `@TABLEAU` metasheet parsing and context. |
| `format/` | Input formats (Excel, CSV, XML, YAML) and output formats (JSON, Bin, Text). |
| `options/` | Functional options pattern. YAML-serializable. `NewDefault()` for defaults. |
| `load/` | Runtime: load generated config files back into protobuf messages. Supports patch/merge/replace modes. |
| `store/` | Runtime: serialize protobuf messages to JSON/Text/Bin files. |
| `proto/tableau/protobuf/` | Source `.proto` files. Published to BSR as `buf.build/tableauio/tableau`. |
| `proto/tableaupb/` | Generated Go code from protos (**do not edit manually**). |
| `log/` | Structured logging via zap. Pluggable driver interface (`log/driver/`). |
| `internal/x/xerrors/` | Hierarchical error collection, structured key-value errors, stack traces, error codes (E0001-E3003). |
| `internal/x/xfs/` | Filesystem utilities (subdir rewrite, path cleaning, permissions). |
| `internal/x/xproto/` | Protobuf helpers: value parsing, merge, patch, union detection, type info. |
| `internal/x/xproto/protoc/` | Protobuf compiler wrapper using `protocompile`. |
| `internal/strcase/` | CamelCase/snake_case conversion with configurable acronyms. |
| `internal/types/` | Type matching (map, list, well-known messages), regex patterns for type DSL. |
| `internal/localizer/` | i18n support (BCP 47 language tags: en, zh). |
| `internal/printer/` | Code generation printer utility (like `protoc-gen-go`'s approach). |
| `internal/excel/` | Excel utilities (column letter axis, file creation). |
| `internal/testutil/` | Shared test helpers. |

### Workbook Concepts

- A **Book** contains multiple **Sheets** (like Excel tabs).
- Each sheet has a **header section** (name row, type row, note row) followed by data rows. Default layout: row 1=names, row 2=types, row 3=notes, row 4+=data.
- A special **metasheet** (named `@TABLEAU` by default) contains per-sheet configuration.
- CSV files use the naming pattern `<BookName>#<SheetName>.csv` to represent multiple sheets in a virtual workbook.
- **Table sheets** (Excel/CSV): row-oriented, parsed by `tableParser`.
- **Document sheets** (XML/YAML): tree-structured, parsed by `documentParser`.

### Sheet Processing Modes

- **Scatter**: One sheet definition produces multiple output files (e.g., from glob-matched workbooks). Configured via `WorksheetOptions.scatter`.
- **Merger**: Multiple sheets/workbooks are merged into one output file. Configured via `WorksheetOptions.merger`. Uses map-reduce pattern for concurrency.
- **Patch**: Supports `PATCH_REPLACE` and `PATCH_MERGE` modes for overlaying configuration.

### Proto Extensions (Custom Options)

Tableau extends protobuf descriptors at field numbers 50000:
- `tableau.workbook` (FileOptions): workbook path, header layout.
- `tableau.worksheet` (MessageOptions): sheet name, layout, transpose, scatter/merger, patch.
- `tableau.field` (FieldOptions): column name, key, layout (vertical/horizontal/incell), prop (unique/sequence/order/range/refer/presence).
- `tableau.etype` / `tableau.evalue` (EnumOptions/EnumValueOptions): enum name aliases.
- `tableau.struct` / `tableau.union` (MessageOptions): custom struct/union annotations.
- `tableau.oneof` (OneofOptions): oneof field mapping.

### Concurrency Model

- `xerrors.Collector`: Thread-safe error accumulator with configurable max capacity. Forms parent-child hierarchies. Uses `golang.org/x/sync/errgroup` for goroutine management.
- `Collector.NewGroup()`: Creates an errgroup with the collector's context; goroutines call `g.Go()`.
- `Collector.NewChild()`: Creates a child collector with its own capacity, registered under the parent.
- The collector stops accepting new errors once full (`IsFull()` returns true), propagating early termination up the tree.

### Error Handling

- Structured errors with key-value fields (`xerrors.NewKV`, `xerrors.WrapKV`).
- Each error chain has exactly one stack trace (captured at creation via `callers()`).
- Error codes (E0001-E3003) are generated from i18n config via `internal/tools/cmd/ecode`. Source: `internal/x/xerrors/ecode_generated.go`.
- Errors carry structured fields: `module`, `bookName`, `sheetName`, `pbMessage`, `position` (Excel cell coordinates).
- At debug log level, errors include full stack traces (`%+v`); at higher levels, concise format (`%v`).

### Configuration

- `options/options.go` defines all configuration via the YAML-serializable `Options` struct (functional options pattern).
- CLI config file (`tableauc -c config.yaml`) and programmatic `options.Option` closures use the same structure.
- `options.NewDefault()` returns sensible defaults. Use `tableauc -s` to print a sample config.

### Proto Integration

- Proto definitions live in `proto/tableau/protobuf/` and are published to the Buf Schema Registry (BSR) as `buf.build/tableauio/tableau`.
- `buf.yaml` defines two modules: a named one (published, excludes `internal/` and `unittest/`) and an unnamed one (for internal protos used in testing).
- Generated Go code goes to `proto/tableaupb/` via `buf generate`.
- CI checks that generated `.pb.go` files are committed and up-to-date.
- Dependency: `buf.build/bufbuild/protovalidate` for field validation.

### Testing Strategy

- **Unit tests**: `go test ./...` covers individual packages. Uses `testify` for assertions.
- **Integration tests**: `internal/confgen/collector_integration_test.go` and `internal/protogen/collector_integration_test.go` use testdata protos parsed at runtime.
- **Functional tests**: `test/functest/` builds a coverage-instrumented binary, runs generation, then compares output against golden files. Covers both proto generation and conf generation.
- **Benchmarks**: `test/bench/` with profiling support (CPU/memory).
- **Test data**: `testdata/unittest/` (CSV fixtures with `#`-separated book/sheet names + expected outputs in `conf/`, `patchconf/`, `invalidconf/` subdirs).
- **CI matrix**: Ubuntu + Windows, Go 1.24.x, x86 + x64.

### Versioning

- Module version in `version.go` (const `version`).
- Sub-module versions: `internal/protogen/version.go` and `internal/confgen/version.go`.
- CLI releases use tag pattern: `cmd/tableauc/vX.Y.Z`.
- Cross-platform release builds: linux/amd64, darwin/amd64+arm64, windows/amd64.

## Code Conventions

### Patterns to Follow

- **Functional options**: Use `options.Option` closures for configurable constructors.
- **Context propagation**: Pass `context.Context` through the call chain; embed custom state via `strcase.NewContext()`, `metasheet.NewContext()`.
- **Error wrapping**: Always use `xerrors.WrapKV` with structured keys (`KeyModule`, `KeyBookName`, `KeySheetName`, etc.) to preserve diagnostic context.
- **sync.Pool**: Used for frequently allocated objects (e.g., `tableaupb.FieldOptions` in `fieldOptionsPool`).
- **Map-reduce**: Concurrent parsing of multiple importers uses `Collector.NewGroup().Go()` for fan-out, mutex-guarded slice for fan-in.
- **Interface-based importers**: All input formats implement the `Importer` interface (`Filename()`, `BookName()`, `Format()`, `GetSheets()`, `GetSheet(name)`).

### Naming

- Proto field names use `snake_case`; Go struct fields use `PascalCase`.
- CSV files: `<BookName>#<SheetName>.csv`.
- Generated config files: `<MessageName>.<ext>` (e.g., `ItemConf.json`).
- Error codes: `E` + 4 digits (E0xxx = general, E2xxx = validation, E3xxx = I/O).

### Files Not to Edit Manually

- `proto/tableaupb/*.pb.go` — regenerate with `buf generate`.
- `internal/x/xerrors/ecode_generated.go` — regenerate with `internal/tools/cmd/ecode`.

### Key Dependencies

| Dependency | Purpose |
|-----------|---------|
| `google.golang.org/protobuf` | Protobuf runtime (reflection, dynamic messages, encoding) |
| `buf.build/go/protovalidate` | Field validation against proto constraints |
| `github.com/bufbuild/protocompile` | Runtime proto compilation (no protoc binary needed) |
| `github.com/xuri/excelize/v2` | Excel (.xlsx) reading/writing |
| `github.com/subchen/go-xmldom` | XML DOM parsing |
| `gopkg.in/yaml.v3` | YAML parsing |
| `github.com/valyala/fastjson` | Fast JSON utilities |
| `github.com/spf13/cobra` | CLI framework |
| `go.uber.org/zap` | Structured logging |
| `golang.org/x/sync` | errgroup for concurrent processing |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/emirpasic/gods` | Data structures (arraylist for sorted stats) |
| `github.com/rogpeppe/go-internal` | Test scripting utilities |
