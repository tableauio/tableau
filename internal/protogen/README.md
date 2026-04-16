# Protogen Module

Protogen is the core module responsible for generating Protocol Buffer files from various data formats (Excel, CSV, XML, YAML). It implements a sophisticated parsing hierarchy with robust error collection mechanisms.

## Parsing Hierarchy

Protogen employs a multi-level parsing hierarchy to handle complex data structures efficiently:

### 1. Generator Level (Top Level)
- **Scope**: Across all concurrent workbooks
- **Responsibility**: 
  - Manages the overall generation process
  - Coordinates concurrent parsing of multiple workbooks
  - Maintains global type information registry
  - Handles two-pass parsing strategy

### 2. Book Level
- **Scope**: Within a single workbook
- **Responsibility**:
  - Parses workbook metadata and options
  - Manages multiple sheets within the workbook
  - Handles subdirectory rewrites and aliases
  - Coordinates first-pass and second-pass parsing

### 3. Sheet Level
- **Scope**: Within a single worksheet
- **Responsibility**:
  - Parses sheet header and metadata
  - Handles different sheet modes (default, enum, struct, union)
  - Manages table header parsing and validation
  - Coordinates field-level parsing

### 4. Field/Message Level
- **Scope**: Individual fields within a sheet
- **Responsibility**:
  - Parses field types (scalar, enum, list, map, struct)
  - Handles complex type layouts (vertical, horizontal, incell)
  - Validates type references and constraints
  - Processes field properties and options

## Error Collector Hierarchy

Protogen implements a hierarchical error collection system with configurable limits at each level:

### Error Collector Levels

```mermaid
flowchart TB
    A[Generator Collector<br/>maxErrors: 10] --> B[Book Collector<br/>maxErrorsPerBook: 5]
    B --> C[Sheet Collector<br/>limit: 3 (configurable)]
    C --> D[Message/Field Collector<br/>limit: 2 (configurable)]
```

### Error Collection Behavior

1. **Fail-fast Mechanism**: Each collector becomes "full" when its error limit is reached
2. **Hierarchical Propagation**: Errors bubble up from lower levels to higher levels
3. **Structured Errors**: Rich error context with workbook, sheet, and field information
4. **Concurrent Safety**: Thread-safe error collection across concurrent workbooks

## Two-Pass Parsing Strategy

Protogen employs a sophisticated two-pass parsing strategy:

### First Pass
- **Purpose**: Extract type information from special sheet modes
- **Processed Modes**: 
  - `MODE_ENUM_TYPE` and `MODE_ENUM_TYPE_MULTI`
  - `MODE_STRUCT_TYPE` and `MODE_STRUCT_TYPE_MULTI`
  - `MODE_UNION_TYPE` and `MODE_UNION_TYPE_MULTI`
- **Output**: Populates type information registry

### Second Pass
- **Purpose**: Parse sheet schemas and generate protobuf definitions
- **Processed Modes**: All modes including `MODE_DEFAULT`
- **Output**: Generated .proto files

## Supported Data Types and Layouts

### Basic Types
- **Scalar Types**: int32, int64, float, double, string, bool, bytes
- **Enum Types**: User-defined enumerations
- **List Types**: Repeated fields with various layouts
- **Map Types**: Key-value mappings
- **Struct Types**: Complex nested structures

### Layout Patterns

#### 1. Vertical Layout
- **Description**: Fields span multiple rows
- **Use Case**: Complex nested structures
- **Example**: Struct fields defined in consecutive rows

#### 2. Horizontal Layout
- **Description**: Fields span multiple columns
- **Use Case**: Arrays and maps with numbered elements
- **Example**: `TaskParam1`, `TaskParam2`, `TaskParam3`

#### 3. In-cell Layout
- **Description**: Complex types defined within a single cell
- **Use Case**: Simple lists and maps of scalar types
- **Example**: `map<int32, string>` or `[]int32`

## Special Sheet Modes

Protogen supports several special sheet modes for type definition:

### Enum Type Modes
- `MODE_ENUM_TYPE`: Single enum type definition
- `MODE_ENUM_TYPE_MULTI`: Multiple enum types in one sheet

### Struct Type Modes
- `MODE_STRUCT_TYPE`: Single struct type definition
- `MODE_STRUCT_TYPE_MULTI`: Multiple struct types in one sheet

### Union Type Modes
- `MODE_UNION_TYPE`: Single union type definition
- `MODE_UNION_TYPE_MULTI`: Multiple union types in one sheet

## Concurrency Model

Protogen is designed for concurrent processing:

### Concurrent Workbook Processing
- Multiple workbooks are processed concurrently using error group pattern
- Each workbook has its own book-level error collector
- Global error collector aggregates errors from all workbooks

### Thread Safety
- Type information registry is thread-safe
- Error collectors are designed for concurrent access
- Cached importers are protected by mutex

## Error Handling Best Practices

1. **Use Structured Errors**: Always provide rich context using `xerrors.NewKV` or `xerrors.WrapKV`
2. **Respect Error Limits**: Check collector status with `IsFull()` before collecting new errors
3. **Hierarchical Propagation**: Let errors bubble up naturally through the hierarchy
4. **Context Preservation**: Maintain workbook and sheet context in error messages

## Testing

The module includes comprehensive tests in:
- `collector_test.go`: Error collector hierarchy tests
- `protogen_test.go`: End-to-end generation tests
- `table_parser_test.go`: Table parsing functionality tests
- `sheet_mode_test.go`: Special sheet mode tests

## Performance Considerations

1. **Caching**: Importers are cached to avoid redundant parsing
2. **Concurrent Processing**: Workbooks are processed in parallel
3. **Early Termination**: Error limits prevent excessive error collection
4. **Memory Efficiency**: Hierarchical error collection minimizes memory usage