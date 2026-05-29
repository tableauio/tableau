package confgen

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"buf.build/go/protovalidate"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/internal/x/xproto/protoc"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/store"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var fieldOptionsPool *sync.Pool

func init() {
	fieldOptionsPool = &sync.Pool{
		New: func() any {
			return new(tableaupb.FieldOptions)
		},
	}
}

type Field struct {
	fd protoreflect.FieldDescriptor
	// seq's value is dynamically merged at different priority levels:
	//  1. field-level: FieldProp.seq
	//  2. sheet-level: WorksheetOptions.seq
	//  3. global-level: options.ConfInputOption.Seq
	sep string
	// subseq's value is dynamically merged at different priority levels:
	//  1. field-level: FieldProp.seq
	//  2. sheet-level: WorksheetOptions.seq
	//  3. global-level: options.ConfInputOption.Subseq
	subsep string
	opts   *tableaupb.FieldOptions
}

// mergeParentFieldProp merges parent field's prop.
func (f *Field) mergeParentFieldProp(parent *Field) {
	if parent != nil && parent.opts != nil {
		if parent.opts.Prop.GetOptional() {
			if f.opts.Prop == nil {
				f.opts.Prop = &tableaupb.FieldProp{}
			}
			f.opts.Prop.Optional = true
		}
	}
}

// release returns back `opts` field to pool.
func (f *Field) release() {
	// return back to pool
	fieldOptionsPool.Put(f.opts)
}

// TODO: use sync.Map to cache *Field for reuse, e.g.: treat key as fd.FullName().
func (p *sheetParser) parseFieldDescriptor(fd protoreflect.FieldDescriptor) *Field {
	// default value
	name := strcase.FromContext(p.ctx).ToCamel(string(fd.FullName().Name()))
	note := ""
	span := tableaupb.Span_SPAN_DEFAULT
	key := ""
	layout := tableaupb.Layout_LAYOUT_DEFAULT
	var sep, subsep string
	var prop *tableaupb.FieldProp

	// opts := fd.Options().(*descriptorpb.FieldOptions)
	fieldOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	if fieldOpts != nil {
		name = fieldOpts.Name
		note = fieldOpts.Note
		span = fieldOpts.Span
		key = fieldOpts.Key
		layout = fieldOpts.Layout
		sep = fieldOpts.Prop.GetSep()
		subsep = fieldOpts.Prop.GetSubsep()
		prop = xproto.Clone(fieldOpts.Prop)
	} else {
		// default processing
		if fd.IsList() {
			// truncate suffix `List` (CamelCase) corresponding to `_list` (snake_case)
			name = strings.TrimSuffix(name, types.DefaultListFieldOptNameSuffix)
		} else if fd.IsMap() {
			// truncate suffix `Map` (CamelCase) corresponding to `_map` (snake_case)
			name = strings.TrimSuffix(name, types.DefaultMapFieldOptNameSuffix)
			key = types.DefaultMapKeyOptName
		}
	}
	if sep == "" {
		sep = p.GetSep()
	}
	if subsep == "" {
		subsep = p.GetSubsep()
	}

	// get from pool
	pooledOpts := fieldOptionsPool.Get().(*tableaupb.FieldOptions)
	pooledOpts.Name = name
	pooledOpts.Note = note
	pooledOpts.Key = key
	pooledOpts.Layout = layout
	pooledOpts.Span = span
	pooledOpts.Prop = prop

	return &Field{
		fd:     fd,
		sep:    sep,
		subsep: subsep,
		opts:   pooledOpts,
	}
}

