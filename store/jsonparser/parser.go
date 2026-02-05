package jsonparser

type Parser interface {
	// Parse parses the given json string into a node.
	Parse(string) (Node, error)
}

type Node interface {
	// String marshals the node into a json string.
	String() (string, error)
	// StrictString returns the string value of a string-type node.
	// For other type nodes, it returns an error.
	StrictString() (string, error)
	// Get returns the child node of a object-type node by the given key.
	Get(string) Node
	// Index returns the child node of a array-type node by the given index.
	Index(int) Node
	// SetString sets the string value of a string-type node.
	SetString(string)
}
