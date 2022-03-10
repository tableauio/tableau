#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

tableau_indir="./proto/tableau/protobuf"
tableau_outdir="./proto/tableaupb"

# remove *.go
rm -fv $tableau_outdir/*.go

for item in "$tableau_indir"/* ; do
    echo "$item"
    if [ -f "$item" ]; then
        protoc \
        --go_out="$tableau_outdir" \
        --go_opt=paths=source_relative \
        --proto_path="$tableau_indir" \
        "$item"
    fi
done