// parseBookSpecifier parses the book specifier to book name and sheet name.
//
// bookSpecifier pattern: <Workbook>#<Worksheet>
//
// Examples:
//   - only workbook: excel/Item.xlsx
//   - with worksheet: excel/Item.xlsx#Item (To be implemented), NOTE: csv not supported
//     because it has special book name pattern.
//   - with special delimiter "#" in dir: excel#dir/Item.xlsx#Item
func parseBookSpecifier(bookSpecifier string) (bookName string, sheetName string, err error) {
	fmt := format.GetFormat(bookSpecifier)
	if fmt == format.CSV {
		// special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
		bookName, err := xfs.ParseCSVBooknamePatternFrom(bookSpecifier)
		if err != nil {
			return "", "", err
		}
		return bookName, "", nil
	}
	dir := filepath.Dir(bookSpecifier)
	baseBookSpecifier := filepath.Base(bookSpecifier)
	tokens := strings.SplitN(baseBookSpecifier, "#", 2)
	if len(tokens) == 2 {
		return xfs.Join(dir, tokens[0]), tokens[1], nil
	}
	return xfs.Join(dir, tokens[0]), "", nil
}

// bookIndex maps each workbook path (primary or secondary) to the proto
// FileDescriptors associated with it. Wrapping the map in a struct gives
// room to add fields (e.g. statistics, locks, derived caches) without
// changing the public surface used by callers.
type bookIndex struct {
	books map[string]*primaryBookInfo
}

// newBookIndex returns an empty bookIndex ready to record entries via add.
func newBookIndex() *bookIndex {
	return &bookIndex{books: map[string]*primaryBookInfo{}}
}

// add records fd under bookName. Duplicate fds (same fd.Path()) are skipped,
// so callers can safely add the same fd multiple times — this happens when
// a Merger/Scatter glob resolves back to an already-indexed workbook, which
// would otherwise cause GenWorkbook to run convert() twice for the same
// (book, fd) pair.
func (b *bookIndex) add(bookName string, fd protoreflect.FileDescriptor) {
	info := b.books[bookName]
	if info == nil {
		info = &primaryBookInfo{}
		b.books[bookName] = info
	}
	info.addFd(fd)
}

// get returns the primaryBookInfo recorded under bookName, or (nil, false)
// if no entry exists.
func (b *bookIndex) get(bookName string) (*primaryBookInfo, bool) {
	info, ok := b.books[bookName]
	return info, ok
}

// primaryBookInfo holds the FileDescriptors mapped to one workbook key in a bookIndex.
type primaryBookInfo struct {
	// fds can contain more than one descriptor because:
	//   - Merger/Scatter: several primary workbooks may reference the same workbook.
	//   - One workbook may be described by multiple proto files (e.g. lite + full variants).
	fds []protoreflect.FileDescriptor
}

// addFd appends fd to p.fds, skipping if an entry with the same fd.Path() already exists.
func (p *primaryBookInfo) addFd(fd protoreflect.FileDescriptor) {
	for _, existed := range p.fds {
		if existed.Path() == fd.Path() {
			return
		}
	}
	p.fds = append(p.fds, fd)
}

// buildWorkbookIndex builds all workbook names (includes primary and secondary) to primary workbook info indexes.
func buildWorkbookIndex(protoPackage, inputDir string, subdirs []string, subdirRewrites map[string]string, prFiles *protoregistry.Files) (bookIndexes *bookIndex, err error) {
	bookIndexes = newBookIndex()
	prFiles.RangeFilesByPackage(
		protoreflect.FullName(protoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			_, workbook := ParseFileOptions(fd)
			if workbook == nil {
				return true
			}
			// filter subdir
			if !xfs.HasSubdirPrefix(workbook.Name, subdirs) {
				return true
			}

			// add self: rewrite subdir
			rewrittenBookName := xfs.RewriteSubdir(workbook.Name, subdirRewrites)
			bookIndexes.add(rewrittenBookName, fd)
			// Merger/Scatter (only one can be set at once)
			msgs := fd.Messages()
			for i := 0; i < msgs.Len(); i++ {
				md := msgs.Get(i)
				opts := md.Options().(*descriptorpb.MessageOptions)
				if opts == nil {
					continue
				}
				sheetOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
				sheetSpecifiers := sheetOpts.GetMerger()
				if len(sheetSpecifiers) == 0 {
					sheetSpecifiers = sheetOpts.GetScatter()
				}
				for _, specifier := range sheetSpecifiers {
					relBookPaths, _, err1 := importer.ResolveSheetSpecifier(inputDir, workbook.Name, specifier, subdirRewrites)
					if err1 != nil {
						err = xerrors.WrapKV(err1, xerrors.KeyPrimarySheetName, sheetOpts.GetName())
						return false
					}
					for relBookPath := range relBookPaths {
						bookIndexes.add(relBookPath, fd)
					}
				}
			}
			return true
		})

	if err != nil {
		return nil, err
	}
	// debugging
	if log.LevelEnabled(zap.DebugLevel) {
		for k, v := range bookIndexes.books {
			for _, fd := range v.fds {
				_, workbook := ParseFileOptions(fd)
				log.Debugf("primary book index: %s -> %s (%s)", k, workbook.GetName(), fd.Path())
			}
		}
	}
	return bookIndexes, nil
}

