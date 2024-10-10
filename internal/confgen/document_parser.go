package confgen

import (
	"strconv"
	"strings"

	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type documentParser struct {
	parser *sheetParser
}

func (sp *documentParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	if len(sheet.Document.Children) != 1 {
		return xerrors.ErrorKV("document should have and only have one child (map node)",
			xerrors.KeySheetName, sheet.Name)
	}
	// get the first child (map node) in document
	child := sheet.Document.Children[0]
	msg := protomsg.ProtoReflect()
	_, err := sp.parseMessage(msg, child)
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeySheetName, sheet.Name)
	}
	return nil
}

// parseMessage parses all fields of a protobuf message.
func (sp *documentParser) parseMessage(msg protoreflect.Message, node *book.Node) (present bool, err error) {
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			field := parseFieldDescriptor(fd, sp.parser.opts.Sep, sp.parser.opts.Subsep)
			defer field.release()
			var fieldNode *book.Node
			if md.FullName() == "tableau.Metabook" {
				// NOTE: this is a workaround specially for parsing metabook.
				//
				// just treat self node (with meta child removed) as field node
				// if option Name is empty
				fieldNode = &book.Node{
					Kind:     node.Kind,
					Name:     node.Name,
					Value:    node.Value,
					Children: node.GetChildrenWithoutMeta(),
					NamePos:  node.NamePos,
					ValuePos: node.ValuePos,
				}
			} else {
				fieldNode = node.FindChild(field.opts.Name)
				if fieldNode == nil && xproto.GetFieldDefaultValue(fd) != "" {
					// if this field has a default value, use virtual node
					fieldNode = &book.Node{
						Name:  node.Name,
						Value: node.Value,
					}
				}
				if fieldNode == nil {
					if sp.parser.IsFieldOptional(field) {
						// field not found and is optional, just return nil.
						return nil
					}
					kvs := node.DebugNameKV()
					kvs = append(kvs,
						xerrors.KeyPBFieldType, xproto.GetFieldTypeName(fd),
						xerrors.KeyPBFieldName, fd.FullName(),
						xerrors.KeyPBFieldOpts, field.opts,
					)
					return xerrors.WrapKV(xerrors.E2014(field.opts.Name), kvs...)
				}
			}
			fieldPresent, err := sp.parseField(field, msg, fieldNode)
			if err != nil {
				return xerrors.WrapKV(err,
					xerrors.KeyPBFieldType, xproto.GetFieldTypeName(fd),
					xerrors.KeyPBFieldName, fd.FullName(),
					xerrors.KeyPBFieldOpts, field.opts)
			}
			if fieldPresent {
				// The message is treated as present at least one field is present.
				present = true
			}
			return nil
		}()
		if err != nil {
			return false, err
		}
	}
	return present, nil
}

func (sp *documentParser) parseField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	// log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return sp.parseMapField(field, msg, node)
	} else if field.fd.IsList() {
		return sp.parseListField(field, msg, node)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		if xproto.IsUnionField(field.fd) {
			return sp.parseUnionField(field, msg, node)
		}
		return sp.parseStructField(field, msg, node)
	} else {
		return sp.parseScalarField(field, msg, node)
	}
}

