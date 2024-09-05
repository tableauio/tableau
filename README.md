<p align="center">
  <a href="https://tableauio.github.io/">
    <img alt="Tableau" src="https://avatars.githubusercontent.com/u/97329105?s=200&v=4" width="160">
  </a>
</p>

<h3 align="center">
  Modern Configuration Converter
</h3>

<p align="center">
    <a href="https://github.com/tableauio/tableau/actions/workflows/release.yml"><img src="https://github.com/tableauio/tableau/actions/workflows/release.yml/badge.svg" alt="Release Status"></a>
    <a href="https://github.com/tableauio/tableau/actions/workflows/testing.yml"><img src="https://github.com/tableauio/tableau/actions/workflows/testing.yml/badge.svg" alt="Testing Status"></a>
    <a href="https://codecov.io/gh/tableauio/tableau"><img src="https://codecov.io/gh/tableauio/tableau/branch/master/graph/badge.svg" alt="Code Coverage"></a>
    <a href="https://github.com/tableauio/tableau/releases"><img src="https://img.shields.io/github/v/release/tableauio/tableau?include_prereleases&style=flat-square"alt="GitHub release (latest SemVer including pre-releases)"></a>
    <a href="https://pkg.go.dev/github.com/tableauio/tableau"><img src="https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white" alt="go.dev"></a>
    <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/github/license/tableauio/tableau?style=flat-square" alt="GitHub"></a>
</p>

# Tableau

A modern configuration converter based on Protobuf (proto3).

## Prerequisites

- **[Go](https://go.dev/)**: any one of the **three latest major** [releases](https://go.dev/doc/devel/release).

## Installation

### API

Simply add the following import to your code, and then `go [build|run|test]`
will automatically fetch the necessary dependencies:

```go
import "github.com/tableauio/tableau"
```

### tableauc

Install: `go install github.com/tableauio/tableau/cmd/tableauc@latest`

## Features

- Convert **Excel/CSV/XML/YAML** to **JSON/Text/Bin**.
- Use **Protobuf** to define the structure of **Excel/CSV/XML/YAML**.
- Use **Golang** to develop the conversion engine.
- Support multiple programming languages, thanks to **Protobuf (proto3)**.

## Concepts

- Importer:
  - imports a **Excel/CSV** file to a in-memory book of **Table** sheets.
  - imports a **XML/YAML** file to a in-memory book of **Document** sheets.
- Parsers:
  - protogen: converts **Excel/CSV/XML/YAML** files to **Protoconf** files.
  - confgen: converts **Excel/CSV/XML/YAML** with **Protoconf** files to **JSON/Text/Bin** files.
- Exporter:
  - protogen: exports a [tableau.Workbook](https://github.com/tableauio/tableau/blob/master/proto/tableau/protobuf/workbook.proto) to a proto file.
  - confgen: exports a protobuf message to a **JSON/Text/Bin** file.
- Protoconf: a dialect of [Protocol Buffers (proto3)](https://developers.google.com/protocol-buffers/docs/proto3) extended with [tableau options](https://github.com/tableauio/tableau/blob/master/proto/tableau/protobuf/tableau.proto), aimed to define the structure of Excel/CSV/XML/YAML.

## Design

See official document: [Design](https://tableauio.github.io/docs/design/overview/).

## Contribution

### Requirements

#### Protobuf

Goto [Protocol Buffers v21.12](https://github.com/protocolbuffers/protobuf/releases/tag/v21.12), choose and download the correct platform of **protoc**, then install by README.

#### protoc-gen-go

Install: `go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.33.0`
