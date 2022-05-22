package confgen

import (
	"strconv"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/internal/confgen/mexporter"
	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type sheetExporter struct {
	OutputDir string
	Output    *options.OutputOption // output settings.

}

func NewSheetExporter(outputDir string, output *options.OutputOption) *sheetExporter {
	return &sheetExporter{
		OutputDir: outputDir,
		Output:    output,
	}
}

// export the protomsg message.
func (x *sheetExporter) Export(parser *sheetParser, protomsg proto.Message, importers ...importer.Importer) error {
	md := protomsg.ProtoReflect().Descriptor()
	msgName, wsOpts := ParseMessageOptions(md)

	for _, imp := range importers {
		sheet := imp.GetSheet(wsOpts.Name)
		if sheet == nil {
			return errors.Errorf("sheet %s not found", wsOpts.Name)
		}

		if err := parser.Parse(protomsg, sheet); err != nil {
			return errors.WithMessage(err, "failed to parse sheet")
		}
	}

	exporter := mexporter.New(msgName, protomsg, x.OutputDir, x.Output, wsOpts)
	return exporter.Export()
}

type sheetParser struct {
	ProtoPackage string
	LocationName string
	opts         *tableaupb.WorksheetOptions
}

func NewSheetParser(protoPackage, locationName string, opts *tableaupb.WorksheetOptions) *sheetParser {
	return &sheetParser{
		ProtoPackage: protoPackage,
		LocationName: locationName,
		opts:         opts,
	}
}

func (sp *sheetParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// atom.Log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	if sp.opts.Transpose {
		// interchange the rows and columns
		// namerow: name column
		// [datarow, MaxCol]: data column
		// kvRow := make(map[string]string)
		var prev *book.RowCells
		for col := int(sp.opts.Datarow) - 1; col < sheet.MaxCol; col++ {
			curr := book.NewRowCells(col, prev)
			for row := 0; row < sheet.MaxRow; row++ {
				nameCol := int(sp.opts.Namerow) - 1
				nameCell, err := sheet.Cell(row, nameCol)
				if err != nil {
					return errors.WithMessagef(err, "failed to get name cell: %d, %d", row, nameCol)
				}
				name := book.ExtractFromCell(nameCell, sp.opts.Nameline)

				typ := ""
				if sp.opts.Typerow > 0 {
					// if typerow is set!
					typeCol := int(sp.opts.Typerow) - 1
					typeCell, err := sheet.Cell(row, typeCol)
					if err != nil {
						return errors.WithMessagef(err, "failed to get name cell: %d, %d", row, typeCol)
					}
					typ = book.ExtractFromCell(typeCell, sp.opts.Typeline)
				}

				data, err := sheet.Cell(row, col)
				if err != nil {
					return errors.WithMessagef(err, "failed to get data cell: %d, %d", row, col)
				}
				curr.SetCell(name, row, data, typ, sp.opts.AdjacentKey)
			}
			err := sp.parseFieldOptions(msg, curr, 0, "")
			if err != nil {
				return err
			}
			prev = curr
		}
	} else {
		// namerow: name row
		// [datarow, MaxRow]: data row
		var prev *book.RowCells
		for row := int(sp.opts.Datarow) - 1; row < sheet.MaxRow; row++ {
			curr := book.NewRowCells(row, prev)
			for col := 0; col < sheet.MaxCol; col++ {
				nameRow := int(sp.opts.Namerow) - 1
				nameCell, err := sheet.Cell(nameRow, col)
				if err != nil {
					return errors.WithMessagef(err, "failed to get name cell: %d, %d", nameRow, col)
				}
				name := book.ExtractFromCell(nameCell, sp.opts.Nameline)

				typ := ""
				if sp.opts.Typerow > 0 {
					// if typerow is set!
					typeRow := int(sp.opts.Typerow) - 1
					typeCell, err := sheet.Cell(typeRow, col)
					if err != nil {
						return errors.WithMessagef(err, "failed to get type cell: %d, %d", typeRow, col)
					}
					typ = book.ExtractFromCell(typeCell, sp.opts.Typeline)
				}

				data, err := sheet.Cell(row, col)
				if err != nil {
					return errors.WithMessagef(err, "failed to get data cell: %d, %d", row, col)
				}
				curr.SetCell(name, col, data, typ, sp.opts.AdjacentKey)
			}
			err := sp.parseFieldOptions(msg, curr, 0, "")
			if err != nil {
				return err
			}
			prev = curr
		}
	}
	return nil
}

