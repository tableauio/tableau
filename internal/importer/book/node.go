package book

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/tableauio/tableau/internal/printer"
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
	KeywordKey    = "@key"
)

// const DefaultNodeKeyVarName = "key"

// Node represents an element in the tree document hierarchy.
//
// References:
//   - https://pkg.go.dev/gopkg.in/yaml.v3?utm_source=godoc#Node
//   - https://pkg.go.dev/github.com/moovweb/gokogiri/xml#Node
type Node struct {
	Kind       Kind
	Name       string
	Content    string
	Attributes map[string]string // name -> value
	Children   []*Node

	// Line and Column hold the node position in the file.
	Line   int
	Column int
}

// IsMeta checks whether this node is meta (which defines schema) or not.
func (n *Node) IsMeta() bool {
	return strings.HasPrefix(n.Name, "@")
}

// GetDataSheetName returns original data sheet name by removing
// leading symbol "@" from meta sheet name.
//
// e.g.: "@SheetName" -> "SheetName"
func (n *Node) GetDataSheetName() string {
	return strings.TrimPrefix(n.Name, "@")
}

// GetMetaType returns this node's type defined in schema sheet.
func (n *Node) GetMetaType() string {
	// If no children, then just treat content as type name
	if len(n.Children) == 0 {
		return n.Content
	}
	for _, child := range n.Children {
		if child.Name == KeywordType {
			return child.Content
		}
	}
	return ""
}

// GetMetaKey returns this node's key defined in schema sheet.
func (n *Node) GetMetaKey() string {
	// If no children, then just treat content as type name
	structNode := n.GetMetaStructNode()
	if structNode != nil {
		for _, child := range structNode.Children {
			if child.Name == KeywordKey {
				return child.Content
			}
		}
	}
	return strings.TrimPrefix(KeywordKey, "@")
}

// GetMetaStructNode returns this node's struct defined in schema sheet.
func (n *Node) GetMetaStructNode() *Node {
	for _, child := range n.Children {
		if child.Name == "@struct" {
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
			line = fmt.Sprintf("%s- %s # %s", printer.Indent(depth), node.Content, node.Kind)
		default:
			line = fmt.Sprintf("%s%s: %s # %s", printer.Indent(depth), node.Name, node.Content, node.Kind)
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