func (sp *documentParser) parseMapField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	reflectMap := newValue.Map()
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()

	if field.opts.Layout == tableaupb.Layout_LAYOUT_INCELL {
		// incell map
		err = sp.parser.parseIncellMap(field, reflectMap, node.Value)
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
	} else if valueFd.Kind() != protoreflect.MessageKind ||
		field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// scalar map (key can be enum)
		err = sp.parseScalarMap(field, reflectMap, node)
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
	} else {
		if valueFd.Kind() == protoreflect.MessageKind {
			for _, elemNode := range node.Children {
				var keyData string
				switch node.Kind {
				case book.MapNode:
					keyData = elemNode.Name
					// auto add virtual key node
					keyNode := &book.Node{
						Kind:     book.ScalarNode,
						Name:     field.opts.Key,
						Value:    elemNode.Name,
						Children: nil,
						NamePos:  node.NamePos,
						ValuePos: node.ValuePos,
					}
					elemNode.Children = append(elemNode.Children, keyNode)
				case book.ListNode:
					keyNode := elemNode.FindChild(field.opts.Key)
					if keyNode == nil {
						return false, xerrors.WrapKV(xerrors.E2018(field.opts.Key), elemNode.DebugKV()...)
					}
					keyData = keyNode.Value
				default:
					return false, xerrors.ErrorKV("should not reach here", node.DebugKV()...)
				}
				newMapKey, keyPresent, err := sp.parser.parseMapKey(field, reflectMap, keyData)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
				var newMapValue protoreflect.Value
				if reflectMap.Has(newMapKey) {
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				valuePresent, err := sp.parseMessage(newMapValue.Message(), elemNode)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
				// TODO: auto remove added virtual key node?
				// check key uniqueness
				if reflectMap.Has(newMapKey) {
					if prop.RequireUnique(field.opts.Prop) ||
						(!prop.HasUnique(field.opts.Prop) && sp.parser.deduceMapKeyUnique(field, reflectMap)) {
						return false, xerrors.WrapKV(xerrors.E2005(elemNode.Name), elemNode.DebugKV()...)
					}
				}
				if !keyPresent && !valuePresent {
					// key and value are both not present.
					continue
				}
				reflectMap.Set(newMapKey, newMapValue)
			}
		} else {
			return false, xerrors.ErrorKV("should not reach here", node.DebugKV()...)
		}
	}

	if !msg.Has(field.fd) && reflectMap.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	if msg.Has(field.fd) || reflectMap.Len() != 0 {
		present = true
	}
	return present, nil
}

func (sp *documentParser) parseScalarMap(field *Field, reflectMap protoreflect.Map, node *book.Node) (err error) {
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	if valueFd.Kind() != protoreflect.MessageKind {
		return sp.parseScalarMapWithSimpleKV(field, reflectMap, node)
	}

	if !types.CheckMessageWithOnlyKVFields(valueFd.Message()) {
		return xerrors.Errorf("map value type is not KV struct, and is not supported")
	}
	return sp.parseScalarMapWithValueAsSimpleKVMessage(field, reflectMap, node)
}

// parseScalarMapWithSimpleKV parses simple incell map with key as scalar type and value as scalar or enum type.
// For example:
//   - map<int32, int32>
//   - map<int32, EnumType>
func (sp *documentParser) parseScalarMapWithSimpleKV(field *Field, reflectMap protoreflect.Map, node *book.Node) (err error) {
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()
	for _, elemNode := range node.Children {
		key, value := elemNode.Name, elemNode.Value

		fieldValue, keyPresent, err := sp.parser.parseFieldValue(keyFd, key, field.opts.Prop)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugNameKV()...)
		}

		newMapKey := fieldValue.MapKey()
		// Currently, we cannot check scalar map value, so do not input field.opts.Prop.
		fieldValue, valuePresent, err := sp.parser.parseFieldValue(valueFd, value, nil)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugKV()...)
		}
		newMapValue := fieldValue

		if !keyPresent && !valuePresent {
			continue
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return nil
}

