package protoc

import (
	"embed"
	"io/fs"
)

// Embedded .proto sources used as fallback dependencies for protocompile's
// experimental compiler. The experimental compiler only accepts source files
// (via [source.Opener]) and does not support injecting pre-compiled
// FileDescriptors via protoreglistry.GlobalFiles like the stable Compiler did.
//
// We embed:
//   - tableau/protobuf/*.proto (this repository's own schemas, copied from
//     proto/tableau/protobuf/)
//   - buf/validate/*.proto (the protovalidate schemas, exported from the buf
//     module pinned in buf.lock)
//
// Standard imports (google/protobuf/*) are provided automatically by
// [source.WKTs].
//
// The contents under embedded/ ARE checked into version control so that
// downstream consumers (e.g. `go install github.com/tableauio/tableau/cmd/
// tableauc@latest`) can build without running `go generate` or having `buf`
// installed. Run `go generate ./...` after editing proto/tableau/protobuf/
// or bumping the protovalidate dep in buf.lock to refresh embedded/.
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
//go:embed embedded/tableau/protobuf/tableau.proto
//go:embed embedded/tableau/protobuf/wellknown.proto
var embeddedProtos embed.FS

// embeddedFS returns the embedded.FS rooted at "embedded/" so that paths
// look like "buf/validate/validate.proto" or "tableau/protobuf/tableau.proto".
func embeddedFS() fs.FS {
	sub, err := fs.Sub(embeddedProtos, "embedded")
	if err != nil {
		// Should never happen because the directory is hard-coded.
		panic(err)
	}
	return sub
}