type Field struct {
	fd   protoreflect.FieldDescriptor
	opts *tableaupb.FieldOptions
}

// parseFieldOptions is aimed to parse the options of all the fields of a protobuf message.
func (sp *sheetParser) parseFieldOptions(msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (err error) {
	md := msg.Descriptor()
	pkg := md.ParentFile().Package()
	// opts := md.Options().(*descriptorpb.MessageOptions)
	// worksheet := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	// worksheetName := ""
	// if worksheet != nil {
	// 	worksheetName = worksheet.Name
	// }
	// atom.Log.Debugf("%s// %s, '%s', %v, %v, %v", printer.Indent(depth), md.FullName(), worksheetName, md.IsMapEntry(), prefix, pkg)
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		if string(pkg) != sp.ProtoPackage && pkg != "google.protobuf" {
			atom.Log.Debugf("no need to process package: %v", pkg)
			return nil
		}

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

		opts := fd.Options().(*descriptorpb.FieldOptions)
		fieldOpts := proto.GetExtension(opts, tableaupb.E_Field).(*tableaupb.FieldOptions)
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
				name = strings.TrimSuffix(name, "List")
			} else if fd.IsMap() {
				// truncate suffix `Map` (CamelCase) corresponding to `_map` (snake_case)
				// name = strings.TrimSuffix(name, "Map")
				name = ""
				key = "Key"
			}
		}
		if sep == "" {
			sep = strings.TrimSpace(sp.opts.Sep)
			if sep == "" {
				sep = ","
			}
		}
		if subsep == "" {
			subsep = strings.TrimSpace(sp.opts.Subsep)
			if subsep == "" {
				subsep = ":"
			}
		}

		field := &Field{
			fd: fd,
			opts: &tableaupb.FieldOptions{
				Name:     name,
				Note:     note,
				Span:     span,
				Key:      key,
				Layout:   layout,
				Sep:      sep,
				Subsep:   subsep,
				Optional: optional,
				Prop:     prop,
			},
		}
		err = sp.parseField(field, msg, rc, depth, prefix)
		if err != nil {
			return errors.WithMessagef(err, "failed to parse field: %s, opts: %v", fd.FullName().Name(), field.opts)
		}
	}
	return nil
}

func (sp *sheetParser) parseField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (err error) {
	// atom.Log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return sp.parseMapField(field, msg, rc, depth, prefix)
	} else if field.fd.IsList() {
		return sp.parseListField(field, msg, rc, depth, prefix)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		return sp.parseStructField(field, msg, rc, depth, prefix)
	} else {
		return sp.parseScalarField(field, msg, rc, depth, prefix)
	}
}

