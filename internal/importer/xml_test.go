package importer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/subchen/go-xmldom"
	"github.com/tableauio/tableau/internal/importer/book"
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
	metasheet := splitXMLMetasheet(string(data))
	rawDocs, err := extractRawXMLDocuments(metasheet)
	require.NoError(t, err)
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		require.NoError(t, err)
		sheet, err := parseXMLSheet(doc, Protogen)
		require.NoError(t, err)
		fmt.Println(sheet.String())
	}
	// confgen
	rawDocs, err = extractRawXMLDocuments(string(data))
	require.NoError(t, err)
	for _, rawDoc := range rawDocs {
		doc, err := xmldom.ParseXML(rawDoc)
		require.NoError(t, err)
		sheet, err := parseXMLSheet(doc, Confgen)
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
			name: "Test.xml",
			args: args{
				filename: "testdata/Test.xml",
				mode:     Protogen,
				parser:   &TestSheetParser{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewXMLImporter(tt.args.filename, tt.args.sheetNames, tt.args.parser, tt.args.mode, tt.args.cloned)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewYAMLImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			fmt.Println(got.String())
		})
	}
}
