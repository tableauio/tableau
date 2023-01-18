package confgen

import (
	"strings"
	"sync"

	"github.com/iancoleman/strcase"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
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
	sep := ","
	subsep := ":"
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
