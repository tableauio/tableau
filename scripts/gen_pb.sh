#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

PROTO_PATH="./proto"
TABLEAU_INDIR="${PROTO_PATH}/tableau/protobuf"
TABLEAU_OUTDIR="./proto/tableaupb"

# remove generated files
rm -rfv $TABLEAU_OUTDIR/*

for item in "$TABLEAU_INDIR"/* ; do
    echo "$item"
    if [ -f "$item" ]; then
        protoc \
        --go_out="$TABLEAU_OUTDIR" \
        --go_opt=module="github.com/tableauio/tableau/proto/tableaupb" \
        --proto_path="$PROTO_PATH" \
        "$item"
    fi
done