# confgen — Configuration Generation

Converts workbook data (Excel/CSV/XML/YAML) into protobuf messages with
concurrent parsing and a hierarchical error collector for multi-level error limiting.

## Parsing Hierarchy

```
Generator
 ├── GenAll / GenWorkbook
 │    └── collector.NewGroup(ctx)                    ← concurrent workbook batch
 │         └── Group.Go(convert)                     ← one goroutine per proto file
 │
 └── convert(fd)                                     ← sequential per-sheet loop within one workbook
      ├── processScatter → ScatterAndExport
      │    ├── parseMessageFromOneImporter(main)      ← main importer: sequential
      │    └── collector.NewGroup(ctx)                ← concurrent scatter batch
      │         └── Group.Go(parseMessageFromOneImporter)
      │
      └── processMerger → MergeAndExport
           └── ParseMessage
                ├── single importer → parseMessageFromOneImporter   ← sequential
                └── multiple importers
                     └── collector.NewGroup(ctx)                    ← concurrent merge batch
                          └── Group.Go(parseMessageFromOneImporter)
```

### parseMessageFromOneImporter (leaf)

```
parseMessageFromOneImporter(info, collector, impInfo)
 └── sheetCollector = collector.NewChild(maxErrorsPerSheet=5)
 └── sheetParser.Parse(protomsg, sheet)
      ├── [document sheet] → documentParser.Parse
      │    └── parseMessage(node)                    ← recursive tree walk
      │         └── messageCollector = sheetCollector.NewChild(maxErrorsPerMessage=3)
      │
      └── [table sheet]    → tableParser.Parse
           └── tableParser.parse
                └── RangeDataRows(row callback)
                     └── parseMessage(row)           ← per row
                          └── messageCollector = sheetCollector.NewChild(maxErrorsPerMessage=3)
```

## Concurrent Model

```mermaid
flowchart TB
    subgraph Generator
        C["gen.collector (maxParseErrors=20)"]
    end

    subgraph "Workbook Group (concurrent)"
        direction TB
        G1["goroutine: convert(fd₁)"]
        G2["goroutine: convert(fd₂)"]
        Gn["goroutine: convert(fdₙ)"]
    end

    Generator --> G1 & G2 & Gn

    subgraph "convert(fd) — sequential sheet loop"
        B["bookCollector = gen.collector.NewChild(maxErrorsPerBook=10)"]
        S1["sheet₁: processScatter → ScatterAndExport"]
        S2["sheet₂: processMerger → MergeAndExport"]
    end

    G1 --> B --> S1 --> S2

    subgraph "parseMessageFromOneImporter"
        SC["sheetCollector = bookCollector.NewChild(maxErrorsPerSheet=5)"]
        TP["tableParser.parse → RangeDataRows"]
        DP["documentParser.Parse"]
    end

    S1 & S2 --> SC --> TP & DP
```

| Level         | Collector                                       | Limit | Scope                             |
| ------------- | ----------------------------------------------- | ----- | --------------------------------- |
| **Generator** | `gen.collector`                                 | 20    | across all concurrent workbooks   |
| **Book**      | `bookCollector = gen.collector.NewChild(10)`    | 10    | across sheets in one workbook     |
| **Sheet**     | `sheetCollector = bookCollector.NewChild(5)`    | 5     | across messages/rows in one sheet |
| **Message**   | `messageCollector = sheetCollector.NewChild(3)` | 3     | across fields in one message/row  |

## Error Collector

### Hierarchy

Errors are counted at **field level**. The `Collector` forms a tree via
`NewChild(maxErrs)` — each level has its own cap. `Collect()` increments
counters on self and all ancestors; when any level is full, further errors
are dropped at that level. `Join()` recursively assembles the error tree.

```mermaid
flowchart TB
    subgraph "Generator"
        Root["gen.collector  (limit=20)"]
    end

    subgraph "convert(fd)"
        Book["bookCollector = gen.collector.NewChild(10)"]
    end

    subgraph "parseMessageFromOneImporter"
        Sheet["sheetCollector = bookCollector.NewChild(5)"]
    end

    subgraph "parseMessage (per row/node)"
        Message["messageCollector = sheetCollector.NewChild(3)"]
    end

    Root --> Book --> Sheet --> Message
    Message -- "Collect(err) → increments self + sheet + book + root" --> Root
    Sheet -- "IsFull() → fail-fast: skip remaining rows" --> Sheet
    Book -- "Join() → assembles all children errors" --> Book
```

### Fail-fast Behavior

- **Message level**: stops iterating fields when `messageCollector.IsFull()`.
- **Sheet level**: `tableParser.parse` checks `sheetCollector.IsFull()` before each row; returns early if full.
- **Book level**: `convert` checks the error returned by `bookCollector.Collect()`; breaks the sheet loop if full.
- **Generator level**: `collector.NewGroup` propagates the first fatal error (book-full) to stop the workbook goroutine.