// parseScalarMapWithValueAsSimpleKVMessage parses simple incell map with key as scalar or enum type
// and value as simple message with only key and value fields. For example:
//
//	enum FruitType {
//		FRUIT_TYPE_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//		FRUIT_TYPE_APPLE   = 1 [(tableau.evalue).name = "Apple"];
//		FRUIT_TYPE_ORANGE  = 2 [(tableau.evalue).name = "Orange"];
//		FRUIT_TYPE_BANANA  = 3 [(tableau.evalue).name = "Banana"];
//	}
//	enum FruitFlavor {
//		FRUIT_FLAVOR_UNKNOWN = 0 [(tableau.evalue).name = "Unknown"];
//		FRUIT_FLAVOR_FRAGRANT = 1 [(tableau.evalue).name = "Fragrant"];
//		FRUIT_FLAVOR_SOUR = 2 [(tableau.evalue).name = "Sour"];
//		FRUIT_FLAVOR_SWEET = 3 [(tableau.evalue).name = "Sweet"];
//	}
//
//	map<int32, Fruit> fruit_map = 1 [(tableau.field) = {name:"Fruit" key:"Key" layout:LAYOUT_INCELL}];
//	message Fruit {
//		FruitType key = 1 [(tableau.field) = {name:"Key"}];
//		int64 value = 2 [(tableau.field) = {name:"Value"}];
//	}
//
//	map<int32, Item> item_map = 3 [(tableau.field) = {name:"Item" key:"Key" layout:LAYOUT_INCELL}];
//	message Item {
//		FruitType key = 1 [(tableau.field) = {name:"Key"}];
//		FruitFlavor value = 2 [(tableau.field) = {name:"Value"}];
//	}
func (sp *documentParser) parseScalarMapWithValueAsSimpleKVMessage(field *Field, reflectMap protoreflect.Map, node *book.Node) (err error) {
	// If s does not contain sep and sep is not empty, Split returns a
	// slice of length 1 whose only element is s.
	for _, elemNode := range node.Children {
		key, value := elemNode.Name, elemNode.Value
		mapItemData := strings.Join([]string{key, value}, field.opts.Subsep)
		newMapKey, keyPresent, err := sp.parser.parseMapKey(field, reflectMap, key)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugNameKV()...)
		}
		newMapValue := reflectMap.NewValue()
		valuePresent, err := sp.parser.parseIncellStruct(newMapValue, mapItemData, field.opts.GetProp().GetForm(), field.opts.Subsep)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugKV()...)
		}

		if !keyPresent && !valuePresent {
			continue
		}
		reflectMap.Set(newMapKey, newMapValue)
	}
	return nil
}

func (sp *documentParser) parseListField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	list := newValue.List()
	switch {
	case field.opts.Layout == tableaupb.Layout_LAYOUT_INCELL,
		// node of XML scalar list with only 1 element is just like an incell list
		node.Kind == book.ScalarNode:
		present, err = sp.parser.parseIncellListField(field, list, node.Value)
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
	default:
		for _, elemNode := range node.Children {
			elemPresent := false
			newListValue := list.NewElement()
			if xproto.IsUnionField(field.fd) {
				// cross-cell union list
				elemPresent, err = sp.parseUnionMessage(field, newListValue.Message(), elemNode)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
			} else if field.fd.Kind() == protoreflect.MessageKind {
				// cross-cell struct list
				elemPresent, err = sp.parseMessage(newListValue.Message(), elemNode)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
			} else {
				// cross-cell scalar list
				newListValue, elemPresent, err = sp.parser.parseFieldValue(field.fd, elemNode.Value, field.opts.Prop)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
			}
			if elemPresent {
				list.Append(newListValue)
			}
		}
	}

	if !msg.Has(field.fd) && list.Len() != 0 {
		msg.Set(field.fd, newValue)
	}
	if msg.Has(field.fd) || list.Len() != 0 {
		present = true
	}
	return present, nil
}

func (sp *documentParser) parseUnionField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		if present, err = sp.parser.parseIncellUnion(structValue, node.Value, field.opts.GetProp().GetForm()); err != nil {
			return false, xerrors.WrapKV(err, node.DebugNameKV()...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}

	if node.Kind == book.ListNode {
		if len(node.Children) != 1 {
			return false, xerrors.ErrorKV("list node of union must have and only have one child", node.DebugKV()...)
		}
		node = node.Children[0]
	}
	present, err = sp.parseUnionMessage(field, structValue.Message(), node)
	if err != nil {
		return false, xerrors.WrapKV(err, node.DebugKV()...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return present, nil
}

func (sp *documentParser) parseStructField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}
	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		if present, err = sp.parser.parseIncellStruct(structValue, node.Value, field.opts.GetProp().GetForm(), field.opts.Sep); err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
		if present {
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}
	// cross cell struct
	subMsgName := string(field.fd.Message().FullName())
	if types.IsWellKnownMessage(subMsgName) {
		// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
		value, present, err := sp.parser.parseFieldValue(field.fd, node.Value, field.opts.Prop)
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
		if present {
			msg.Set(field.fd, value)
		}
		return present, nil
	} else {
		if node.Kind == book.ListNode {
			if len(node.Children) != 1 {
				return false, xerrors.ErrorKV("list node of struct must have and only have one child", node.DebugKV()...)
			}
			node = node.Children[0]
		}
		present, err := sp.parseMessage(structValue.Message(), node)
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
		if present {
			// only set field if it is present.
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}
}

func (sp *documentParser) parseScalarField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	var newValue protoreflect.Value
	// FIXME(wenchy): treat any scalar field's present as true if this field's key exists?
	newValue, present, err = sp.parser.parseFieldValue(field.fd, node.Value, field.opts.Prop)
	if err != nil {
		return false, xerrors.WrapKV(err, node.DebugKV()...)
	}
	if !present {
		return false, nil
	}
	msg.Set(field.fd, newValue)
	return true, nil
}

