package importer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/subchen/go-xmldom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/importer/metasheet"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

func Test_inspectXMLNode(t *testing.T) {
	// your byte array
	data := []byte(`
<?xml version="1.0" encoding="UTF-8"?>
<!--
<@TABLEAU>
  <Item Sheet="RankConf"/>
</@TABLEAU>
<RankConf>
  <RankItem ID="map<uint32, RankItem>" Score="int32" Name="string"/>
  <MaxScore>int32</MaxScore>
</RankConf>
-->

<RankConf>
  <RankItem ID="1" Score="100" Name="Tony"/>
  <RankItem ID="2" Score="99" Name="Eric"/>
  <RankItem ID="3" Score="98" Name="David"/>
  <RankItem ID="4" Score="98" Name="Jenny"/>
  <MaxScore>100</MaxScore>
</RankConf>
`)
	// protogen
	ms := extractXMLMetasheetInComment(string(data), metasheet.DefaultMetasheetName)
	rawDocs, err := extractRawXMLDocuments(ms)
	require.NoError(t, err)
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		require.NoError(t, err)
		sheet, err := parseXMLSheet(doc, Protogen, metasheet.DefaultMetasheetName)
		require.NoError(t, err)
		fmt.Println(sheet.String())
	}
	// confgen
	rawDocs, err = extractRawXMLDocuments(string(data))
	require.NoError(t, err)
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		require.NoError(t, err)
		sheet, err := parseXMLSheet(doc, Confgen, metasheet.DefaultMetasheetName)
		require.NoError(t, err)
		fmt.Println(sheet.String())
	}
}

func TestXMLImporter_extractsNote(t *testing.T) {
	data := []byte(`
<?xml version="1.0" encoding="UTF-8"?>
<!--
<@TABLEAU>
  <Item Sheet="NoteConf"/>
</@TABLEAU>
<NoteConf>
    <Item @note="item struct" ID="uint32" Name="string" @note.ID="primary key" @note.Name="display name" />
    <KeyMap Name="map<string, KeyMap>" @note.Name="key field note" Score="int32" @note.Score="score note" />
    <Deep @note="deep struct">
        <Sub ID="uint32" @note.ID="sub field id" />
    </Deep>
    <Scalar @note="scalar field">int32</Scalar>
</NoteConf>
-->
`)

	ms := extractXMLMetasheetInComment(string(data), metasheet.DefaultMetasheetName)
	rawDocs, err := extractRawXMLDocuments(ms)
	require.NoError(t, err)
	require.Len(t, rawDocs, 2)
	// First doc is the metasheet, second is the schema sheet.
	doc, err := xmldom.ParseXML(rawDocs[1])
	require.NoError(t, err)
	sheet, err := parseXMLSheet(doc, Protogen, metasheet.DefaultMetasheetName)
	require.NoError(t, err)

	// sheet.Document.Children[0] is the map node holding all fields.
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

	// Item: element-level note + inline attribute field notes
	item := findChild(root, "Item")
	require.NotNil(t, item)
	assert.Equal(t, "item struct", item.Note)
	itemStruct := item.FindChild(book.KeywordStruct)
	require.NotNil(t, itemStruct)
	idField := itemStruct.FindChild("ID")
	require.NotNil(t, idField)
	assert.Equal(t, "primary key", idField.Note)
	nameField := itemStruct.FindChild("Name")
	require.NotNil(t, nameField)
	assert.Equal(t, "display name", nameField.Note)

	// Scalar: note via attribute on a text-only child element.
	scalar := findChild(root, "Scalar")
	require.NotNil(t, scalar)
	assert.Equal(t, "scalar field", scalar.Note)

	// KeyMap: a vertical struct map. The `@note.Name` note targets the map
	// key field (represented by the @key node), which previously was dropped.
	keyMap := findChild(root, "KeyMap")
	require.NotNil(t, keyMap)
	keyMapStruct := keyMap.FindChild(book.KeywordStruct)
	require.NotNil(t, keyMapStruct)
	keyNode := keyMapStruct.FindChild(book.KeywordKey)
	require.NotNil(t, keyNode)
	assert.Equal(t, "key field note", keyNode.Note)
	scoreField := keyMapStruct.FindChild("Score")
	require.NotNil(t, scoreField)
	assert.Equal(t, "score note", scoreField.Note)

	// Deep: element-level note on nested struct
	deep := findChild(root, "Deep")
	require.NotNil(t, deep)
	assert.Equal(t, "deep struct", deep.Note)
	deepStruct := deep.FindChild(book.KeywordStruct)
	require.NotNil(t, deepStruct)
	subField := deepStruct.FindChild("Sub")
	require.NotNil(t, subField)
	// Sub is itself a struct; its inner field ID has a note.
	subStruct := subField.FindChild(book.KeywordStruct)
	require.NotNil(t, subStruct)
	subID := subStruct.FindChild("ID")
	require.NotNil(t, subID)
	assert.Equal(t, "sub field id", subID.Note)
}

