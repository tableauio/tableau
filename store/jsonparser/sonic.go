package jsonparser

// import (
// 	"github.com/bytedance/sonic"
// 	"github.com/bytedance/sonic/ast"
// )

// var Sonic Parser = &sonicParser{}

// type sonicParser struct{}

// func (p *sonicParser) Parse(jsonStr string) (Node, error) {
// 	root, err := sonic.Get([]byte(jsonStr))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &sonicNode{node: &root}, nil
// }

// type sonicNode struct {
// 	node *ast.Node
// }

// func (n *sonicNode) String() (string, error) {
// 	return n.node.Raw()
// }

// func (n *sonicNode) StrictString() (string, error) {
// 	return n.node.StrictString()
// }

// func (n *sonicNode) Get(key string) Node {
// 	return &sonicNode{node: n.node.Get(key)}
// }

// func (n *sonicNode) Index(i int) Node {
// 	return &sonicNode{node: n.node.Index(i)}
// }

// func (n *sonicNode) SetString(s string) {
// 	*n.node = ast.NewString(s)
// }
