package confgen

import (
	"strconv"
	"strings"

	"github.com/tableauio/tableau/internal/confgen/fieldprop"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/x/xproto"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type documentParser struct {
	*sheetParser
}

func (sp *documentParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	if len(sheet.Document.Children) != 1 {
		return xerrors.ErrorKV("document should have and only have one child (map node)",
			xerrors.KeySheetName, sheet.Name)
	}
	// get the first child (map node) in document
	child := sheet.Document.Children[0]
	msg := protomsg.ProtoReflect()
	_, err := sp.parseMessage(msg, child, "")
	if err != nil {
		return xerrors.WrapKV(err, xerrors.KeySheetName, sheet.Name)
	}
	return nil
}

// parseMessage parses all fields of a protobuf message.
func (sp *documentParser) parseMessage(msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			field := sp.parseFieldDescriptor(fd)
			defer field.release()
			var fieldNode *book.Node
			if md.FullName() == xproto.MetabookFullName {
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
					if sp.IsFieldOptional(field) {
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
			newCardPrefix := cardPrefix + "." + string(fd.Name())
			fieldPresent, err := sp.parseField(field, msg, fieldNode, newCardPrefix)
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

func (sp *documentParser) parseField(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	// log.Debug(field.fd.ContainingMessage().FullName())
	if field.fd.IsMap() {
		return sp.parseMapField(field, msg, node, cardPrefix)
	} else if field.fd.IsList() {
		return sp.parseListField(field, msg, node, cardPrefix)
	} else if field.fd.Kind() == protoreflect.MessageKind {
		if xproto.IsUnionField(field.fd) {
			return sp.parseUnionField(field, msg, node, cardPrefix)
		}
		return sp.parseStructField(field, msg, node, cardPrefix)
	} else {
		return sp.parseScalarField(field, msg, node)
	}
}

func (sp *documentParser) parseMapField(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	prefValue := msg.Mutable(field.fd)
	reflectMap := prefValue.Map()
	// keyFd := field.fd.MapKey()
	valueFd := field.fd.MapValue()

	if field.opts.Layout == tableaupb.Layout_LAYOUT_INCELL {
		// incell map
		err = sp.parseIncellMap(field, reflectMap, node.ScalarValue())
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
				newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, keyData)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
				var newMapValue protoreflect.Value
				newMapKeyExisted := reflectMap.Has(newMapKey)
				if newMapKeyExisted {
					// check map key unique
					if err := sp.checkMapKeyUnique(field, reflectMap, keyData); err != nil {
						return false, xerrors.WrapKV(err, node.DebugKV()...)
					}
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				newCardPrefix := cardPrefix + "." + escapeMapKey(newMapKey.Value())
				valuePresent, err := sp.parseMessage(newMapValue.Message(), elemNode, newCardPrefix)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
				// TODO: auto remove added virtual key node?
				if !keyPresent && !valuePresent {
					// key and value are both not present.
					continue
				}
				if !newMapKeyExisted {
					// check map value's sub-field unique
					dupName, err := sp.checkSubFieldUnique(field, cardPrefix, newMapValue)
					if err != nil {
						return false, xerrors.WrapKV(err, elemNode.FindChild(dupName).DebugKV()...)
					}
				}
				reflectMap.Set(newMapKey, newMapValue)
			}
		} else {
			return false, xerrors.ErrorKV("should not reach here", node.DebugKV()...)
		}
	}

	if !msg.Has(field.fd) && reflectMap.Len() != 0 {
		msg.Set(field.fd, prefValue)
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

		fieldValue, keyPresent, err := sp.parseFieldValue(keyFd, key, field.opts.Prop)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugNameKV()...)
		}

		newMapKey := fieldValue.MapKey()
		if reflectMap.Has(newMapKey) {
			// scalar map key must be unique
			return xerrors.WrapKV(xerrors.E2005(key))
		}
		// Currently, we cannot check scalar map value, so do not input field.opts.Prop.
		fieldValue, valuePresent, err := sp.parseFieldValue(valueFd, value, nil)
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
		mapItemData := strings.Join([]string{key, value}, field.subsep)
		newMapKey, keyPresent, err := sp.parseMapKey(field, reflectMap, key)
		if err != nil {
			return xerrors.WrapKV(err, elemNode.DebugNameKV()...)
		}
		if reflectMap.Has(newMapKey) {
			// scalar map key must be unique
			return xerrors.WrapKV(xerrors.E2005(key))
		}
		newMapValue := reflectMap.NewValue()
		valuePresent, err := sp.parseIncellStruct(newMapValue, mapItemData, field.opts.GetProp().GetForm(), field.subsep)
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

func (sp *documentParser) parseListField(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	list := newValue.List()
	switch {
	case field.opts.Layout == tableaupb.Layout_LAYOUT_INCELL,
		// node of XML scalar list with only 1 element is just like an incell list
		node.Kind == book.ScalarNode:
		present, err = sp.parseIncellList(field, list, cardPrefix, node.ScalarValue())
		if err != nil {
			return false, xerrors.WrapKV(err, node.DebugKV()...)
		}
	default:
		for _, elemNode := range node.Children {
			elemPresent := false
			elemValue := list.NewElement()
			newCardPrefix := cardPrefix + "." + strconv.Itoa(list.Len())
			if xproto.IsUnionField(field.fd) {
				// cross-cell union list
				elemPresent, err = sp.parseUnionMessage(field, elemValue.Message(), elemNode, newCardPrefix)
			} else if field.fd.Kind() == protoreflect.MessageKind {
				// cross-cell struct list
				if types.IsWellKnownMessage(field.fd.Message().FullName()) {
					elemValue, elemPresent, err = sp.parseFieldValue(field.fd, elemNode.Value, field.opts.Prop)
				} else {
					elemPresent, err = sp.parseMessage(elemValue.Message(), elemNode, newCardPrefix)
				}
			} else {
				// cross-cell scalar list
				elemValue, elemPresent, err = sp.parseFieldValue(field.fd, elemNode.Value, field.opts.Prop)
			}
			if err != nil {
				return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
			}
			if elemPresent {
				// check list elem's sub-field unique
				_, err := sp.checkSubFieldUnique(field, cardPrefix, elemValue)
				if err != nil {
					return false, xerrors.WrapKV(err, elemNode.DebugKV()...)
				}
				list.Append(elemValue)
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

func (sp *documentParser) parseUnionField(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		present, err = sp.parseIncellUnion(structValue, node.ScalarValue(), field.opts.GetProp().GetForm())
	} else {
		if node.Kind == book.ListNode {
			if len(node.Children) != 1 {
				return false, xerrors.ErrorKV("list node of union must have and only have one child", node.DebugKV()...)
			}
			node = node.Children[0]
		}
		present, err = sp.parseUnionMessage(field, structValue.Message(), node, cardPrefix)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, node.DebugKV()...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (sp *documentParser) parseStructField(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}

	if field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		// incell struct
		present, err = sp.parseIncellStruct(structValue, node.ScalarValue(), field.opts.GetProp().GetForm(), field.sep)
	} else if types.IsWellKnownMessage(field.fd.Message().FullName()) {
		structValue, present, err = sp.parseFieldValue(field.fd, node.ScalarValue(), field.opts.Prop)
	} else {
		// cross-cell struct
		if node.Kind == book.ListNode {
			if len(node.Children) != 1 {
				return false, xerrors.ErrorKV("list node of struct must have and only have one child", node.DebugKV()...)
			}
			node = node.Children[0]
		}
		present, err = sp.parseMessage(structValue.Message(), node, cardPrefix)
	}
	if err != nil {
		return false, xerrors.WrapKV(err, node.DebugKV()...)
	}
	if present {
		msg.Set(field.fd, structValue)
	}
	return
}

func (sp *documentParser) parseScalarField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	var newValue protoreflect.Value
	// FIXME(wenchy): treat any scalar field's present as true if this field's key exists?
	newValue, present, err = sp.parseFieldValue(field.fd, node.ScalarValue(), field.opts.Prop)
	if err != nil {
		return false, xerrors.WrapKV(err, node.DebugKV()...)
	}
	if present {
		msg.Set(field.fd, newValue)
	}
	return
}

func (sp *documentParser) parseUnionMessage(field *Field, msg protoreflect.Message, node *book.Node, cardPrefix string) (present bool, err error) {
	unionDesc := xproto.ExtractUnionDescriptor(field.fd.Message())
	if unionDesc == nil {
		return false, xerrors.Errorf("illegal definition of union: %s", field.fd.Message().FullName())
	}

	// parse union type
	typeNodeName := sp.strcaseCtx.ToCamel(unionDesc.TypeName())
	typeNode := node.FindChild(typeNodeName)
	if typeNode == nil && xproto.GetFieldDefaultValue(unionDesc.Type) != "" {
		// if this field has a default value, use virtual node
		typeNode = &book.Node{
			Name:  node.Name,
			Value: node.Value,
		}
	}
	if typeNode == nil {
		if sp.IsFieldOptional(field) {
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
	typeVal, present, err = sp.parseFieldValue(unionDesc.Type, typeNode.Value, nil)
	if err != nil {
		return false, xerrors.WrapKV(err, typeNode.DebugNameKV()...)
	}
	if !present {
		return false, nil
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
		// This enum value has not bound to a oneof field.
		return true, nil
	}
	fieldValue := msg.NewField(valueFD)
	if valueFD.Kind() != protoreflect.MessageKind {
		return false, xerrors.Errorf("union value (oneof) as scalar type not supported: %s", valueFD.FullName())
	}
	// MUST be message type.
	md := valueFD.Message()
	fieldMsg := fieldValue.Message()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		valNodeName := unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number()))
		valNode := node.FindChild(valNodeName)
		err := func() error {
			subField := sp.parseFieldDescriptor(fd)
			defer subField.release()
			if valNode == nil && xproto.GetFieldDefaultValue(fd) != "" {
				// if this field has a default value, use virtual node
				valNode = &book.Node{
					Name:  node.Name,
					Value: node.Value,
				}
			}
			if valNode == nil {
				if sp.IsFieldOptional(subField) {
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
			crossNodeValues := []string{valNode.Value}
			if fieldCount := fieldprop.GetUnionCrossFieldCount(subField.opts.Prop); fieldCount > 0 {
				for j := 1; j < fieldCount; j++ {
					nodeName := unionDesc.ValueFieldName() + strconv.Itoa(int(fd.Number())+j)
					node := node.FindChild(nodeName)
					if node == nil {
						break
					}
					crossNodeValues = append(crossNodeValues, node.Value)
				}
			}
			return sp.parseUnionMessageField(subField, fieldMsg, cardPrefix, crossNodeValues)
		}()
		if err != nil {
			return false, xerrors.WrapKV(err, valNode.DebugNameKV()...)
		}
	}
	msg.Set(valueFD, fieldValue)
	return
}
