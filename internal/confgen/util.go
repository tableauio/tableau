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
	"github.com/tableauio/tableau/log"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var fieldOptionsPool *sync.Pool

func init() {
	fieldOptionsPool = &sync.Pool{
		New: func() interface{} {
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
	optional := false
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
		optional = fieldOpts.Optional
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
	pooledOpts.Optional = optional
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
func parseBookSpecifier(bookSpecifier string) (bookName string, sheetName string, err error) {
	fmt := format.Ext2Format(filepath.Ext(bookSpecifier))
	if fmt == format.CSV {
		// special process for CSV filename pattern: "<BookName>#<SheetName>.csv"
		bookName, err := fs.ParseCSVBooknamePatternFrom(bookSpecifier)
		if err != nil {
			return "", "", err
		}
		return bookName, "", nil
	}
	tokens := strings.SplitN(bookSpecifier, "#", 2)
	if len(tokens) == 2 {
		return tokens[0], tokens[1], nil
	}
	return tokens[0], "", nil
}

type bookIndexInfo struct {
	primaryBookName string
	fd              protoreflect.FileDescriptor
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
			bookIndexes[rewrittenWorkbookName] = &bookIndexInfo{primaryBookName: workbook.Name, fd: fd}
			// merger or scatter (only one can be set at once)
			msgs := fd.Messages()
			for i := 0; i < msgs.Len(); i++ {
				md := msgs.Get(i)
				opts := md.Options().(*descriptorpb.MessageOptions)
				if opts == nil {
					continue
				}
				sheetOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
				bookNameGlobs := sheetOpts.GetMerger()
				if len(bookNameGlobs) == 0 {
					bookNameGlobs = sheetOpts.GetScatter()
				}
				relBookPaths, err1 := importer.ResolveBookPathPattern(inputDir, workbook.Name, bookNameGlobs, subdirRewrites)
				if err1 != nil {
					err = err1
					return false
				}
				for relBookPath := range relBookPaths {
					bookIndexes[relBookPath] = &bookIndexInfo{primaryBookName: workbook.Name, fd: fd}
				}
			}
			return true
		})

	if err != nil {
		return nil, err
	}
	for k, v := range bookIndexes {
		log.Debugf("primary book index: %s -> %s", k, v.primaryBookName)
	}
	return bookIndexes, nil
}
