// Package protoc compiles .proto sources into a [protoregistry.Files].
//
// It is built on top of bufbuild/protocompile's experimental incremental
// compiler, which is the only backend that supports Protobuf Edition 2024.
// The stable [github.com/bufbuild/protocompile.Compiler] was bypassed in
// favor of the experimental pipeline because:
//
//   - Edition 2024 features are implemented exclusively on the experimental
//     side (the stable compiler caps its MaxSupportedEdition at 2023).
//   - buf v1.69+ has fully migrated to the experimental compiler.
//
// The experimental compiler only accepts .proto source files (via
// [source.Opener]); it cannot consume pre-compiled descriptors from
// [protoregistry.GlobalFiles]. To preserve the previous behaviour where
// the well-known imports of tableau and protovalidate are resolvable
// without the user supplying them explicitly, this package embeds those
// .proto sources via [embed.FS]. See [embed.go].
package protoc

import (
	"context"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bufbuild/protocompile/experimental/incremental"
	"github.com/bufbuild/protocompile/experimental/incremental/queries"
	"github.com/bufbuild/protocompile/experimental/ir"
	"github.com/bufbuild/protocompile/experimental/report"
	"github.com/bufbuild/protocompile/experimental/source"
	"github.com/bufbuild/protocompile/walk"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// NewFiles creates a new [protoregistry.Files] from the provided proto
// import paths, proto files (globs accepted), and excluded proto files
// (globs accepted).
func NewFiles(protoPaths []string, protoFiles []string, excludedProtoFiles ...string) (*protoregistry.Files, error) {
	cleanSlashProtoPaths := make([]string, len(protoPaths))
	for i, protoPath := range protoPaths {
		cleanSlashProtoPaths[i] = xfs.CleanSlashPath(protoPath)
	}
	parsedExcludedProtoFiles := make(map[string]bool)
	for _, filename := range excludedProtoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			parsedExcludedProtoFiles[cleanSlashPath] = true
		}
	}
	parsedProtoFiles := make(map[string]string) // full path -> rel path
	for _, filename := range protoFiles {
		matches, err := filepath.Glob(filename)
		if err != nil {
			return nil, xerrors.Wrapf(err, "failed to glob files in %s", filename)
		}
		for _, match := range matches {
			cleanSlashPath := xfs.CleanSlashPath(match)
			if !parsedExcludedProtoFiles[cleanSlashPath] {
				rel := rel(cleanSlashPath, cleanSlashProtoPaths)
				parsedProtoFiles[cleanSlashPath] = rel
			}
		}
	}
	return parseProtos(cleanSlashProtoPaths, parsedProtoFiles)
}

func rel(filename string, protoPaths []string) string {
	for _, protoPath := range protoPaths {
		if rel, err := filepath.Rel(protoPath, filename); err == nil {
			return xfs.CleanSlashPath(rel)
		}
	}
	return filename
}

