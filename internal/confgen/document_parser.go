package confgen

import (
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
		return xerrors.Errorf("document should have and only have one child (map node), sheet: %s", sheet.Name)
	}
	// get the first child (map node) in document
	child := sheet.Document.Children[0]
	msg := protomsg.ProtoReflect()
	_, err := sp.parseMessage(msg, child)
	if err != nil {
		return err
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
			if field.opts.Name == "" {
				// just treat self node (with meta child removed) as field node
				// if option Name is empty
				fieldNode = &book.Node{
					Kind:     node.Kind,
					Name:     node.Name,
					Value:    node.Value,
					Children: []*book.Node{},
					Line:     node.Line,
					Column:   node.Column,
				}
				for _, child := range node.Children {
					if !child.IsMeta() {
						fieldNode.Children = append(fieldNode.Children, child)
					}
				}
			} else {
				fieldNode = node.FindChild(field.opts.Name)
				if fieldNode == nil {
					if field.opts.Optional {
						// field not found and is optional, no need to process, just return.
						return nil
					}
					return xerrors.Errorf("field node not found: %s", field.opts.Name)
				}
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

	// simple scalar map (span inner cell)
	if valueFd.Kind() != protoreflect.MessageKind || field.opts.Span == tableaupb.Span_SPAN_INNER_CELL {
		var pairs []string
		for _, elemNode := range node.Children {
			pairs = append(pairs, elemNode.Name+":"+elemNode.Value)
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
					Kind:     book.ScalarNode,
					Name:     field.opts.Key,
					Value:    elemNode.Name,
					Children: nil,
					Line:     node.Line,
					Column:   node.Column,
				}
				elemNode.Children = append(elemNode.Children, keyNode)
				valuePresent, err := sp.parseMessage(newMapValue.Message(), elemNode)
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
			elemPresent, err = sp.parseMessage(newListValue.Message(), elemNode)
			if err != nil {
				return false, xerrors.WithMessageKV(err, "cross-cell struct list", "failed to parse struct")
			}
		} else {
			// cross-cell scalar list
			newListValue, elemPresent, err = sp.parser.parseFieldValue(field.fd, elemNode.Value, field.opts.Prop)
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
		value, present, err := sp.parser.parseFieldValue(field.fd, node.Value, field.opts.Prop)
		if err != nil {
			return false, xerrors.WithMessageKV(err)
		}
		if present {
			msg.Set(field.fd, value)
		}
		return present, nil
	} else {
		present, err := sp.parseMessage(structValue.Message(), node)
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
	newValue, present, err = sp.parser.parseFieldValue(field.fd, node.Value, field.opts.Prop)
	if err != nil {
		return false, xerrors.WithMessageKV(err)
	}
	if !present {
		return false, nil
	}
	msg.Set(field.fd, newValue)
	return true, nil
}
