#!/usr/bin/env bash
# Refreshes the embedded/ directory consumed by embed.go.
#
# Invoked by `go generate ./...` (see embed.go).
#
# `buf export` ignores buf.lock, so we read the pinned protovalidate commit
# from buf.lock ourselves and pass it explicitly to stay in sync.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
BUF_LOCK="$REPO_ROOT/buf.lock"

rm -rf embedded

commit=$(awk '
  /name: buf.build\/bufbuild\/protovalidate/ { found = 1 }
  found && /commit:/ { print $2; exit }
' "$BUF_LOCK")

if [ -z "$commit" ]; then
    echo "gen_embedded.sh: failed to read protovalidate commit from $BUF_LOCK" >&2
    exit 1
fi

buf export "buf.build/bufbuild/protovalidate:$commit" -o embedded
