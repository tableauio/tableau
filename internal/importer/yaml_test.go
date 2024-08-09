package importer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"gopkg.in/yaml.v3"
)

func Test_inspectYAMLNode(t *testing.T) {
	// your byte array
	data := []byte(`
---
"@sheet": "@TABLEAU"
ServiceConf:
  Template: true
---
# define schema
"@sheet": "@ServiceConf"
ID: uint32
Name: string
---
"@sheet": ServiceConf
ID: {{ env.id }}
Name: {{ env.name}}
{% if env.name == 'prod' %}
Enabled: true
{% else %}
Enabled: false
{% endif %}
`)

	rawDocs, err := extractRawYAMLDocuments(string(data))
	require.NoError(t, err)
	for i, rawDoc := range rawDocs {
		if !isSchemaSheet(rawDoc) {
			continue
		}
		var node yaml.Node
		err := yaml.Unmarshal([]byte(rawDoc), &node)
		require.NoError(t, err)
		sheet, err := parseYAMLSheet(&node, i)
		require.NoError(t, err)
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

func Test_readYAMLBookWithOnlySchemaSheet(t *testing.T) {
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
			name: "TestTemplate.yaml",
			args: args{
				filename: "testdata/TestTemplate.yaml",
				parser:   nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readYAMLBookWithOnlySchemaSheet(tt.args.filename, tt.args.parser)
			if (err != nil) != tt.wantErr {
				t.Errorf("readYAMLBookWithOnlySchemaSheet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got.String())
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("readYAMLBookWithOnlySchemaSheet() = %v, want %v", got, tt.want)
			// }
		})
	}
}
