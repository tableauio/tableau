package importer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xerrors"
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

func TestYAMLImporter_extractsCommentAsNote(t *testing.T) {
	imp, err := NewYAMLImporter(
		context.Background(),
		"testdata/TestNote.yaml",
	)
	require.NoError(t, err)

	sheet := imp.GetSheet("@NoteConf")
	require.NotNil(t, sheet, "book sheets: %v", func() []string {
		var names []string
		for _, s := range imp.GetSheets() {
			names = append(names, s.Name)
		}
		return names
	}())
	require.Len(t, sheet.Document.Children, 1)
	root := sheet.Document.Children[0]

	findChild := func(n *book.Node, name string) *book.Node {
		for _, c := range n.Children {
			if c.Name == name {
				return c
			}
		}
		return nil
	}

	// Scalar fields: trailing `# ...` on the same line.
	id := findChild(root, "ID")
	require.NotNil(t, id)
	assert.Equal(t, "primary key", id.Note)

	name := findChild(root, "Name")
	require.NotNil(t, name)
	assert.Equal(t, "display name", name.Note)

	// List field: `# ...` on the key line (value is a mapping below).
	fruits := findChild(root, "Fruits")
	require.NotNil(t, fruits)
	assert.Equal(t, "fruit list", fruits.Note)
	structNode := findChild(fruits, "@struct")
	require.NotNil(t, structNode)
	fid := findChild(structNode, "ID")
	require.NotNil(t, fid)
	assert.Equal(t, "fruit id", fid.Note)
	fname := findChild(structNode, "Name")
	require.NotNil(t, fname)
	assert.Equal(t, "fruit name", fname.Note)

	// Map field: `# ...` on the key line (value is a mapping below).
	countries := findChild(root, "Countries")
	require.NotNil(t, countries)
	assert.Equal(t, "country map", countries.Note)
	cstruct := findChild(countries, "@struct")
	require.NotNil(t, cstruct)
	desc := findChild(cstruct, "Desc")
	require.NotNil(t, desc)
	assert.Equal(t, "short description", desc.Note)
}
