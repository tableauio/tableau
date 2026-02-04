package jsonparser

import (
	"strconv"

	"github.com/valyala/fastjson"
)

var Fastjson Parser = &fastjsonParser{}

type fastjsonParser struct{}

func (p *fastjsonParser) Parse(jsonStr string) (Node, error) {
	root, err := fastjson.Parse(jsonStr)
	if err != nil {
		return nil, err
	}
	return &fastjsonNode{node: root}, nil
}

type fastjsonNode struct {
	node *fastjson.Value
}

func (n *fastjsonNode) String() (string, error) {
	return n.node.String(), nil
}

func (n *fastjsonNode) StrictString() (string, error) {
	bytes, err := n.node.StringBytes()
	return string(bytes), err
}

func (n *fastjsonNode) Get(key string) Node {
	return &fastjsonNode{node: n.node.Get(key)}
}

func (n *fastjsonNode) Index(i int) Node {
	return &fastjsonNode{node: n.node.Get(strconv.Itoa(i))}
}

func (n *fastjsonNode) SetString(s string) {
	*n.node = *fastjson.MustParse(strconv.Quote(s))
}