func TestXMLImporter_textOnlyChildNote(t *testing.T) {
	tests := []struct {
		name     string
		xml      string
		find     func(root *book.Node) *book.Node
		wantKey  string
		wantNote string
	}{
		{
			name: "scalar text-only child with note",
			xml: `<NoteConf>
    <Score @note="player score">int32</Score>
</NoteConf>`,
			find: func(root *book.Node) *book.Node {
				return root.FindChild("Score")
			},
			wantNote: "player score",
		},
		{
			name: "text-only child without note still works",
			xml: `<NoteConf>
    <Level>int32</Level>
</NoteConf>`,
			find: func(root *book.Node) *book.Node {
				return root.FindChild("Level")
			},
			wantNote: "",
		},
		{
			name: "nested text-only child with note inside struct",
			xml: `<NoteConf>
    <Config>
        <Timeout @note="timeout in seconds">int32</Timeout>
        <Retry @note="retry count">int32</Retry>
    </Config>
</NoteConf>`,
			find: func(root *book.Node) *book.Node {
				configStruct := root.FindChild("Config")
				if configStruct == nil {
					return nil
				}
				s := configStruct.FindChild(book.KeywordStruct)
				if s == nil {
					return nil
				}
				return s.FindChild("Timeout")
			},
			wantNote: "timeout in seconds",
		},
		{
			name: "text-only child with note and special characters",
			xml: `<NoteConf>
    <Desc @note="player description: name &amp; title">string</Desc>
</NoteConf>`,
			find: func(root *book.Node) *book.Node {
				return root.FindChild("Desc")
			},
			wantNote: "player description: name & title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!--
<@TABLEAU>
  <Item Sheet="NoteConf"/>
</@TABLEAU>
` + tt.xml + `
-->
`)
			ms := extractXMLMetasheetInComment(string(data), metasheet.DefaultMetasheetName)
			rawDocs, err := extractRawXMLDocuments(ms)
			require.NoError(t, err)
			require.Len(t, rawDocs, 2)
			doc, err := xmldom.ParseXML(rawDocs[1])
			require.NoError(t, err)
			sheet, err := parseXMLSheet(doc, Protogen, metasheet.DefaultMetasheetName)
			require.NoError(t, err)
			require.Len(t, sheet.Document.Children, 1)
			root := sheet.Document.Children[0]
			node := tt.find(root)
			require.NotNil(t, node, "field not found")
			assert.Equal(t, tt.wantNote, node.Note)
		})
	}
}

func TestNewXMLImporter(t *testing.T) {
	type args struct {
		filename string
		setters  []Option
	}
	tests := []struct {
		name    string
		args    args
		want    *XMLImporter
		wantErr bool
		err     error
	}{
		{
			name: "not-exist",
			args: args{
				filename: "testdata/not-exist.xml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
		{
			name: "not-exist",
			args: args{
				filename: "testdata/not-exist.xml",
				setters: []Option{
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
			err:     xerrors.ErrE3002,
		},
		{
			name: "Test.xml",
			args: args{
				filename: "testdata/Test.xml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
		},
		{
			name: "InvalidMeta1.xml",
			args: args{
				filename: "testdata/InvalidMeta1.xml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
		{
			name: "InvalidMeta2.xml",
			args: args{
				filename: "testdata/InvalidMeta2.xml",
				setters: []Option{
					Mode(Protogen),
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
		{
			name: "InvalidMeta2.xml",
			args: args{
				filename: "testdata/InvalidMeta2.xml",
				setters: []Option{
					Parser(&TestSheetParser{}),
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewXMLImporter(context.Background(), tt.args.filename, tt.args.setters...)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewXMLImporter() error = %v, wantErr %v", err, tt.wantErr)
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

func Test_escapeAttrs(t *testing.T) {
	type args struct {
		doc string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "standard",
			args: args{
				doc: `
<Conf>
    <Server Type = "map<enum<.ServerType>, Server>" Value = "int32"/>
</Conf>
`,
			},
			want: `
<Conf>
    <Server Type="map&lt;enum&lt;.ServerType&gt;, Server&gt;" Value="int32"/>
</Conf>
`,
		},
		{
			name: "FeatureToggle",
			args: args{
				doc: `
<Conf>
	<Client EnvID="map<uint32,Client>">
		<Toggle ID="map<enum<.ToggleType>, Toggle>" WorldID="uint32"/>
	</Client>
</Conf>
`,
			},
			want: `
<Conf>
	<Client EnvID="map&lt;uint32,Client&gt;">
		<Toggle ID="map&lt;enum&lt;.ToggleType&gt;, Toggle&gt;" WorldID="uint32"/>
	</Client>
</Conf>
`,
		},
		{
			name: "Prop",
			args: args{
				doc: `
<Conf>
	<Client ID="map<uint32, Client>|{unique:true range:"1,~"}" OpenTime="datetime|{default:"2022-01-23 15:40:00"}"/>
</Conf>
`,
			},
			want: `
<Conf>
	<Client ID="map&lt;uint32, Client&gt;|{unique:true range:&#34;1,~&#34;}" OpenTime="datetime|{default:&#34;2022-01-23 15:40:00&#34;}"/>
</Conf>
`,
		},
		{
			name: "SimpleTag",
			args: args{
				doc: `
<Conf>
	<ClientID>enum<.ClientIDType></ClientID>
</Conf>
`,
			},
			want: `
<Conf>
	<ClientID>enum&lt;.ClientIDType&gt;</ClientID>
</Conf>
`,
		},
		{
			name: "ComplexTag",
			args: args{
				doc: `
<Conf>
	<ClientID>map<uint32, Client>|{unique:true range:"1,~"}</ClientID>
</Conf>
`,
			},
			want: `
<Conf>
	<ClientID>map&lt;uint32, Client&gt;|{unique:true range:&#34;1,~&#34;}</ClientID>
</Conf>
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeAttrs(tt.args.doc); got != tt.want {
				t.Errorf("escapeAttrs() = %v, want %v", got, tt.want)
			}
		})
	}
}
