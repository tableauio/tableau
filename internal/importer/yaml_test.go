package importer

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/log"
	"gopkg.in/yaml.v3"
)

func Test_readYAMLBook(t *testing.T) {
	type args struct {
		filename string
		parser   book.SheetParser
	}
	tests := []struct {
		name    string
		args    args
		want    *book.Book
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name: "testdata/test.yaml",
			args: args{
				filename: "test",
				parser:   nil,
			},
			want:    &book.Book{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readYAMLBook(tt.args.filename, tt.args.parser)
			if (err != nil) != tt.wantErr {
				t.Errorf("readYAMLBook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("readYAMLBook() = %v, want %v", got, tt.want)
			// }
		})
	}
}

func Test_inspectYAMLNode(t *testing.T) {
	// your byte array
	data := []byte(`
---
"@metasheet": "@TABLEAU"
AnimalConf:
Servers:
  gamesvr:
    Name: gamesvr
    Confs:
      ItemConf:
        Async: true
      DropConf:
        Async: true
`)
	// ---
	// "@metasheet": AnimalConf
	// Animals:
	//   "@type": "[]Animal"
	//   "@struct":
	//     ID: uint32
	//     Name: string
	// Username: John # line comment1
	// Age: 23
	// ---
	// "@sheet": AnimalConf
	// Animals:
	//   - ID: 1
	//     Name: fish
	//   - ID: 2
	//     Name: dog
	// Username: John # line comment1
	// Age: 23
	// `)

	// Create a new decoder
	dec := yaml.NewDecoder(bytes.NewReader(data))

	for {
		var node yaml.Node

		// Decode one document at a time
		err := dec.Decode(&node)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			} else {
				log.Fatalf("error: %v", err)
			}
		}
		sheet, err := parseYAMLSheet(&node)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("Sheet:\n", sheet.String())
		// Inspect the node
		inspectNode(&node, 0)
	}
}

func inspectNode(node *yaml.Node, level int) {
	indent := ""
	for i := 0; i < level; i++ {
		indent += "  "
	}
	switch node.Kind {
	case yaml.DocumentNode:
		fmt.Printf("%s--- Document\n", indent)
		for _, child := range node.Content {
			inspectNode(child, level+1)
		}
	case yaml.MappingNode:
		fmt.Printf("%sMapping\n", indent)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			value := node.Content[i+1]
			fmt.Printf("%s- Key: %v, Value: %v\n", indent, key.Value, value.Value)
			inspectNode(value, level+1)
		}
	case yaml.SequenceNode:
		fmt.Println(indent, "Sequence")
		for _, child := range node.Content {
			inspectNode(child, level+1)
		}
	case yaml.ScalarNode:
		// fmt.Printf("%sScalar: %v\n", indent, node.Value)
	default:
		fmt.Printf("%sUnknown node kind: %v\n", indent, node.Kind)
	}
}
