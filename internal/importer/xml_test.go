package importer

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/subchen/go-xmldom"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/metasheet"
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
	ms := splitXMLMetasheet(string(data), metasheet.DefaultMetasheetName)
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

func TestNewXMLImporter(t *testing.T) {
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
		want    *XMLImporter
		wantErr bool
	}{
		{
			name: "not-exist",
			args: args{
				filename: "testdata/not-exist.xml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
			wantErr: true,
		},
		{
			name: "Test.xml",
			args: args{
				filename: "testdata/Test.xml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
		},
		{
			name: "InvalidMeta1.xml",
			args: args{
				filename: "testdata/InvalidMeta1.xml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
			wantErr: true,
		},
		{
			name: "InvalidMeta2.xml",
			args: args{
				filename: "testdata/InvalidMeta2.xml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
			wantErr: true,
		},
		{
			name: "InvalidMeta2.xml",
			args: args{
				filename: "testdata/InvalidMeta2.xml",
				parser:   &TestSheetParser{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewXMLImporter(context.Background(), tt.args.filename, tt.args.sheetNames, tt.args.parser, tt.args.mode, tt.args.cloned)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewXMLImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				fmt.Println(got.String())
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