func (sp *sheetParser) parseMapField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectMap := newValue.Map()
	// reflectMap := msg.Mutable(field.fd).Map()
	keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()

	layout := field.opts.Layout
	if field.opts.Layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// Map default layout is vertical
		layout = tableaupb.Layout_LAYOUT_VERTICAL
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		emptyMapValue := reflectMap.NewValue()
		if valueFd.Kind() == protoreflect.MessageKind {
			keyColName := prefix + field.opts.Name + field.opts.Key
			cell := rc.Cell(keyColName, field.opts.Optional)
			if cell == nil {
				return errors.Errorf("%s|vertical map: key column not found", rc.CellDebugString(keyColName))
			}
			newMapKey, err := sp.parseMapKey(field.opts, reflectMap, cell.Data)
			if err != nil {
				return errors.WithMessagef(err, "%s|vertical map: failed to parse key: %s", rc.CellDebugString(keyColName), cell.Data)
			}

			var newMapValue protoreflect.Value
			if reflectMap.Has(newMapKey) {
				// check uniqueness
				if prop.IsUnique(field.opts.Prop) {
					return errors.Errorf("%s|vertical map: key %s already exists", rc.CellDebugString(keyColName), cell.Data)
				}
				newMapValue = reflectMap.Mutable(newMapKey)
			} else {
				newMapValue = reflectMap.NewValue()
			}
			err = sp.parseFieldOptions(newMapValue.Message(), rc, depth+1, prefix+field.opts.Name)
			if err != nil {
				return errors.WithMessagef(err, "%s|vertical map: failed to parse field options with prefix: %s", rc.CellDebugString(keyColName), prefix+field.opts.Name)
			}
			if !types.EqualMessage(emptyMapValue, newMapValue) {
				reflectMap.Set(newMapKey, newMapValue)
			}
		} else {
			// value is scalar type
			key := "Key"     // default key name
			value := "Value" // default value name
			// key cell
			keyColName := prefix + field.opts.Name + key
			cell := rc.Cell(keyColName, field.opts.Optional)
			if cell == nil {
				return errors.Errorf("%s|vertical map(scalar): key column not found", rc.CellDebugString(keyColName))
			}
			fieldValue, _, err := sp.parseFieldValue(keyFd, cell.Data)
			if err != nil {
				return errors.WithMessagef(err, "%s|failed to parse field value: %s", rc.CellDebugString(keyColName), cell.Data)
			}

			// check range
			if !prop.InRange(field.opts.Prop, keyFd, fieldValue) {
				return errors.Errorf("%s|vertical map(scalar): value %s out of range [%s]", rc.CellDebugString(keyColName), cell.Data, field.opts.Prop.Range)
			}

			newMapKey := fieldValue.MapKey()
			var newMapValue protoreflect.Value
			if reflectMap.Has(newMapKey) {
				// check uniqueness
				if prop.IsUnique(field.opts.Prop) {
					return errors.Errorf("%s|vertical map(scalar): key %s already exists", rc.CellDebugString(keyColName), cell.Data)
				}
				newMapValue = reflectMap.Mutable(newMapKey)
			} else {
				newMapValue = reflectMap.NewValue()
			}
			// value cell
			valueColName := prefix + field.opts.Name + value
			cell = rc.Cell(valueColName, field.opts.Optional)
			if cell == nil {
				return errors.Errorf("%s|vertical map(scalar): value colum not found", rc.CellDebugString(valueColName))
			}
			newMapValue, _, err = sp.parseFieldValue(field.fd, cell.Data)
			if err != nil {
				return errors.WithMessagef(err, "%s|vertical map(scalar): failed to parse field value: %s", rc.CellDebugString(valueColName), cell.Data)
			}
			if !reflectMap.Has(newMapKey) {
				reflectMap.Set(newMapKey, newMapValue)
			}
		}

	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		emptyMapValue := reflectMap.NewValue()
		if valueFd.Kind() == protoreflect.MessageKind {
			if msg.Has(field.fd) {
				// When the map's layout is horizontal, only parse field
				// if it is not already present. This means the first not
				// empty related row part (related to this map) is parsed.
				return nil
			}
			size := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
			// atom.Log.Debug("prefix size: ", size)
			for i := 1; i <= size; i++ {
				keyColName := prefix + field.opts.Name + strconv.Itoa(i) + field.opts.Key
				cell := rc.Cell(keyColName, field.opts.Optional)
				if cell == nil {
					return errors.Errorf("%s|horizontal map: key column not found", rc.CellDebugString(keyColName))
				}
				newMapKey, err := sp.parseMapKey(field.opts, reflectMap, cell.Data)
				if err != nil {
					return errors.WithMessagef(err, "%s|horizontal map: failed to parse key: %s", rc.CellDebugString(keyColName), cell.Data)
				}

				var newMapValue protoreflect.Value
				if reflectMap.Has(newMapKey) {
					// check uniqueness
					if prop.IsUnique(field.opts.Prop) {
						return errors.Errorf("%s|horizontal map: key %s already exists", rc.CellDebugString(keyColName), cell.Data)
					}
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				err = sp.parseFieldOptions(newMapValue.Message(), rc, depth+1, prefix+field.opts.Name+strconv.Itoa(i))
				if err != nil {
					return errors.WithMessagef(err, "%s|horizontal map: failed to parse field options with prefix: %s", rc.CellDebugString(keyColName), prefix+field.opts.Name+strconv.Itoa(i))
				}
				if !types.EqualMessage(emptyMapValue, newMapValue) {
					reflectMap.Set(newMapKey, newMapValue)
				}
			}
		}

	case tableaupb.Layout_LAYOUT_INCELL:
		colName := prefix + field.opts.Name
		cell := rc.Cell(colName, field.opts.Optional)
		if cell == nil {
			return errors.Errorf("%s|column not found", rc.CellDebugString(colName))
		}
		if valueFd.Kind() == protoreflect.MessageKind {
			return errors.Errorf("%s|incell map: message type not supported", rc.CellDebugString(colName))
		}

		if cell.Data != "" {
			// If s does not contain sep and sep is not empty, Split returns a
			// slice of length 1 whose only element is s.
			splits := strings.Split(cell.Data, field.opts.Sep)
			for _, pair := range splits {
				kv := strings.SplitN(pair, field.opts.Subsep, 2)
				if len(kv) == 1 {
					// If value is not set, then treated it as default empty string.
					kv = append(kv, "")
				}
				key, value := kv[0], kv[1]

				fieldValue, _, err := sp.parseFieldValue(keyFd, key)
				if err != nil {
					return errors.WithMessagef(err, "%s|incell map: failed to parse field value: %s", rc.CellDebugString(colName), key)
				}

				// check range: key
				if !prop.InRange(field.opts.Prop, keyFd, fieldValue) {
					return errors.Errorf("%s|incell map: %s out of range [%s]", rc.CellDebugString(colName), key, field.opts.Prop.Range)
				}

				newMapKey := fieldValue.MapKey()
				fieldValue, _, err = sp.parseFieldValue(valueFd, value)
				if err != nil {
					return errors.WithMessagef(err, "%s|incell map: failed to parse field value: %s", rc.CellDebugString(colName), value)
				}
				newMapValue := reflectMap.NewValue()
				newMapValue = fieldValue

				reflectMap.Set(newMapKey, newMapValue)
			}
		}
	}

	if !msg.Has(field.fd) && reflectMap.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	return nil
}