// parseProtos parses the proto paths and proto files to protoregistry.Files
// using protocompile's experimental incremental compiler.
func parseProtos(protoPaths []string, protoFilesMap map[string]string) (*protoregistry.Files, error) {
	log.Debugf("proto paths: %v", protoPaths)
	log.Debugf("proto files: %v", protoFilesMap)

	protoFiles := slices.Collect(maps.Values(protoFilesMap))

	// Build a layered Opener:
	//   1. user sources (resolved against protoPaths, gated by protoFilesMap)
	//   2. embedded sources (tableau + buf/validate)
	//   3. WKTs (google/protobuf/*)
	userOpener := &filesystemOpener{importPaths: protoPaths}
	embeddedOpener := &fsOpener{fs: embeddedFS(), tag: "<tableau-embedded>"}
	combined := &source.Openers{userOpener, embeddedOpener, source.WKTs()}

	workspace := source.NewWorkspace(protoFiles...)
	executor := incremental.New(incremental.WithParallelism(1))
	session := new(ir.Session)

	query := queries.FDS{
		Opener:    combined,
		Session:   session,
		Workspace: workspace,
	}
	results, rpt, err := incremental.Run(context.Background(), executor, query)
	if err != nil {
		return nil, err
	}
	if err := reportToError(rpt); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("protocompile: no result returned")
	}
	if results[0].Fatal != nil {
		return nil, results[0].Fatal
	}
	fds := results[0].Value
	if fds == nil {
		return nil, fmt.Errorf("protocompile: nil FileDescriptorSet")
	}

	// The experimental compiler emits custom options as unknown wire bytes
	// (it has no notion of Go-side extension types, unlike the stable
	// Compiler which used [protoregistry.GlobalFiles] to drive linking).
	// Re-parse each FileDescriptorProto using [protoregistry.GlobalTypes]
	// as the extension resolver so that statically-known extensions
	// (e.g. tableau.workbook, buf.validate.field, ...) become typed
	// fields rather than opaque unknown bytes. Then strip any leftover
	// dynamic extensions, mirroring the previous stable-Compiler flow.
	if err := resolveTypedExtensions(fds); err != nil {
		return nil, err
	}
	for _, fdp := range fds.GetFile() {
		removeDynamicExtensionsFromProto(fdp)
	}
	return protodesc.NewFiles(fds)
}

// resolveTypedExtensions rewrites each FileDescriptorProto in fds so that
// any custom options that arrived as unknown wire bytes are re-decoded
// using the linked-in extension registry ([protoregistry.GlobalTypes]).
//
// This is the bridge between protocompile/experimental's
// "extension-agnostic" output and the rest of tableau, which relies on
// [proto.GetExtension] of typed extensions.
func resolveTypedExtensions(fds *descriptorpb.FileDescriptorSet) error {
	resolver := protoregistry.GlobalTypes
	for i, fdp := range fds.GetFile() {
		data, err := proto.Marshal(fdp)
		if err != nil {
			return xerrors.Wrapf(err, "marshal FileDescriptorProto %q", fdp.GetName())
		}
		clone := &descriptorpb.FileDescriptorProto{}
		if err := (proto.UnmarshalOptions{Resolver: resolver, AllowPartial: true}).Unmarshal(data, clone); err != nil {
			return xerrors.Wrapf(err, "re-unmarshal FileDescriptorProto %q with extension resolver", fdp.GetName())
		}
		fds.File[i] = clone
	}
	return nil
}

// filesystemOpener resolves a proto path against a list of import roots,
// matching the semantics of the stable [protocompile.SourceResolver]:
//
//   - When importPaths is non-empty, the requested path is treated as
//     relative to one of the roots; the verbatim path is NOT tried.
//   - When importPaths is empty, the path is opened verbatim relative to
//     the current working directory.
//
// Opener implementations are required by protocompile to be comparable;
// we use a pointer receiver so identity equality holds across query reuse.
type filesystemOpener struct {
	importPaths []string
}

// Open implements [source.Opener].
func (o *filesystemOpener) Open(path string) (*source.File, error) {
	clean := xfs.CleanSlashPath(path)
	if len(o.importPaths) == 0 {
		if data, err := os.ReadFile(clean); err == nil {
			return source.NewFile(path, string(data)), nil
		}
		return nil, fs.ErrNotExist
	}
	for _, root := range o.importPaths {
		full := xfs.CleanSlashPath(filepath.Join(root, clean))
		if data, err := os.ReadFile(full); err == nil {
			return source.NewFile(path, string(data)), nil
		}
	}
	return nil, fs.ErrNotExist
}

// fsOpener adapts an [fs.FS] (e.g. an embed.FS) to [source.Opener].
//
// Like [filesystemOpener], it must be comparable; we use a pointer receiver
// and only pointer-compare the wrapped fs.FS handle for query caching.
type fsOpener struct {
	fs  fs.FS
	tag string // used as the prefix of File.Path() for diagnostics
}

