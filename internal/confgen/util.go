package confgen

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xfs"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/store"
	"github.com/tableauio/tableau/xerrors"
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

// release returns back `opts` field to pool.
func (f *Field) release() {
	// return back to pool
	fieldOptionsPool.Put(f.opts)
}

// TODO: use sync.Map to cache *Field for reuse, e.g.: treat key as fd.FullName().
func (sp *sheetParser) parseFieldDescriptor(fd protoreflect.FieldDescriptor) *Field {
	// default value
	name := strcase.FromContext(sp.ctx).ToCamel(string(fd.FullName().Name()))
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
		prop = fieldOpts.Prop
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
		sep = sp.GetSep()
	}
	if subsep == "" {
		subsep = sp.GetSubsep()
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

type bookIndexInfo struct {
	books map[string]protoreflect.FileDescriptor // primary book name -> fd
}

// buildWorkbookIndex builds the secondary workbook name (including self) -> primary workbook info indexes.
func buildWorkbookIndex(protoPackage, inputDir string, subdirs []string, subdirRewrites map[string]string, prFiles *protoregistry.Files) (bookIndexes map[string]*bookIndexInfo, err error) {
	bookIndexes = map[string]*bookIndexInfo{} // init
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
			rewrittenWorkbookName := xfs.RewriteSubdir(workbook.Name, subdirRewrites)
			if bookIndexes[rewrittenWorkbookName] == nil {
				bookIndexes[rewrittenWorkbookName] = &bookIndexInfo{
					books: make(map[string]protoreflect.FileDescriptor),
				}
			}
			bookIndexes[rewrittenWorkbookName].books[workbook.Name] = fd
			// merger or scatter (only one can be set at once)
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
						if bookIndexes[relBookPath] == nil {
							bookIndexes[relBookPath] = &bookIndexInfo{
								books: make(map[string]protoreflect.FileDescriptor),
							}
						}
						bookIndexes[relBookPath].books[workbook.Name] = fd
					}
				}
			}
			return true
		})

	if err != nil {
		return nil, err
	}
	for k, v := range bookIndexes {
		for primaryBookName := range v.books {
			log.Debugf("primary book index: %s -> %s", k, primaryBookName)
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
	return xproto.NewFiles(protoPaths, protoFiles, excludeProtoFiles...)
}

// storeMessage stores a message to one or multiple file formats.
func storeMessage(msg proto.Message, name, locationName, outputDir string, opt *options.ConfOutputOption) error {
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := format.OutputFormats
	if len(opt.Formats) != 0 {
		formats = opt.Formats
	}
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
			return err
		}
	}
	return nil
}

// storePatchMergeMessage stores a patch merge message to one or multiple file
// formats. It will not emit unpopulated fields for clear reading.
func storePatchMergeMessage(msg proto.Message, name, locationName, outputDir string, opt *options.ConfOutputOption) error {
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := format.OutputFormats
	if len(opt.Formats) != 0 {
		formats = opt.Formats
	}
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
