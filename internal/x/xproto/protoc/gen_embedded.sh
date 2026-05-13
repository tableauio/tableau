#!/usr/bin/env bash
# Refreshes the embedded/ directory consumed by embed.go.
#
# Invoked by `go generate ./...` (see embed.go). Run from the directory
# containing this script (the dir of embed.go); `go generate` guarantees
# that cwd.
#
# Steps:
#   1. Wipe embedded/ to start clean.
#   2. Copy this repository's own proto/tableau/protobuf/*.proto files
#      (top-level only — internal/ and unittest/ are intentionally skipped
#      to match buf.yaml excludes).
#   3. Read the protovalidate commit pinned in buf.lock and `buf export`
#      that exact commit so the vendored copy stays aligned with buf.lock.
#      `buf export <remote-module>` without a commit suffix pulls latest;
#      buf.lock only governs intra-workspace resolution, not ad-hoc export.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
BUF_LOCK="$REPO_ROOT/buf.lock"

rm -rf embedded
mkdir -p embedded/tableau/protobuf

cp "$REPO_ROOT"/proto/tableau/protobuf/*.proto embedded/tableau/protobuf/

commit=$(awk '
  /name: buf.build\/bufbuild\/protovalidate/ { found = 1 }
  found && /commit:/ { print $2; exit }
' "$BUF_LOCK")

if [ -z "$commit" ]; then
    echo "gen_embedded.sh: failed to read protovalidate commit from $BUF_LOCK" >&2
    exit 1
fi

buf export "buf.build/bufbuild/protovalidate:$commit" -o embedded
