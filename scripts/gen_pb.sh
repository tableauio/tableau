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

protoc \
--go_out="$TABLEAU_OUTDIR" \
--go_opt=module="github.com/tableauio/tableau/proto/tableaupb" \
--proto_path="$PROTO_PATH" \
"$TABLEAU_INDIR"/*.proto "$TABLEAU_INDIR"/**/*.proto