func (sp *sheetParser) parseMapKey(opts *tableaupb.FieldOptions, reflectMap protoreflect.Map, cellData string) (protoreflect.MapKey, error) {
	var mapKey protoreflect.MapKey
	var keyFd protoreflect.FieldDescriptor

	md := reflectMap.NewValue().Message().Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		fdOpts := fd.Options().(*descriptorpb.FieldOptions)
		if fdOpts != nil {
			tableauFieldOpts := proto.GetExtension(fdOpts, tableaupb.E_Field).(*tableaupb.FieldOptions)
			if tableauFieldOpts != nil && tableauFieldOpts.Name == opts.Key {
				keyFd = fd
				break
			}
		}
	}
	if keyFd == nil {
		return mapKey, errors.Errorf("opts.Key %s not found in proto definition", opts.Key)
	}

	if keyFd.Kind() == protoreflect.EnumKind {
		fieldValue, _, err := sp.parseFieldValue(keyFd, cellData)
		if err != nil {
			return mapKey, errors.Errorf("failed to parse key: %s", cellData)
		}
		v := protoreflect.ValueOfInt32(int32(fieldValue.Enum()))
		mapKey = v.MapKey()
	} else {
		fieldValue, _, err := sp.parseFieldValue(keyFd, cellData)
		if err != nil {
			return mapKey, errors.WithMessagef(err, "failed to parse key: %s", cellData)
		}
		// check range: key
		if !prop.InRange(opts.Prop, keyFd, fieldValue) {
			return mapKey, errors.Errorf("%s out of range [%s]", cellData, opts.Prop.Range)
		}
		mapKey = fieldValue.MapKey()
	}
	return mapKey, nil
}

