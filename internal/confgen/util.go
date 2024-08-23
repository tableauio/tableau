package confgen

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/fs"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
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
	fd   protoreflect.FieldDescriptor
	opts *tableaupb.FieldOptions
}

// release returns back `opts` field to pool.
func (f *Field) release() {
	// return back to pool
	fieldOptionsPool.Put(f.opts)
}

func parseFieldDescriptor(fd protoreflect.FieldDescriptor, sheetSep, sheetSubsep string) *Field {
	// default value
	name := strcase.ToCamel(string(fd.FullName().Name()))
	note := ""
	span := tableaupb.Span_SPAN_DEFAULT
	key := ""
	layout := tableaupb.Layout_LAYOUT_DEFAULT
	sep := ""
	subsep := ""
	var prop *tableaupb.FieldProp

	// opts := fd.Options().(*descriptorpb.FieldOptions)
	fieldOpts := proto.GetExtension(fd.Options(), tableaupb.E_Field).(*tableaupb.FieldOptions)
	if fieldOpts != nil {
		name = fieldOpts.Name
		note = fieldOpts.Note
		span = fieldOpts.Span
		key = fieldOpts.Key
		layout = fieldOpts.Layout
		sep = strings.TrimSpace(fieldOpts.Sep)
		subsep = strings.TrimSpace(fieldOpts.Subsep)
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
		sep = strings.TrimSpace(sheetSep)
		if sep == "" {
			sep = ","
		}
	}
	if subsep == "" {
		subsep = strings.TrimSpace(sheetSubsep)
		if subsep == "" {
			subsep = ":"
		}
	}

	// get from pool
	pooledOpts := fieldOptionsPool.Get().(*tableaupb.FieldOptions)
	pooledOpts.Name = name
	pooledOpts.Note = note
	pooledOpts.Span = span
	pooledOpts.Key = key
	pooledOpts.Layout = layout
	pooledOpts.Sep = sep
	pooledOpts.Subsep = subsep
	pooledOpts.Prop = prop

	return &Field{
		fd:   fd,
		opts: pooledOpts,
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
		bookName, err := fs.ParseCSVBooknamePatternFrom(bookSpecifier)
		if err != nil {
			return "", "", err
		}
		return bookName, "", nil
	}
	dir := filepath.Dir(bookSpecifier)
	baseBookSpecifier := filepath.Base(bookSpecifier)
	tokens := strings.SplitN(baseBookSpecifier, "#", 2)
	if len(tokens) == 2 {
		return fs.Join(dir, tokens[0]), tokens[1], nil
	}
	return fs.Join(dir, tokens[0]), "", nil
}

type bookIndexInfo struct {
	books map[string]protoreflect.FileDescriptor // primary book name -> fd
}

// buildWorkbookIndex builds the secondary workbook name (including self) -> primary workbook info indexes.
func buildWorkbookIndex(protoPackage protoreflect.FullName, inputDir string, subdirRewrites map[string]string, prFiles *protoregistry.Files) (bookIndexes map[string]*bookIndexInfo, err error) {
	bookIndexes = map[string]*bookIndexInfo{} // init
	prFiles.RangeFilesByPackage(
		protoPackage,
		func(fd protoreflect.FileDescriptor) bool {
			_, workbook := ParseFileOptions(fd)
			if workbook == nil {
				return true
			}
			// add self: rewrite subdir
			rewrittenWorkbookName := fs.RewriteSubdir(workbook.Name, subdirRewrites)
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
						err = xerrors.WithMessageKV(err1, xerrors.KeyPrimarySheetName, sheetOpts.GetName())
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
	sheetName := info.Opts.GetName()
	if impInfo.SpecifiedSheetName != "" {
		// sheet name is specified
		sheetName = impInfo.SpecifiedSheetName
	}
	return sheetName
}

func getRelBookName(basepath, filename string) string {
	if relBookName, err := fs.Rel(basepath, filename); err != nil {
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
func storeMessage(msg proto.Message, name string, outputDir string, opt *options.ConfOutputOption) error {
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := format.OutputFormats
	if len(opt.Formats) != 0 {
		formats = opt.Formats
	}
	for _, fmt := range formats {
		err := store.Store(msg, outputDir, fmt,
			store.Name(name),
			store.Pretty(opt.Pretty),
			store.EmitUnpopulated(opt.EmitUnpopulated),
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
func storePatchMergeMessage(msg proto.Message, name string, outputDir string, opt *options.ConfOutputOption) error {
	outputDir = filepath.Join(outputDir, opt.Subdir)
	formats := format.OutputFormats
	if len(opt.Formats) != 0 {
		formats = opt.Formats
	}
	for _, fmt := range formats {
		err := store.Store(msg, outputDir, fmt,
			store.Name(name),
			store.Pretty(opt.Pretty),
			// store.EmitUnpopulated(opt.EmitUnpopulated), // DO NOT emit unpopulated fields for clear reading
			store.UseProtoNames(opt.UseProtoNames),
			store.UseEnumNumbers(opt.UseEnumNumbers),
		)
		if err != nil {
			return err
		}
	}
	return nil
}
