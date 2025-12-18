package importer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

func Test_inspectYAMLNode(t *testing.T) {
	// your byte array
	data := []byte(`
"@sheet": "@TABLEAU"
ServiceConf:
  Template: true
  Patch: PATCH_MERGE
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
	fmt.Println("no-op: TestSheetParser for only test")
	return nil
}

func TestNewYAMLImporter(t *testing.T) {
	type args struct {
		filename string
		setters  []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *YAMLImporter
		wantErr bool
		err     error
	}{
		{
			name: "Test.yaml",
			args: args{
				filename: "testdata/Test.yaml",
			},
		},
		{
			name: "TestTemplate.yaml",
			args: args{
				filename: "testdata/TestTemplate.yaml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
		},
		{
			name: "not-exist.yaml",
			args: args{
				filename: "testdata/not-exist.yaml",
			},
			wantErr: true,
			err:     xerrors.ErrE3002,
		},
		{
			name: "NotSupportAliasNode.yaml",
			args: args{
				filename: "testdata/NotSupportAliasNode.yaml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
		{
			name: "NotSupportAliasNode.yaml",
			args: args{
				filename: "testdata/NotSupportAliasNode.yaml",
				setters: []Option{
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewYAMLImporter(context.Background(), tt.args.filename, tt.args.setters...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewYAMLImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				fmt.Println(got.String())
			} else if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			}
		})
	}
}
