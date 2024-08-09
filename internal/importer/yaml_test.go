package importer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"google.golang.org/protobuf/proto"
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

type TestSheetParser struct {
}

func (p *TestSheetParser) Parse(protomsg proto.Message, sheet *book.Sheet) error {
	return nil
}

func TestNewYAMLImporter(t *testing.T) {
	type args struct {
		filename   string
		sheetNames []string
		parser     book.SheetParser
		mode       ImporterMode
		cloned     bool
	}
	tests := []struct {
		name    string
		args    args
		want    *YAMLImporter
		wantErr bool
	}{
		{
			name: "Test.yaml",
			args: args{
				filename: "testdata/Test.yaml",
				parser:   nil,
			},
		},
		{
			name: "TestTemplate.yaml",
			args: args{
				filename: "testdata/TestTemplate.yaml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewYAMLImporter(tt.args.filename, tt.args.sheetNames, tt.args.parser, tt.args.mode, tt.args.cloned)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewYAMLImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got.String())
			// if !reflect.DeepEqual(got, tt.want) {
			// 	t.Errorf("NewYAMLImporter() = %v, want %v", got, tt.want)
			// }
		})
	}
}
