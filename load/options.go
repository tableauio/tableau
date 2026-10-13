package load

import (
	"io/fs"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/importer"
	"google.golang.org/protobuf/proto"
)

// BaseOptions is the common struct for both global-level and messager-level
// options.
type BaseOptions struct {
	// Location represents the collection of time offsets in use in
	// a geographical area.
	//
	// NOTE: only input formats(Excel, CSV, XML, YAML) are supported.
	//
	// If the name is "" or "UTC", LoadLocation returns UTC.
	// If the name is "Local", LoadLocation returns Local.
	//
	// Default: "Local".
	LocationName string

	// IgnoreUnknownFields maps to UnmarshalOptions.DiscardUnknown for each
	// output format (JSON / Text / Bin):
	//   - JSON: protojson.UnmarshalOptions.DiscardUnknown
	//   - Text: prototext.UnmarshalOptions.DiscardUnknown
	//   - Bin:  proto.UnmarshalOptions.DiscardUnknown
	//
	// For JSON and Text, unknown fields error by default; when true they are
	// ignored so loading succeeds. For Bin, unknown wire tags never cause
	// unmarshal failure; when true they are discarded instead of preserved
	// in the message's unknown-field set.
	//
	// Default: false.
	IgnoreUnknownFields *bool

	// PatchDirs specifies the directory paths for config patching.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchDirs []string

	// Mode specifies the loading mode for config patching.
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: ModeAll.
	Mode *LoadMode

	// FS is the filesystem used to open Path / PatchPaths and to expand
	// PatchPaths globs for output formats (JSON, Bin, Text).
	//
	// When nil, GetFS returns a host OS filesystem (cwd-relative names,
	// absolute paths and ".." allowed). When set, that fs.FS is used instead;
	// path acceptance follows the implementation (see io/fs.FS.Open).
	//
	// WithFileSystem and deprecated WithReadFunc both assign this field;
	// later Option calls overwrite earlier ones (last wins). There is no
	// separate ReadFunc field or FS-vs-ReadFunc priority.
	//
	// Default: nil (→ host OS FS via GetFS).
	FS fs.FS

	// SubdirRewrites rewrites subdir paths (relative to workbook name option
	// in .proto file).
	//
	// NOTE: only input formats (Excel, CSV, XML, YAML) are supported.
	//
	// Default: nil.
	SubdirRewrites map[string]string

	// MaxErrorsPerSheet caps how many errors loadOrigin aggregates while
	// loading a single messager (one sheet) before bailing out.
	//
	//   <=0 : fail-fast (1, current behavior).
	//    1  : fail-fast.
	//   >1  : aggregate up to N errors.
	//
	// NOTE: only input formats (Excel, CSV, XML, YAML) are supported.
	//
	// Default: 0 (=> 1, fail-fast).
	MaxErrorsPerSheet int

	// Shared by all messager options in the same load scope.
	referredCache *fieldprop.ReferredCache
	importerCache *importer.Cache
}

// MessagerOptions is the options struct for a messager.
type MessagerOptions struct {
	BaseOptions
	// Path specifies the main config file for this messager.
	//
	// Resolution against the load dir passed to Load*/LoadMessagerInDir:
	//   - empty:    load from dir/<Name>.<ext> (default)
	//   - relative: filepath.Join(dir, Path)
	//   - absolute: Path as-is (filepath.IsAbs)
	//
	// The resolved name is opened via GetFS(). Relative paths are preferred
	// when the same MessagerOptions is reused with different dirs
	// (e.g. comparing two config trees).
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: "".
	Path string

	// PatchPaths specifies one or multiple patch file paths or glob patterns.
	// Each entry follows the same empty/relative/absolute rules as Path
	// (empty entries are skipped; relative paths are joined with the load dir).
	//
	// Each entry is resolved against the load dir, then expanded with
	// fs.Glob on GetFS() (hostFS when FS is nil). Literal paths are valid
	// Glob patterns. Zero matches is an error.
	//
	// Prefer relative globs (e.g. "AiAccountPvpStatConf*") when the same options
	// are reused across dirs (CheckCompatibility).
	//
	// NOTE: only output formats (JSON, Bin, Text) are supported.
	//
	// Default: nil.
	PatchPaths []string
}

