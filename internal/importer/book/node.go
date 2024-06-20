package book

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tableauio/tableau/internal/printer"
	"github.com/tableauio/tableau/internal/types"
)

type Kind int

const (
	ScalarNode Kind = iota
	ListNode
	MapNode
	DocumentNode
)

func (k Kind) String() string {
	switch k {
	case ScalarNode:
		return "scalar"
	case ListNode:
		return "list"
	case MapNode:
		return "map"
	case DocumentNode:
		return "document"
	default:
		return "unknown"
	}
}

const (
	KeywordSheet  = "@sheet"
	KeywordType   = "@type"
	KeywordStruct = "@struct"
	KeywordKey    = types.DefaultDocumentMapKeyOptName   // @key
	KeywordValue  = types.DefaultDocumentMapValueOptName // @value
)

// MetaSign signifies the name starts with leading "@" is meta name.
const MetaSign = "@"

// Node represents an element in the tree document hierarchy.
//
// References:
//   - https://pkg.go.dev/gopkg.in/yaml.v3?utm_source=godoc#Node
//   - https://pkg.go.dev/github.com/moovweb/gokogiri/xml#Node
type Node struct {
	Kind     Kind
	Name     string
	Value    string
	Children []*Node

	// Line and Column hold the node position in the file.
	Line   int
	Column int
}

// GetValue returns node's value. It will return empty string if
// node is nil.
func (n *Node) GetValue() string {
	if n == nil {
		return ""
	}
	return n.Value
}

// IsMeta checks whether this node is meta (name starts with leading "@") or not.
func (n *Node) IsMeta() bool {
	return strings.HasPrefix(n.Name, MetaSign)
}

// GetDataSheetName returns original data sheet name by removing
// leading symbol "@" from meta sheet name.
//
// e.g.: "@SheetName" -> "SheetName"
func (n *Node) GetDataSheetName() string {
	return strings.TrimPrefix(n.Name, MetaSign)
}

// GetMetaSheet returns this node's @sheet defined in document node.
func (n *Node) GetMetaSheet() string {
	if n.Kind == DocumentNode && len(n.Children) > 0 {
		return n.Children[0].FindChild(KeywordSheet).GetValue()
	}
	return ""
}

// GetMetaType returns this node's @type defined in schema sheet.
func (n *Node) GetMetaType() string {
	// If no children, then just treat content as type name
	if len(n.Children) == 0 {
		return n.Value
	}
	return n.FindChild(KeywordType).GetValue()
}

// GetMetaKey returns this node's key defined in schema sheet.
func (n *Node) GetMetaKey() string {
	// If no children, then just treat content as type name
	keyNode := n.GetMetaStructNode().FindChild(KeywordKey)
	if keyNode != nil {
		return keyNode.Value
	}
	return strings.TrimPrefix(KeywordKey, "@")
}

// GetMetaStructNode returns this node's struct defined in schema sheet.
func (n *Node) GetMetaStructNode() *Node {
	return n.FindChild(KeywordStruct)
}

// FindChild finds the child with specified name.
func (n *Node) FindChild(name string) *Node {
	if n == nil {
		return nil
	}
	for _, child := range n.Children {
		if child.Name == name {
			return child
		}
	}
	return nil
}

// String returns hierarchy representation of the Node, mainly
// for debugging.
func (n *Node) String() string {
	var buffer bytes.Buffer
	dumpNode(n, DocumentNode, &buffer, 0)
	return buffer.String()
}

func dumpNode(node *Node, parentKind Kind, buffer *bytes.Buffer, depth int) {
	var line string
	switch node.Kind {
	case ScalarNode:
		switch parentKind {
		case ListNode:
			line = fmt.Sprintf("%s- %s # %s", printer.Indent(depth), node.Value, node.Kind)
		default:
			line = fmt.Sprintf("%s%s: %s # %s", printer.Indent(depth), node.Name, node.Value, node.Kind)
		}
	case ListNode:
		line = fmt.Sprintf("%s%s: # %s", printer.Indent(depth), node.Name, node.Kind)
	case MapNode:
		var desc string
		if node.Name == "" {
			desc = fmt.Sprintf("# %s", node.Kind)
		} else {
			desc = fmt.Sprintf("%s: # %s", node.Name, node.Kind)
		}
		switch parentKind {
		case ListNode:
			line = fmt.Sprintf("%s- %s", printer.Indent(depth), desc)
		default:
			line = fmt.Sprintf("%s%s", printer.Indent(depth), desc)
		}
	case DocumentNode:
		line = fmt.Sprintf("%s# %s %s %v", printer.Indent(depth), node.Kind, node.Name, node.IsMeta())
	}
	buffer.WriteString(line + "\n")
	for _, child := range node.Children {
		dumpNode(child, node.Kind, buffer, depth+1)
	}
}
