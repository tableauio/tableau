package importer

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/pkg/errors"
	"github.com/tableauio/tableau/internal/importer/book"
	"gopkg.in/yaml.v3"
)

func Test_inspectYAMLNode(t *testing.T) {
	// your byte array
	data := []byte(`
---
"@metasheet": "@TABLEAU"
LiteConf:
LoaderConf:
  OrderedMap: true
---
"@metasheet": LiteConf
RoleLite:
  "@type": Lite
  Expire: duration
  Count: int32
GuildLite:
  "@type": Lite
Ids: "[]int32"
Heros:
  "@type": "[]Hero"
  "@struct":
    ID: uint32
    Name: string
---
"@metasheet": LoaderConf
Servers:
  "@type": "map<string, Server>"
  "@struct":
    Name: string
    Confs:
      "@type": "map<string, Conf>"
      "@struct":
        Async: bool
        Limit: int32
---
"@sheet": LiteConf
RoleLite:
  Expire: 2h
  Count: 50
GuildLite:
  Expire: 2h
  Count: 50
Ids: [1, 2, 3]
Heros:
  - ID: 1
    Name: fish
  - ID: 2
    Name: dog
---
"@sheet": LoaderConf
Servers:
  gamesvr:
    Name: gamesvr
    Confs:
      ItemConf:
        Async: true
      DropConf:
        Async: true
  mailsvr:
    Name: mailsvr
    Confs:
      ItemConf:
        Async: true
      DropConf:
        Async: true
`)

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
				t.Fatalf("error: %v", err)
			}
		}
		sheet, err := parseYAMLSheet(&node)
		if err != nil {
			t.Fatalf("%+v", err)
		}
		fmt.Println(sheet.String())
	}
}

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
		{
			name: "Test.yaml",
			args: args{
				filename: "testdata/Test.yaml",
				parser:   nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readYAMLBook(tt.args.filename, tt.args.parser)
			if (err != nil) != tt.wantErr {
				t.Errorf("readYAMLBook() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got.String())
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("readYAMLBook() = %v, want %v", got, tt.want)
			// }
		})
	}
}