func (o *MessagerOptions) getReferredCache() *fieldprop.ReferredCache {
	if o == nil || o.referredCache == nil {
		return fieldprop.NewReferredCache()
	}
	return o.referredCache
}

// getImporterCache returns the cache shared by parsed options. A direct
// MessagerOptions value gets its own cache for the call.
func (o *MessagerOptions) getImporterCache() *importer.Cache {
	if o == nil || o.importerCache == nil {
		return importer.NewCache()
	}
	return o.importerCache
}

// GetLocationName returns the location name.
func (o *MessagerOptions) GetLocationName() string {
	if o == nil || o.LocationName == "" {
		return "Local"
	}
	return o.LocationName
}

// GetIgnoreUnknownFields returns whether unknown fields are discarded when
// unmarshaling JSON, Text, or Bin (see IgnoreUnknownFields).
func (o *MessagerOptions) GetIgnoreUnknownFields() bool {
	if o == nil || o.IgnoreUnknownFields == nil {
		return false
	}
	return *o.IgnoreUnknownFields
}

// GetPatchDirs returns the directory paths for config patching.
func (o *MessagerOptions) GetPatchDirs() []string {
	if o == nil {
		return nil
	}
	return o.PatchDirs
}

// GetMode returns the loading mode.
func (o *MessagerOptions) GetMode() LoadMode {
	if o == nil || o.Mode == nil {
		return ModeAll
	}
	return *o.Mode
}

// GetFS returns the filesystem for output-format IO.
// When o is nil or o.FS is nil, it returns the default host OS filesystem
// (see hostFS).
func (o *MessagerOptions) GetFS() fs.FS {
	if o == nil || o.FS == nil {
		return hostFS{}
	}
	return o.FS
}

// GetSubdirRewrites returns the subdir path mapping.
func (o *MessagerOptions) GetSubdirRewrites() map[string]string {
	if o == nil {
		return nil
	}
	return o.SubdirRewrites
}

// GetMaxErrorsPerSheet returns the configured per-sheet cap, or 1 (fail-fast)
// if unset.
func (o *MessagerOptions) GetMaxErrorsPerSheet() int {
	if o == nil || o.MaxErrorsPerSheet <= 0 {
		return 1
	}
	return o.MaxErrorsPerSheet
}

// GetPath returns the configured Path (not yet resolved against a load dir).
// Resolution happens at load time against the load dir.
func (o *MessagerOptions) GetPath() string {
	if o == nil {
		return ""
	}
	return o.Path
}

// GetPatchPaths returns the configured PatchPaths (not yet resolved against a
// load dir). Resolution and glob expansion happen at load time.
func (o *MessagerOptions) GetPatchPaths() []string {
	if o == nil {
		return nil
	}
	return o.PatchPaths
}

// Options contains global-level and messager-level options. MessagerOptions
// returned by ParseMessagerOptionsByName share importer and referred-data
// caches while the Options value remains in use.
type Options struct {
	BaseOptions
	// MessagerOptions maps each messager name to a MessageOptions.
	// If specified, then the messager will be parsed with the given options
	// directly.
	//
	// Default: empty.
	MessagerOptions map[string]*MessagerOptions
}

// ParseMessagerOptionsByName parses messager options with both global-level and
// messager-level options taken into consideration.
func (o *Options) ParseMessagerOptionsByName(name string) *MessagerOptions {
	var mopts MessagerOptions
	if o == nil {
		return &mopts
	}
	if opts := o.MessagerOptions[name]; opts != nil {
		mopts = *opts
	}
	if mopts.LocationName == "" {
		mopts.LocationName = o.LocationName
	}
	if mopts.IgnoreUnknownFields == nil {
		mopts.IgnoreUnknownFields = o.IgnoreUnknownFields
	}
	if mopts.PatchDirs == nil {
		mopts.PatchDirs = o.PatchDirs
	}
	if mopts.Mode == nil {
		mopts.Mode = o.Mode
	}
	if mopts.FS == nil {
		mopts.FS = o.FS
	}
	if mopts.SubdirRewrites == nil {
		mopts.SubdirRewrites = o.SubdirRewrites
	}
	if mopts.MaxErrorsPerSheet == 0 {
		mopts.MaxErrorsPerSheet = o.MaxErrorsPerSheet
	}
	mopts.referredCache = o.referredCache
	mopts.importerCache = o.importerCache
	return &mopts
}