func getRealSheetName(info *SheetInfo, impInfo importer.ImporterInfo) string {
	sheetName := info.SheetOpts.GetName()
	if impInfo.SpecifiedSheetName != "" {
		// sheet name is specified
		sheetName = impInfo.SpecifiedSheetName
	}
	return sheetName
}

func getRelBookName(basepath, filename string) string {
	if relBookName, err := xfs.Rel(basepath, filename); err != nil {
		log.Warnf("get relative path failed: %+v", err)
		return filename
	} else {
		return relBookName
	}
}

// loadProtoRegistryFiles auto loads all protoregistry.Files in protoregistry.GlobalFiles or parsed from proto files.
func loadProtoRegistryFiles(protoPackage string, protoPaths []string, protoFiles []string, excludeProtoFiles ...string) (*protoregistry.Files, error) {
	count := 0
	protoregistry.GlobalFiles.RangeFilesByPackage(
		protoreflect.FullName(protoPackage),
		func(fd protoreflect.FileDescriptor) bool {
			count++
			return false
		})
	if count != 0 {
		log.Debugf("use already injected protoregistry.GlobalFiles")
		return protoregistry.GlobalFiles, nil
	}
	return protoc.NewFiles(protoPaths, protoFiles, excludeProtoFiles...)
}

// parseOutputFormats parses the output formats of the specified message.
func parseOutputFormats(msg proto.Message, opt *options.ConfOutputOption) []format.Format {
	messagerName := string(msg.ProtoReflect().Descriptor().Name())
	if formats, ok := opt.MessagerFormats[messagerName]; ok && len(formats) != 0 {
		return formats
	}
	if formats := opt.Formats; len(formats) != 0 {
		return formats
	}
	return format.OutputFormats
}

// validate validates a proto message using the provided validator. Each violation
// is wrapped as an E2027 error and all violations are joined together.
func validate(msg proto.Message, validator protovalidate.Validator) error {
	err := validator.Validate(msg)
	if err == nil {
		return nil
	}
	var valErr *protovalidate.ValidationError
	if errors.As(err, &valErr) {
		errs := make([]error, 0, len(valErr.Violations))
		for _, v := range valErr.Violations {
			fieldValue := v.FieldValue.String()
			if !v.FieldValue.IsValid() ||
				(v.FieldDescriptor != nil && (v.FieldDescriptor.IsList() || v.FieldDescriptor.IsMap())) {
				// FieldValue is not set for message-level constraints, or is a
				// list/map whose String() returns a meaningless pointer address;
				// use field path as a fallback.
				fieldValue = protovalidate.FieldPathString(v.Proto.GetField())
			}
			errs = append(errs, xerrors.E2027(v.String(), fieldValue))
		}
		return errors.Join(errs...)
	}
	return xerrors.Wrap(err)
}

// storeMessage stores a message to one or multiple file formats.
func storeMessage(msg proto.Message, name, locationName, outputDir string, opt *options.ConfOutputOption, validator protovalidate.Validator) error {
	if err := validate(msg, validator); err != nil {
		return err
	}
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := parseOutputFormats(msg, opt)
	for _, fmt := range formats {
		err := store.Store(msg, outputDir, fmt,
			store.Name(name),
			store.LocationName(locationName),
			store.Pretty(opt.Pretty),
			store.EmitUnpopulated(opt.EmitUnpopulated),
			store.EmitTimezones(opt.EmitTimezones),
			store.UseProtoNames(opt.UseProtoNames),
			store.UseEnumNumbers(opt.UseEnumNumbers),
		)
		if err != nil {
			return xerrors.Wrap(err)
		}
	}
	return nil
}

