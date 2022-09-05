<p align="center">
  <a href="https://getdoks.org/">
    <img alt="Tableau" src="https://avatars.githubusercontent.com/u/97329105?s=200&v=4" width="200">
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

## Features

- Convert **Excel/CSV/XML** to **JSON/Text/Wire**, JSON is the first-class citizen of exporting targets.
- Use **Protoconf** as the IDL(Interface Description Language) to define the structure of **Excel/CSV/XML**.
- Use **Golang** to develop the conversion engine.
- Support multiple programming languages, thanks to **Protobuf (proto3)**.

## Concepts

- Importer: Excel/XML importer.
- IR: Intermediate Representation.
- Filter: filter the IR.
- Exporter: JSON (protojson), Text (prototext), and wire (protowire).
- Protoconf: a configuration metadata format based on protobuf.


## Design

See official document: [Design](https://tableauio.github.io/docs/design/overview/).

## Contribution

### Requirements

#### Protobuf

Goto [Protocol Buffers v3.17.3](https://github.com/protocolbuffers/protobuf/releases/tag/v3.17.3), choose and download the correct platform of **protoc**, then install by README.

#### protoc-gen-go

Install: `go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.27.1`