type LoadMode int

const (
	ModeAll       LoadMode = iota // Load all related files
	ModeOnlyMain                  // Only load the main file
	ModeOnlyPatch                 // Only load the patch files
)

// ReadFunc reads a config file and returns its content.
//
// Deprecated: implement [fs.FS] (typically [fs.ReadFileFS]) and use
// [WithFileSystem] instead.
type ReadFunc func(name string) ([]byte, error)

// Option is the functional option type.
type Option func(*Options)

// ParseOptions parses functional options and merge them to default Options.
func ParseOptions(setters ...Option) *Options {
	// Default Options
	opts := &Options{}
	for _, setter := range setters {
		setter(opts)
	}
	opts.referredCache = fieldprop.NewReferredCache()
	opts.importerCache = importer.NewCache()
	return opts
}

// WithReadFunc sets a custom read func by wrapping it as an [fs.FS]
// (ReadFile → readFunc; Open/Glob → host OS).
//
// Deprecated: use [WithFileSystem].
//
// Overwrites any FS previously set by WithFileSystem or WithReadFunc.
func WithReadFunc(readFunc ReadFunc) Option {
	return func(opts *Options) {
		opts.FS = readFuncFS{read: readFunc}
	}
}

// WithFileSystem sets the filesystem used for output-format config IO.
// Path acceptance follows fsys. When omitted, GetFS uses the host OS FS.
//
// Overwrites any FS previously set by WithFileSystem or WithReadFunc.
func WithFileSystem(fsys fs.FS) Option {
	return func(opts *Options) {
		opts.FS = fsys
	}
}

// LocationName sets TZ location name for parsing datetime format.
func LocationName(name string) Option {
	return func(opts *Options) {
		opts.LocationName = name
	}
}

// IgnoreUnknownFields sets DiscardUnknown for JSON, Text, and Bin unmarshaling.
// See [MessagerOptions.IgnoreUnknownFields].
func IgnoreUnknownFields() Option {
	return func(opts *Options) {
		opts.IgnoreUnknownFields = proto.Bool(true)
	}
}

// SubdirRewrites rewrites subdir paths (relative to workbook name option
// in .proto file).
func SubdirRewrites(subdirRewrites map[string]string) Option {
	return func(opts *Options) {
		opts.SubdirRewrites = subdirRewrites
	}
}

// PatchDirs specifies the directory paths for config patching.
func PatchDirs(dirs ...string) Option {
	return func(opts *Options) {
		opts.PatchDirs = dirs
	}
}

// Mode specifies the loading mode for config patching.
//
// NOTE: only JSON, Bin, and Text formats are supported.
func Mode(mode LoadMode) Option {
	return func(opts *Options) {
		opts.Mode = &mode
	}
}

// WithMessagerOptions sets the messager options.
func WithMessagerOptions(options map[string]*MessagerOptions) Option {
	return func(opts *Options) {
		opts.MessagerOptions = options
	}
}

// MaxErrorsPerSheet caps how many errors loadOrigin aggregates while
// loading a single messager (one sheet) before bailing out.
//
//	<=0 : fail-fast (1, current behavior).
//	 1  : fail-fast.
//	>1  : aggregate up to n errors.
//
// NOTE: only input formats (Excel, CSV, XML, YAML) are supported.
func MaxErrorsPerSheet(n int) Option {
	return func(opts *Options) {
		opts.MaxErrorsPerSheet = n
	}
}