// Open implements [source.Opener].
func (o *fsOpener) Open(path string) (*source.File, error) {
	clean := strings.TrimPrefix(xfs.CleanSlashPath(path), "./")
	data, err := fs.ReadFile(o.fs, clean)
	if err != nil {
		return nil, err
	}
	return source.NewFile(o.tag+"/"+path, string(data)), nil
}

// reportToError converts protocompile diagnostics into a single error if
// any diagnostic at Error level or above was emitted. Warnings are logged
// but not treated as failures.
//
// Each diagnostic is formatted to match buf's convention:
//
//	<file>:<line>:<col>:<message>
//
// When no source span is available, the line/column components are
// omitted. When no file is available, only the message is emitted.
func reportToError(rpt *report.Report) error {
	if rpt == nil {
		return nil
	}
	var errs []string
	for i := range rpt.Diagnostics {
		d := &rpt.Diagnostics[i]
		msg := formatDiagnostic(d)
		switch d.Level() {
		case report.Error, report.ICE:
			errs = append(errs, msg)
		case report.Warning:
			log.Warnf("protocompile: %s", msg)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("protocompile: %s", strings.Join(errs, "\n"))
}

// formatDiagnostic renders a single diagnostic in buf's
// "<file>:<line>:<col>:<message>" format. Missing components are skipped.
func formatDiagnostic(d *report.Diagnostic) string {
	file := d.File()
	span := d.Primary()
	if !span.IsZero() {
		loc := span.StartLoc()
		if file != "" {
			return fmt.Sprintf("%s:%d:%d:%s", file, loc.Line, loc.Column, d.Message())
		}
		return fmt.Sprintf("%d:%d:%s", loc.Line, loc.Column, d.Message())
	}
	if file != "" {
		return fmt.Sprintf("%s:%s", file, d.Message())
	}
	return d.Message()
}

// removeDynamicExtensionsFromProto rewrites *FileDescriptorProto-tree
// options so that custom options known to the linked-in Go runtime
// surface as concrete generated types (e.g. *tableaupb.UnionOptions)
// instead of dynamicpb messages.
//
// Refer:
//
//	https://github.com/jhump/protoreflect/blob/v1.17.0/desc/protoparse/parser.go#L724
func removeDynamicExtensionsFromProto(fd *descriptorpb.FileDescriptorProto) {
	fd.Options = removeDynamicExtensionsFromOptions(fd.Options)
	err := walk.DescriptorProtos(fd, func(_ protoreflect.FullName, msg proto.Message) error {
		switch msg := msg.(type) {
		case *descriptorpb.DescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
			for _, extr := range msg.ExtensionRange {
				extr.Options = removeDynamicExtensionsFromOptions(extr.Options)
			}
		case *descriptorpb.FieldDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.OneofDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.EnumValueDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.ServiceDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		case *descriptorpb.MethodDescriptorProto:
			msg.Options = removeDynamicExtensionsFromOptions(msg.Options)
		}
		return nil
	})
	if err != nil {
		log.Warnf("walk descriptor protos failed: %v", err)
	}
}

func removeDynamicExtensionsFromOptions[O proto.Message](opts O) O {
	removeOne(opts.ProtoReflect())
	return opts
}

func removeOne(opts protoreflect.Message) {
	dynamicOpts := opts.Type().New()
	opts.Range(func(fd protoreflect.FieldDescriptor, val protoreflect.Value) bool {
		if fd.IsExtension() {
			dynamicOpts.Set(fd, val)
			opts.Clear(fd)
		}
		return true
	})
	// serialize only these custom options
	data, err := proto.MarshalOptions{AllowPartial: true}.Marshal(dynamicOpts.Interface())
	if err != nil {
		// oh, well... can't fix this one
		log.Warnf("marshal dynamic options failed: %v", err)
		return
	}
	// and then replace values by clearing these custom options and
	// deserializing.
	err = proto.UnmarshalOptions{AllowPartial: true, Merge: true}.Unmarshal(data, opts.Interface())
	if err != nil {
		// oh, well... can't fix this one
		log.Warnf("unmarshal dynamic options failed: %v", err)
		return
	}
}