// storePatchMergeMessage stores a patch merge message to one or multiple file
// formats. It will not emit unpopulated fields for clear reading.
func storePatchMergeMessage(msg proto.Message, name, locationName, outputDir string, opt *options.ConfOutputOption) error {
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := parseOutputFormats(msg, opt)
	for _, fmt := range formats {
		err := store.Store(msg, outputDir, fmt,
			store.Name(name),
			store.LocationName(locationName),
			store.Pretty(opt.Pretty),
			// store.EmitUnpopulated(opt.EmitUnpopulated), // DO NOT emit unpopulated fields for clear reading
			store.EmitTimezones(opt.EmitTimezones),
			store.UseProtoNames(opt.UseProtoNames),
			store.UseEnumNumbers(opt.UseEnumNumbers),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// parseTableMapLayout parses the layout of a map in table.
// Map default layout is vertical in table.
func parseTableMapLayout(layout tableaupb.Layout) tableaupb.Layout {
	if layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// map default layout is vertical
		layout = tableaupb.Layout_LAYOUT_VERTICAL
	}
	return layout
}

// parseTableListLayout parses the layout of a list in table.
// List default layout is horizontal in table.
func parseTableListLayout(layout tableaupb.Layout) tableaupb.Layout {
	if layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// list default layout is horizontal
		layout = tableaupb.Layout_LAYOUT_HORIZONTAL
	}
	return layout
}

// isSafePathKeyChar returns true if the input character is safe for not
// needing escaping.
func isSafePathKeyChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c <= ' ' || c > '~' || c == '_' ||
		c == '-' || c == ':'
}

// refer: https://github.com/tidwall/gjson/blob/v1.18.0/gjson.go#L3560
func escapeMapKey(key protoreflect.Value) string {
	comp := fmt.Sprint(key)
	for i := 0; i < len(comp); i++ {
		if !isSafePathKeyChar(comp[i]) {
			ncomp := make([]byte, len(comp)+1)
			copy(ncomp, comp[:i])
			ncomp = ncomp[:i]
			for ; i < len(comp); i++ {
				if !isSafePathKeyChar(comp[i]) {
					ncomp = append(ncomp, '\\')
				}
				ncomp = append(ncomp, comp[i])
			}
			return string(ncomp)
		}
	}
	return comp
}

// listValues flattens a protoreflect.List into []any for E2023 diagnostics.
func listValues(list protoreflect.List) []any {
	values := make([]any, 0, list.Len())
	for i := 0; i < list.Len(); i++ {
		values = append(values, list.Get(i).Interface())
	}
	return values
}

// mapValues flattens a protoreflect.Map into map[any]any for E2023 diagnostics.
func mapValues(mapValue protoreflect.Map) map[any]any {
	values := make(map[any]any)
	mapValue.Range(func(k protoreflect.MapKey, v protoreflect.Value) bool {
		values[k.Interface()] = v.Interface()
		return true
	})
	return values
}

// findKeyFieldDescriptor returns the sub-field descriptor of msgFd whose
// tableau field option `name` (or default CamelCase fallback) equals keyName.
// It is used to locate the key sub-field of a message keyed-list element.
// Returns nil if msgFd is not a message kind or no matching sub-field is found.
func findKeyFieldDescriptor(ctx context.Context, msgFd protoreflect.FieldDescriptor, keyName string) protoreflect.FieldDescriptor {
	if keyName == "" || msgFd.Kind() != protoreflect.MessageKind {
		return nil
	}
	fields := msgFd.Message().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		name := strcase.FromContext(ctx).ToCamel(string(fd.FullName().Name()))
		if opts, ok := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions); ok && opts != nil && opts.Name != "" {
			name = opts.Name
		}
		if name == keyName {
			return fd
		}
	}
	return nil
}