func (sp *documentParser) parseUnionMessage(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeNodeName := unionDesc.TypeName()
	typeNode := node.FindChild(typeNodeName)
	if typeNode == nil && xproto.GetFieldDefaultValue(unionDesc.Type) != "" {
		// if this field has a default value, use virtual node
		typeNode = &book.Node{
			Name:  node.Name,
			Value: node.Value,
		}
	}
	if typeNode == nil {
		if sp.parser.IsFieldOptional(field) {
			// field not found and is optional, just return nil.
			return false, nil
		}
		kvs := node.DebugNameKV()
		kvs = append(kvs,
			xerrors.KeyPBFieldType, xproto.GetFieldTypeName(unionDesc.Type),
			xerrors.KeyPBFieldName, unionDesc.Type.FullName(),
			xerrors.KeyPBFieldOpts, field.opts,
		)
		return false, xerrors.WrapKV(xerrors.E2014(field.opts.Name), kvs...)
	}

	var typeVal protoreflect.Value
	typeVal, present, err = sp.parser.parseFieldValue(unionDesc.Type, typeNode.Value, nil)
	if err != nil {
		return false, xerrors.WrapKV(err, typeNode.DebugNameKV()...)
	}
	msg.Set(unionDesc.Type, typeVal)

	// parse value
	fieldNumber := int32(typeVal.Enum())
	if fieldNumber == 0 {
		// default enum value 0, no need to parse.
		return false, nil
	}
	valueFD := unionDesc.GetValueByNumber(fieldNumber)
	if valueFD == nil {
		typeValue := unionDesc.Type.Enum().Values().ByNumber(protoreflect.EnumNumber(fieldNumber)).Name()
		return false, xerrors.WrapKV(xerrors.E2010(typeValue, fieldNumber), node.DebugKV()...)
	}
	fieldValue := msg.NewField(valueFD)
	if valueFD.Kind() == protoreflect.MessageKind {
		// MUST be message type.
		md := valueFD.Message()
		msg := fieldValue.Message()
		for i := 0; i < md.Fields().Len(); i++ {
			fd := md.Fields().Get(i)
			valNodeName := unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
			err := func() error {
				subField := parseFieldDescriptor(fd, sp.parser.opts.Sep, sp.parser.opts.Subsep)
				defer subField.release()
				valNode := node.FindChild(valNodeName)
				if valNode == nil && xproto.GetFieldDefaultValue(fd) != "" {
					// if this field has a default value, use virtual node
					valNode = &book.Node{
						Name:  node.Name,
						Value: node.Value,
					}
				}
				if valNode == nil {
					if sp.parser.IsFieldOptional(subField) {
						// field not found and is optional, just return nil.
						return nil
					}
					kvs := node.DebugNameKV()
					kvs = append(kvs,
						xerrors.KeyPBFieldType, xproto.GetFieldTypeName(fd),
						xerrors.KeyPBFieldName, fd.FullName(),
						xerrors.KeyPBFieldOpts, subField.opts,
					)
					return xerrors.WrapKV(xerrors.E2014(subField.opts.Name), kvs...)
				}
				err = sp.parser.parseUnionMessageField(subField, msg, valNode.Value)
				if err != nil {
					return xerrors.WrapKV(err, valNode.DebugNameKV()...)
				}
				return nil
			}()
			if err != nil {
				return false, err
			}
		}
	} else {
		// scalar: not supported yet.
		return false, xerrors.Errorf("union value (oneof) as scalar type not supported: %s", valueFD.FullName())
	}
	msg.Set(valueFD, fieldValue)
	return present, nil
}
