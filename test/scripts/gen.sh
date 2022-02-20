#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

bash ./scripts/gen_pb.sh

tableau_proto="./proto"
test_indir="./test/protoconf"
test_outdir="./test/testpb"


gen_pb()
{
    # remove generated files
    rm -rf $test_outdir/$1
    mkdir -p $test_outdir/$1

    for item in "$test_indir/$1"/* ; do
        echo "$item"
        if [ -f "$item" ]; then
            protoc \
            --go_out="$test_outdir/$1" \
            --go_opt=paths=source_relative \
            --proto_path="$test_indir/$1" \
            --proto_path="$tableau_proto" \
            "$item"
        fi
    done
}
gen_pb excel
gen_pb xml