func (sp *sheetParser) parseListField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) (err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectList := newValue.List()

	layout := field.opts.Layout
	if field.opts.Layout == tableaupb.Layout_LAYOUT_DEFAULT {
		// List default layout is horizontal
		layout = tableaupb.Layout_LAYOUT_HORIZONTAL
	}

	switch layout {
	case tableaupb.Layout_LAYOUT_VERTICAL:
		emptyListValue := reflectList.NewElement()
		// vertical list
		if field.fd.Kind() == protoreflect.MessageKind {
			// struct list
			if field.opts.Key != "" {
				// KeyedList means the list is keyed by the first field with tag number 1.
				listItemValue := reflectList.NewElement()
				keyedListItemExisted := false
				keyColName := prefix + field.opts.Name + field.opts.Key
				for i := 0; i < reflectList.Len(); i++ {
					item := reflectList.Get(i)
					md := item.Message().Descriptor()
					keyProtoName := protoreflect.Name(strcase.ToSnake(field.opts.Key))
					fd := md.Fields().ByName(keyProtoName)
					if fd == nil {
						return errors.Errorf("%s|vertical keyed list: key field not found in proto definition: %s", rc.CellDebugString(keyColName), keyProtoName)
					}
					cell := rc.Cell(keyColName, field.opts.Optional)
					if cell == nil {
						return errors.Errorf("%s|vertical keyed list: key column not found", rc.CellDebugString(keyColName))
					}
					key, _, err := sp.parseFieldValue(fd, cell.Data)
					if err != nil {
						return errors.Errorf("%s|vertical keyed list: failed to parse field value: %s", rc.CellDebugString(keyColName), cell.Data)
					}
					if types.EqualValue(fd, item.Message().Get(fd), key) {
						listItemValue = item
						keyedListItemExisted = true
						break
					}
				}
				err = sp.parseFieldOptions(listItemValue.Message(), rc, depth+1, prefix+field.opts.Name)
				if err != nil {
					return errors.WithMessagef(err, "...|vertical list: failed to parse field options with prefix: %s", prefix+field.opts.Name)
				}
				if !keyedListItemExisted && !types.EqualMessage(emptyListValue, listItemValue) {
					reflectList.Append(listItemValue)
					// if prefix+field.opts.Name == "ServerConfCondition" {
					// 	atom.Log.Debugf("append list item: %+v", listItemValue)
					// }
				}
			} else {
				newListValue := reflectList.NewElement()
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// incell-struct list
					colName := prefix + field.opts.Name
					cell := rc.Cell(colName, field.opts.Optional)
					if cell == nil {
						return errors.Errorf("%s|incell struct: column not found", rc.CellDebugString(colName))
					}
					if err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.Sep); err != nil {
						return errors.WithMessagef(err, "...|vertical list: failed to parse field options at column: %s", colName)
					}
				} else {
					err = sp.parseFieldOptions(newListValue.Message(), rc, depth+1, prefix+field.opts.Name)
					if err != nil {
						return errors.WithMessagef(err, "...|vertical list: failed to parse field options with prefix: %s", prefix+field.opts.Name)
					}
				}
				if !types.EqualMessage(emptyListValue, newListValue) {
					reflectList.Append(newListValue)
				}
			}
		} else {
			// TODO: support list of scalar type when layout is vertical?
			// NOTE(wenchyzhu): we don't support list of scalar type when layout is vertical
		}
	case tableaupb.Layout_LAYOUT_HORIZONTAL:
		emptyListValue := reflectList.NewElement()
		// horizontal list
		if msg.Has(field.fd) {
			// When the list's layout is horizontal, only parse field
			// if it is not already present. This means the first not
			// empty related row part (part related to this list) is parsed.
			return nil
		}
		size := rc.GetCellCountWithPrefix(prefix + field.opts.Name)
		if size <= 0 {
			return errors.Errorf("%s|horizontal list: no cell found with digit suffix", rc.CellDebugString(prefix+field.opts.Name))
		}
		for i := 1; i <= size; i++ {
			newListValue := reflectList.NewElement()
			if field.fd.Kind() == protoreflect.MessageKind {
				if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
					// incell-struct list
					colName := prefix + field.opts.Name + strconv.Itoa(i)
					cell := rc.Cell(colName, field.opts.Optional)
					if cell == nil {
						return errors.Errorf("%s|incell struct: column not found", rc.CellDebugString(colName))
					}
					if err = sp.parseIncellStruct(newListValue, cell.Data, field.opts.Sep); err != nil {
						return errors.WithMessagef(err, "...|horizontal list: failed to parse field options at column: %s", colName)
					}
				} else {
					// struct list
					err = sp.parseFieldOptions(newListValue.Message(), rc, depth+1, prefix+field.opts.Name+strconv.Itoa(i))
					if err != nil {
						return errors.WithMessagef(err, "...|horizontal list: failed to parse field options with prefix: %s", prefix+field.opts.Name+strconv.Itoa(i))
					}
				}
				if err != nil {
					return errors.WithMessagef(err, "...|horizontal list: failed to parse field options with prefix: %s", prefix+field.opts.Name+strconv.Itoa(i))
				}
				if !types.EqualMessage(emptyListValue, newListValue) {
					reflectList.Append(newListValue)
				}
			} else {
				// scalar list
				colName := prefix + field.opts.Name + strconv.Itoa(i)
				cell := rc.Cell(colName, field.opts.Optional)
				if cell == nil {
					return errors.Errorf("%s|horizontal list(scalar): column not found", rc.CellDebugString(colName))
				}
				newListValue, _, err = sp.parseFieldValue(field.fd, cell.Data)
				if err != nil {
					return errors.WithMessagef(err, "%s|horizontal list(scalar): failed to parse field value: %s", rc.CellDebugString(colName), cell.Data)
				}
				reflectList.Append(newListValue)
			}
		}
	case tableaupb.Layout_LAYOUT_INCELL:
		// incell list
		colName := prefix + field.opts.Name
		cell := rc.Cell(colName, field.opts.Optional)
		if cell == nil {
			return errors.Errorf("%s|incell list: column not found", rc.CellDebugString(colName))
		}
		if cell.Data != "" {
			// If s does not contain sep and sep is not empty, Split returns a
			// slice of length 1 whose only element is s.
			splits := strings.Split(cell.Data, field.opts.Sep)
			for _, incell := range splits {
				fieldValue, _, err := sp.parseFieldValue(field.fd, incell)
				if err != nil {
					return errors.WithMessagef(err, "%s|incell list: failed to parse field value: %s", rc.CellDebugString(colName), incell)
				}
				// check range
				if !prop.InRange(field.opts.Prop, field.fd, fieldValue) {
					return errors.Errorf("%s|incell list: value %s out of range [%s]", rc.CellDebugString(colName), incell, field.opts.Prop.Range)
				}
				reflectList.Append(fieldValue)
			}
		}
	}

	if !msg.Has(field.fd) && reflectList.Len() != 0 {
		msg.Set(field.fd, newValue)
	}

	return nil
}

