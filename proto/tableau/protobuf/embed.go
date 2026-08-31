// Package protobuf embeds this repository's own core .proto sources
// (tableau.proto and wellknown.proto) so that other Go packages — notably
// internal/x/xproto/protoc, which needs their source text to satisfy
// protocompile's experimental [source.Opener] — can reuse them directly
// instead of vendoring a second copy.
package protobuf

import "embed"

// FS embeds tableau.proto and wellknown.proto (top-level only; the
// internal/ and unittest/ subdirectories are intentionally excluded to
// match buf.yaml's unnamed-module includes).
//
//go:embed tableau.proto wellknown.proto
var FS embed.FS
