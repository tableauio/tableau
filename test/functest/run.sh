#!/bin/bash
# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"
cd test/functest

rm -rf covdatafiles
mkdir covdatafiles

# Build mdtool binary for testing purposes.
rm -f functest.exe
go build -cover -o functest.exe .

# Pass in "-cover" to the script to build for coverage, then
# run with GOCOVERDIR set.
GOCOVERDIR=covdatafiles ./functest.exe

# Post-process the resulting profiles.
# go tool covdata percent -i=covdatafiles

# Converting profiles to ‘-coverprofile’ text format
go tool covdata textfmt -i=covdatafiles -o=coverage.txt