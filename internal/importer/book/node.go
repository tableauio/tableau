package book

import (
	"bytes"
	"fmt"

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

	IsMeta bool // for DocumentNode, this node is metasheet or not

	// Line and Column hold the node position in the file.
	Line   int
	Column int
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
		line = fmt.Sprintf("%s# %s %s %v", printer.Indent(depth), node.Kind, node.Name, node.IsMeta)
	}
	buffer.WriteString(line + "\n")
	for _, child := range node.Children {
		dumpNode(child, node.Kind, buffer, depth+1)
	}
}