func (sp *sheetParser) parseStructField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) error {
	// NOTE(wenchy): `proto.Equal` treats a nil message as not equal to an empty one.
	// doc: [Equal](https://pkg.go.dev/google.golang.org/protobuf/proto?tab=doc#Equal)
	// issue: [APIv2: protoreflect: consider Message nilness test](https://github.com/golang/protobuf/issues/966)
	// ```
	// nilMessage = (*MyMessage)(nil)
	// emptyMessage = new(MyMessage)
	//
	// Equal(nil, nil)                   // true
	// Equal(nil, nilMessage)            // false
	// Equal(nil, emptyMessage)          // false
	// Equal(nilMessage, nilMessage)     // true
	// Equal(nilMessage, emptyMessage)   // ??? false
	// Equal(emptyMessage, emptyMessage) // true
	// ```
	//
	// Case: `subMsg := msg.Mutable(fd).Message()`
	// `Message.Mutable` will allocate new "empty message", and is not equal to "nil"
	//
	// Solution:
	// 1. spawn two values: `emptyValue` and `structValue`
	// 2. set `structValue` back to field if `structValue` is not equal to `emptyValue`

	structValue := msg.NewField(field.fd)
	if msg.Has(field.fd) {
		// Get it if this field is populated.
		structValue = msg.Mutable(field.fd)
	}

	colName := prefix + field.opts.Name
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		cell := rc.Cell(colName, field.opts.Optional)
		if cell == nil {
			return errors.Errorf("%s|incell struct: column not found", rc.CellDebugString(colName))
		}
		if err := sp.parseIncellStruct(structValue, cell.Data, field.opts.Sep); err != nil {
			return errors.WithMessagef(err, "%s|incell struct: failed to parse field options with prefix: %s", rc.CellDebugString(colName), prefix+field.opts.Name)
		}
	} else {
		subMsgName := string(field.fd.Message().FullName())
		_, found := specialMessageMap[subMsgName]
		if found {
			// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
			cell := rc.Cell(colName, field.opts.Optional)
			if cell == nil {
				return errors.Errorf("%s|built-in type %s: column not found", rc.CellDebugString(colName), subMsgName)
			}
			value, present, err := sp.parseFieldValue(field.fd, cell.Data)
			if err != nil {
				return errors.WithMessagef(err, "%s|built-in type %s: failed to parse field value: %s", rc.CellDebugString(colName), subMsgName, cell.Data)
			}
			if !present {
				return nil
			}
			msg.Set(field.fd, value)
			return nil
		} else {
			pkgName := structValue.Message().Descriptor().ParentFile().Package()
			if string(pkgName) != sp.ProtoPackage {
				return errors.Errorf("%s|struct: unknown message %v in package %s", rc.CellDebugString(colName), subMsgName, pkgName)
			}
			err := sp.parseFieldOptions(structValue.Message(), rc, depth+1, prefix+field.opts.Name)
			if err != nil {
				return errors.WithMessagef(err, "%s|struct: failed to parse field options with prefix: %s", rc.CellDebugString(colName), prefix+field.opts.Name)
			}
		}
	}

	emptyValue := msg.NewField(field.fd)
	if !types.EqualMessage(emptyValue, structValue) {
		// only set field if it is not empty
		msg.Set(field.fd, structValue)
	}
	return nil
}

