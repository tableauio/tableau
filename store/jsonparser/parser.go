package jsonparser

type Parser interface {
	Parse(string) (Node, error)
}

type Node interface {
	String() (string, error)
	StrictString() (string, error)
	Get(string) Node
	Index(int) Node

	SetString(string)
}
