package confgen

import (
	"strings"

	"github.com/tableauio/tableau/internal/confgen/prop"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/types"
	"github.com/tableauio/tableau/internal/xproto"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type documentParser struct {
	parser *sheetParser
}

func (sp *documentParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	// log.Debugf("parse sheet: %s", sheet.Name)
	msg := protomsg.ProtoReflect()
	// for _, node := range sheet.Document.Children[0].Children {
	// 	_, err := sp.parseFieldOptions(msg, node)
	// 	if err != nil {
	// 		return err
	// 	}
	// }
	_, err := sp.parseFieldOptions(msg, sheet.Document.Children[0])
	if err != nil {
		return err
	}
	return nil
}

// parseFieldOptions is aimed to parse the options of all the fields of a protobuf message.
func (sp *documentParser) parseFieldOptions(msg protoreflect.Message, node *book.Node) (present bool, err error) {
	md := msg.Descriptor()
	for i := 0; i < md.Fields().Len(); i++ {
		fd := md.Fields().Get(i)
		err := func() error {
			field := parseFieldDescriptor(fd, sp.parser.opts.Sep, sp.parser.opts.Subsep)
			defer field.release()
			fieldNode := node.FindChild(field.opts.Name)
			if fieldNode == nil {
				return xerrors.Errorf("field node not found: %s", field.opts.Name)
			}
			fieldPresent, err := sp.parseField(field, msg, fieldNode)
			if err != nil {
				return xerrors.WithMessageKV(err,
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

	// simple scalar map
	var isScalar bool
	for _, elemNode := range node.Children {
		if elemNode.Kind == book.ScalarNode {
			isScalar = true
			break
		}
	}
	if isScalar {
		var pairs []string
		for _, elemNode := range node.Children {
			pairs = append(pairs, elemNode.Name+":"+elemNode.Content)
		}
		data := strings.Join(pairs, ",")
		err = sp.parser.parseIncellMap(field, reflectMap, data)
		if err != nil {
			return false, xerrors.WithMessageKV(err)
		}
	} else {
		if valueFd.Kind() == protoreflect.MessageKind {
			for _, elemNode := range node.Children {
				newMapKey, keyPresent, err := sp.parser.parseMapKey(field, reflectMap, elemNode.Name)
				if err != nil {
					return false, xerrors.WithMessageKV(err)
				}
				var newMapValue protoreflect.Value
				if reflectMap.Has(newMapKey) {
					newMapValue = reflectMap.Mutable(newMapKey)
				} else {
					newMapValue = reflectMap.NewValue()
				}
				// auto add virtual key node
				keyNode := &book.Node{
					Kind:       book.ScalarNode,
					Name:       book.KeywordKey,
					Content:    elemNode.Name,
					Attributes: nil,
					Children:   nil,
					Line:       node.Line,
					Column:     node.Column,
				}
				elemNode.Children = append(elemNode.Children, keyNode)
				valuePresent, err := sp.parseFieldOptions(newMapValue.Message(), elemNode)
				if err != nil {
					return false, xerrors.WithMessageKV(err)
				}
				// TODO: auto remove added virtual key node?
				// check key uniqueness
				if reflectMap.Has(newMapKey) {
					if prop.RequireUnique(field.opts.Prop) ||
						(!prop.HasUnique(field.opts.Prop) && sp.parser.deduceMapKeyUnique(field, reflectMap)) {
						return false, xerrors.WrapKV(xerrors.E2005(elemNode.Name))
					}
				}
				if !keyPresent && !valuePresent {
					// key and value are both not present.
					continue
				}
				reflectMap.Set(newMapKey, newMapValue)
			}
		} else {
			return false, xerrors.Errorf("should not reach here: %v", valueFd.Kind())
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

func (sp *documentParser) parseListField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	// Mutable returns a mutable reference to a composite type.
	newValue := msg.Mutable(field.fd)
	list := newValue.List()
	// TODO: incell list?
	for _, elemNode := range node.Children {
		elemPresent := false
		newListValue := list.NewElement()
		if field.fd.Kind() == protoreflect.MessageKind {
			// cross-cell struct list
			elemPresent, err = sp.parseFieldOptions(newListValue.Message(), elemNode)
			if err != nil {
				return false, xerrors.WithMessageKV(err, "cross-cell struct list", "failed to parse struct")
			}
		} else {
			// cross-cell scalar list
			newListValue, elemPresent, err = sp.parser.parseFieldValue(field.fd, elemNode.Content, field.opts.Prop)
			if err != nil {
				return false, xerrors.WithMessageKV(err)
			}
		}
		if elemPresent {
			list.Append(newListValue)
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
	return false, xerrors.Errorf("union type not supported yet")
}

func (sp *documentParser) parseStructField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	var structValue protoreflect.Value
	if msg.Has(field.fd) {
		// Get it if this field is populated. It will be overwritten if present.
		structValue = msg.Mutable(field.fd)
	} else {
		structValue = msg.NewField(field.fd)
	}
	subMsgName := string(field.fd.Message().FullName())
	if types.IsWellKnownMessage(subMsgName) {
		// built-in message type: google.protobuf.Timestamp, google.protobuf.Duration
		value, present, err := sp.parser.parseFieldValue(field.fd, node.Content, field.opts.Prop)
		if err != nil {
			return false, xerrors.WithMessageKV(err)
		}
		if present {
			msg.Set(field.fd, value)
		}
		return present, nil
	} else {
		present, err := sp.parseFieldOptions(structValue.Message(), node)
		if err != nil {
			return false, xerrors.WithMessageKV(err)
		}
		if present {
			// only set field if it is present.
			msg.Set(field.fd, structValue)
		}
		return present, nil
	}
}

func (sp *documentParser) parseScalarField(field *Field, msg protoreflect.Message, node *book.Node) (present bool, err error) {
	if msg.Has(field.fd) {
		// Only parse if this field is not populated. This means the first
		// none-empty related row part (related to scalar) is parsed.
		return true, nil
	}
	var newValue protoreflect.Value
	newValue, present, err = sp.parser.parseFieldValue(field.fd, node.Content, field.opts.Prop)
	if err != nil {
		return false, xerrors.WithMessageKV(err)
	}
	if !present {
		return false, nil
	}
	msg.Set(field.fd, newValue)
	return true, nil
}