func (sp *sheetParser) parseIncellStruct(structValue protoreflect.Value, cellData, sep string) (err error) {
	if cellData == "" {
		return nil
	}
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	splits := strings.Split(cellData, sep)
	subMd := structValue.Message().Descriptor()
	for i := 0; i < subMd.Fields().Len() && i < len(splits); i++ {
		fd := subMd.Fields().Get(i)
		// atom.Log.Debugf("fd.FullName().Name(): ", fd.FullName().Name())
		incell := splits[i]
		value, present, err := sp.parseFieldValue(fd, incell)
		if err != nil {
			return errors.WithMessagef(err, "incell struct(%s): failed to parse field value: %s", cellData, incell)
		}
		if fd.HasPresence() && !present{
			continue
		}
		structValue.Message().Set(fd, value)
	}
	return nil
}

func (sp *sheetParser) parseScalarField(field *Field, msg protoreflect.Message, rc *book.RowCells, depth int, prefix string) error {
	if msg.Has(field.fd) {
		// Only parse if this field is not polulated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return nil
	}

	newValue := msg.NewField(field.fd)
	colName := prefix + field.opts.Name
	cell := rc.Cell(colName, field.opts.Optional)
	if cell == nil {
		return errors.Errorf("%s|scalar: column not found", rc.CellDebugString(colName))
	}
	newValue, present, err := sp.parseFieldValue(field.fd, cell.Data)
	if err != nil {
		return errors.WithMessagef(err, "%s|scalar: failed to parse field value: %s", rc.CellDebugString(colName), cell.Data)
	}
	// check range
	if !prop.InRange(field.opts.Prop, field.fd, newValue) {
		return errors.Errorf("%s|scalar: value %v out of range [%s]", rc.CellDebugString(colName), newValue, field.opts.Prop.Range)
	}
	if field.fd.HasPresence() && !present {
		return nil
	}
	msg.Set(field.fd, newValue)
	return nil
}

func (sp *sheetParser) parseFieldValue(fd protoreflect.FieldDescriptor, rawValue string) (v protoreflect.Value, present bool, err error) {
	return xproto.ParseFieldValue(fd, rawValue, sp.LocationName)
}

// ParseFileOptions parse the options of a protobuf definition file.
func ParseFileOptions(fd protoreflect.FileDescriptor) (string, *tableaupb.WorkbookOptions) {
	opts := fd.Options().(*descriptorpb.FileOptions)
	protofile := string(fd.FullName())
	workbook := proto.GetExtension(opts, tableaupb.E_Workbook).(*tableaupb.WorkbookOptions)
	return protofile, workbook
}

// ParseMessageOptions parse the options of a protobuf message.
func ParseMessageOptions(md protoreflect.MessageDescriptor) (string, *tableaupb.WorksheetOptions) {
	opts := md.Options().(*descriptorpb.MessageOptions)
	msgName := string(md.Name())
	wsOpts := proto.GetExtension(opts, tableaupb.E_Worksheet).(*tableaupb.WorksheetOptions)
	if wsOpts.Namerow == 0 {
		wsOpts.Namerow = 1 // default
	}
	if wsOpts.Typerow == 0 {
		wsOpts.Typerow = 2 // default
	}

	if wsOpts.Noterow == 0 {
		wsOpts.Noterow = 3 // default
	}

	if wsOpts.Datarow == 0 {
		wsOpts.Datarow = 4 // default
	}
	// atom.Log.Debugf("msg: %v, wsOpts: %+v", msgName, wsOpts)
	return msgName, wsOpts
}
