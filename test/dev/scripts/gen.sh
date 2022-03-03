#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

bash ./scripts/gen_pb.sh

TABLEAU_PROTO_PATH="./proto"
INDIR="./test/dev/protoconf"
OUTDIR="./test/dev/testpb"

# remove generated dir
rm -rfv $OUTDIR
mkdir -p $OUTDIR

for item in "$INDIR"/* ; do
    echo "$item"
    if [ -f "$item" ]; then
        protoc \
        --go_out="$OUTDIR" \
        --go_opt=paths=source_relative \
        --proto_path="$INDIR" \
        --proto_path="$TABLEAU_PROTO_PATH" \
        "$item"
    fi
done
