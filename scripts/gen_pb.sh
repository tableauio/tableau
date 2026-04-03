#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

PROTO_PATH="./proto"
TABLEAU_INDIR="${PROTO_PATH}/tableau/protobuf"
TABLEAU_OUTDIR="./proto/tableaupb"

# remove generated files
rm -rfv $TABLEAU_OUTDIR/*.pb.go $TABLEAU_OUTDIR/**/*.pb.go

# Download buf/validate proto files from GitHub
BUF_VALIDATE_PROTO_DIR=$(mktemp -d)
BUF_VALIDATE_VERSION="v1.1.0"
mkdir -p "$BUF_VALIDATE_PROTO_DIR/buf/validate"
curl -sSfL "https://raw.githubusercontent.com/bufbuild/protovalidate/${BUF_VALIDATE_VERSION}/proto/protovalidate/buf/validate/validate.proto" \
    -o "$BUF_VALIDATE_PROTO_DIR/buf/validate/validate.proto"

protoc \
    --go_out="$TABLEAU_OUTDIR" \
    --go_opt=module="github.com/tableauio/tableau/proto/tableaupb" \
    --proto_path="$PROTO_PATH" \
    --proto_path="$BUF_VALIDATE_PROTO_DIR" \
    "$TABLEAU_INDIR"/*.proto "$TABLEAU_INDIR"/**/*.proto

rm -rf "$BUF_VALIDATE_PROTO_DIR"
