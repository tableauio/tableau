package protoc

import (
	"embed"
	"io/fs"
	"strings"

	tableauprotobuf "github.com/tableauio/tableau/proto/tableau/protobuf"
)

// Embedded .proto sources used as fallback dependencies for protocompile's
// experimental compiler. The experimental compiler only accepts source files
// (via [source.Opener]) and does not support injecting pre-compiled
// FileDescriptors via protoreglistry.GlobalFiles like the stable Compiler did.
//
// We embed:
//   - tableau/protobuf/*.proto: this repository's own schemas, reused
//     directly from [tableauprotobuf.FS] (proto/tableau/protobuf/) — no
//     second copy, so it can never go stale.
//   - buf/validate/*.proto: the protovalidate schemas, exported from the buf
//     module pinned in buf.lock (checked into embedded/ below, since this
//     repo has no local source for it).
//
// Standard imports (google/protobuf/*) are provided automatically by
// [source.WKTs].
//
// The contents under embedded/ ARE checked into version control so that
// downstream consumers (e.g. `go install github.com/tableauio/tableau/cmd/
// tableauc@latest`) can build without running `go generate` or having `buf`
// installed. Run `go generate ./...` after bumping the protovalidate dep in
// buf.lock to refresh embedded/.
//
// Version pinning: `buf export <remote-module>` does NOT consult the
// workspace's buf.lock — that file only governs transitive resolution for
// modules declared inside the workspace. Without an explicit commit suffix,
// `buf export` always pulls the latest published commit. To stay aligned
// with buf.lock (the single source of truth for the protovalidate version),
// gen_embedded.sh reads the commit out of buf.lock at generate time and
// passes it to `buf export`. To upgrade, run `buf dep update` then
// `go generate ./...`.

//go:generate ./gen_embedded.sh

//go:embed embedded/buf/validate/validate.proto
var embeddedBufValidate embed.FS

// embeddedFS returns an [fs.FS] that resolves paths like
// "buf/validate/validate.proto" (from the vendored buf export) and
// "tableau/protobuf/tableau.proto" (from [tableauprotobuf.FS], the single
// source of truth also used to build proto/tableaupb).
func embeddedFS() fs.FS {
	bufValidate, err := fs.Sub(embeddedBufValidate, "embedded")
	if err != nil {
		// Should never happen because the directory is hard-coded.
		panic(err)
	}
	return &layeredFS{tableauProtobuf: tableauprotobuf.FS, bufValidate: bufValidate}
}

// layeredFS routes "tableau/protobuf/*" reads to tableauProtobuf (stripping
// the prefix) and everything else to bufValidate.
type layeredFS struct {
	tableauProtobuf fs.FS
	bufValidate     fs.FS
}

// Open implements [fs.FS].
func (l *layeredFS) Open(name string) (fs.File, error) {
	if rest, ok := strings.CutPrefix(name, "tableau/protobuf/"); ok {
		return l.tableauProtobuf.Open(rest)
	}
	return l.bufValidate.Open(name)
}
