package importer

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
)

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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := escapeAttrs(tt.args.doc); got != tt.want {
				t.Errorf("escapeAttrs() = %v, want %v", got, tt.want)
			}
		})
	}
}

func FindMetaNode(xmlSheet *tableaupb.XMLSheet, path string) *tableaupb.XMLNode {
	if node, ok := xmlSheet.MetaNodeMap[path]; ok {
		return node
	}
	return nil
}

func Test_isRepeated(t *testing.T) {
	doc := `
<?xml version='1.0' encoding='UTF-8'?>
<!-- 
<@TABLEAU />
-->

<MatchCfg Open="true">
	<TeamRatingWeight>
		<Weight Num="1">
			<Param Value="100"/>
		</Weight>
		<Weight Num="2">
			<Param Value="30"/>
			<Param Value="70"/>
		</Weight>
	</TeamRatingWeight>
</MatchCfg>
`
	metasheet, content := splitRawXML(doc)
	newBook := book.NewBook(`Test.xml`, `Test.xml`, nil)
	xmlMeta, _ := readXMLFile(metasheet, content, newBook)
	sheet1 := getXMLSheet(xmlMeta, "MatchCfg")
	node1 := FindMetaNode(sheet1, "MatchCfg/TeamRatingWeight/Weight")
	node2 := FindMetaNode(sheet1, "MatchCfg/TeamRatingWeight/Weight/Param")
	node3 := FindMetaNode(sheet1, "MatchCfg/TeamRatingWeight")
	node4 := FindMetaNode(sheet1, "MatchCfg")
	type args struct {
		xmlSheet *tableaupb.XMLSheet
		curr     *tableaupb.XMLNode
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "node1",
			args: args{
				xmlSheet: sheet1,
				curr:     node1,
			},
			want: true,
		},
		{
			name: "node2",
			args: args{
				xmlSheet: sheet1,
				curr:     node2,
			},
			want: true,
		},
		{
			name: "node3",
			args: args{
				xmlSheet: sheet1,
				curr:     node3,
			},
			want: false,
		},
		{
			name: "sheet attr",
			args: args{
				xmlSheet: sheet1,
				curr:     node4,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRepeated(tt.args.xmlSheet, tt.args.curr); got != tt.want {
				t.Errorf("isRepeated() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_matchAttr(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "scalar type",
			args: args{
				s: `<AAA bb = "bool" cc = "int64" dd = "enum<.EnumType>" >`,
			},
			want: []string{
				`bb = "bool"`, `bb`, `bool`, ``,
			},
		},
		{
			name: "Prop",
			args: args{
				s: `<Client OpenTime="datetime|{default:"2022-01-23 15:40:00"}" CloseTime="datetime|{default:"2022-01-23 15:40:00"}"/>`,
			},
			want: []string{
				`OpenTime="datetime|{default:"2022-01-23 15:40:00"}"`,
				`OpenTime`, `datetime`, `|{default:"2022-01-23 15:40:00"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchAttr(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchAttr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isFirstChild(t *testing.T) {
	doc := `
<?xml version='1.0' encoding='UTF-8'?>
<!--
<@TABLEAU>
	<Item Sheet="Server" />
</@TABLEAU>

<Server>
    <MapConf>
        <Weight Num="map&lt;uint32,Weight&gt;"/>
    </MapConf>
</Server>
-->
`
	metasheet, content := splitRawXML(doc)
	newBook := book.NewBook(`Test.xml`, `Test.xml`, nil)
	xmlMeta, _ := readXMLFile(metasheet, content, newBook)
	sheet1 := getXMLSheet(xmlMeta, "Server")
	node1 := FindMetaNode(sheet1, "Server/MapConf/Weight")
	type args struct {
		curr *tableaupb.XMLNode
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Weight",
			args: args{
				curr: node1,
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isFirstChild(tt.args.curr); got != tt.want {
				t.Errorf("isFirstChild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_fixNodeType(t *testing.T) {
	doc := `
<?xml version='1.0' encoding='UTF-8'?>
<!--
<@TABLEAU>
	<Item Sheet="MatchCfg" />
</@TABLEAU>

<MatchCfg open="true">
	<MatchMode MissionType="map&lt;enum&lt;.MissionType&gt;,MatchMode&gt;">
		<MatchAI IsOpen="bool" PlayerOnlyOneCamp="bool">
			<AI Type="[AI]&lt;enum&lt;.ENMAIWarmType&gt;&gt;" IsOpen="bool" MinTime="duration" MaxTime="duration" />
		</MatchAI>
    </MatchMode>
	<MapConf Param="[]int64">
		<Test>
        	<Weight Num="map&lt;uint32,Weight&gt;"/>
		</Test>
    </MapConf>
	<Client EnvID="map&lt;uint32,Client&gt;">
		<Toggle ID="map&lt;enum&lt;.ToggleType&gt;, Toggle&gt;" WorldID="uint32"/>
	</Client>
	
	<Broadcast Id="[Broadcast]int32" Priority="int32">
		<BroadcastTime BeginTime="datetime" EndTime="datetime" Gap="duration" />
		<Content Txt="string"/>
	</Broadcast>
</MatchCfg>
-->

<MatchCfg>
	<StructConf>
		<Weight Num="1">
			<Param Value="100"/>
		</Weight>
		<Test Value="1"/>
	</StructConf>

	<ListConf>
        <Weight Num="1">
            <Param Value="100"/>
        </Weight>
        <Weight Num="2">
            <Param Value="30"/>
            <Param Value="70"/>
        </Weight>
    </ListConf>
	
	<Broadcast Id="2" Priority="2">
		<BroadcastTime BeginTime="2016-11-15 19:30:10" EndTime="2022-04-21 23:29:59" Gap="60s" />
		<Content txt="每分钟发送一次测试"/>
	</Broadcast>
</MatchCfg>
`
	metasheet, content := splitRawXML(doc)
	newBook := book.NewBook(`Test.xml`, `Test.xml`, nil)
	xmlMeta, _ := readXMLFile(metasheet, content, newBook)
	sheet1 := getXMLSheet(xmlMeta, "MatchCfg")
	node1 := FindMetaNode(sheet1, "MatchCfg/MatchMode/MatchAI/AI")
	node2 := FindMetaNode(sheet1, "MatchCfg/MapConf/Test/Weight")
	node3 := FindMetaNode(sheet1, "MatchCfg/StructConf/Weight")
	node4 := FindMetaNode(sheet1, "MatchCfg/ListConf/Weight")
	node5 := FindMetaNode(sheet1, "MatchCfg/ListConf/Weight/Param")
	node6 := FindMetaNode(sheet1, "MatchCfg/Client/Toggle")
	node7 := FindMetaNode(sheet1, "MatchCfg")
	node8 := FindMetaNode(sheet1, "MatchCfg/StructConf/Test")
	node9 := FindMetaNode(sheet1, "MatchCfg/MapConf")
	node10 := FindMetaNode(sheet1, "MatchCfg/Broadcast/Content")
	type args struct {
		xmlSheet *tableaupb.XMLSheet
		curr     *tableaupb.XMLNode
		oriType  string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "MatchCfg/MatchMode/MatchAI/AI",
			args: args{
				xmlSheet: sheet1,
				curr:     node1,
				oriType:  `[AI]<enum<.ENMAIWarmType>>`,
			},
			want: `[AI]<enum<.ENMAIWarmType>>`,
		},
		{
			name: "MatchCfg/MapConf/Test/Weight",
			args: args{
				xmlSheet: sheet1,
				curr:     node2,
				oriType:  `map<uint32,Weight>`,
			},
			want: `{Test}map<uint32,Weight>`,
		},
		{
			name: "MatchCfg/StructConf/Weight",
			args: args{
				xmlSheet: sheet1,
				curr:     node3,
				oriType:  `int32`,
			},
			want: `{StructConf}{Weight}int32`,
		},
		{
			name: "MatchCfg/ListConf/Weight",
			args: args{
				xmlSheet: sheet1,
				curr:     node4,
				oriType:  `int32`,
			},
			want: `{ListConf}[Weight]<int32>`,
		},
		{
			name: "MatchCfg/ListConf/Weight/Param",
			args: args{
				xmlSheet: sheet1,
				curr:     node5,
				oriType:  `int32`,
			},
			want: `[Param]<int32>`,
		},
		{
			name: "MatchCfg/Client/Toggle",
			args: args{
				xmlSheet: sheet1,
				curr:     node6,
				oriType:  `map<enum<.ToggleType>, Toggle>`,
			},
			want: `map<enum<.ToggleType>, Toggle>`,
		},
		{
			name: "MatchCfg",
			args: args{
				xmlSheet: sheet1,
				curr:     node7,
				oriType:  `bool`,
			},
			want: `bool`,
		},
		{
			name: "MatchCfg/StructConf/Test",
			args: args{
				xmlSheet: sheet1,
				curr:     node8,
				oriType:  `int32`,
			},
			want: `{Test}int32`,
		},
		{
			name: "MatchCfg/MapConf@Param",
			args: args{
				xmlSheet: sheet1,
				curr:     node9,
				oriType:  `[]int64`,
			},
			want: `{MapConf}[]int64`,
		},
		{
			name: "MatchCfg/Broadcast/Content@txt",
			args: args{
				xmlSheet: sheet1,
				curr:     node10,
				oriType:  `string`,
			},
			want: `{Content}string`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fixNodeType(tt.args.xmlSheet, tt.args.curr, tt.args.oriType); got != tt.want {
				t.Errorf("fixNodeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getNodePath(t *testing.T) {
	metaDoc := `
<?xml version='1.0' encoding='UTF-8'?>
<MatchCfg open="true">
	<MapConf>
        <Weight Num="map&lt;uint32,Weight&gt;"/>
    </MapConf>
</MatchCfg>
`
	doc := `
<?xml version='1.0' encoding='UTF-8'?>
<MatchCfg>
	<StructConf>
		<Weight Num="1">
			<Param Value="100"/>
		</Weight>
		<Test Value="1"/>
	</StructConf>
</MatchCfg>
`
	metaRoot, _ := xmlquery.Parse(strings.NewReader(metaDoc))
	node1 := xmlquery.FindOne(metaRoot, "MatchCfg/MapConf/Weight")
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	node2 := xmlquery.FindOne(root, "MatchCfg/StructConf/Test")
	type args struct {
		curr *xmlquery.Node
	}
	tests := []struct {
		name     string
		args     args
		wantRoot *xmlquery.Node
		wantPath string
	}{
		{
			name: "MatchCfg/MapConf/Weight",
			args: args{
				curr: node1,
			},
			wantRoot: metaRoot,
			wantPath: "MatchCfg/MapConf/Weight",
		},
		{
			name: "MatchCfg/StructConf/Test",
			args: args{
				curr: node2,
			},
			wantRoot: root,
			wantPath: "MatchCfg/StructConf/Test",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRoot, gotPath := getNodePath(tt.args.curr)
			if !reflect.DeepEqual(gotRoot, tt.wantRoot) {
				t.Errorf("getNodePath() gotRoot = %v, want %v", gotRoot, tt.wantRoot)
			}
			if gotPath != tt.wantPath {
				t.Errorf("getNodePath() gotPath = %v, want %v", gotPath, tt.wantPath)
			}
		})
	}
}

func Test_isCrossCell(t *testing.T) {
	type args struct {
		t string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "map<uint32, Item>",
			args: args{
				t: "map<uint32, Item>",
			},
			want: true,
		},
		{
			name: "map<uint32, enum<.RankType>>",
			args: args{
				t: "map<uint32, enum<.RankType>>",
			},
			want: false,
		},
		{
			name: "[]int32",
			args: args{
				t: "[]int32",
			},
			want: false,
		},
		{
			name: "[.Item]uint32",
			args: args{
				t: "[.Item]uint32",
			},
			want: true,
		},
		{
			name: "[RankConf]<uint32>",
			args: args{
				t: "[RankConf]<uint32>",
			},
			want: true,
		},
		{
			name: "{int32 ID,string Name,string Desc}Prop",
			args: args{
				t: "{int32 ID,string Name,string Desc}Prop",
			},
			want: false,
		},
		{
			name: "{.Item}uint32",
			args: args{
				t: "{.Item}uint32",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isCrossCell(tt.args.t); got != tt.want {
				t.Errorf("isCrossCell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_genMetasheet(t *testing.T) {
	metaDoc := fmt.Sprintf(`
<?xml version='1.0' encoding='UTF-8'?>
<%v>
	<Item Sheet="ServerConf" Sep="|" />
</%v>
`, atTableauDisplacement, atTableauDisplacement)

	metaRoot, _ := xmlquery.Parse(strings.NewReader(metaDoc))
	node := xmlquery.FindOne(metaRoot, atTableauDisplacement)

	type args struct {
		tableauNode *xmlquery.Node
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]map[string]string
		want1   *book.Sheet
		wantErr bool
	}{
		{
			name: "Common Case",
			args: args{
				tableauNode: node,
			},
			want: map[string]map[string]string{
				"ServerConf": {
					"Nested":   "true",
					"Sheet":    "ServerConf",
					"Sep":      "|",
					"Nameline": "1",
					"Typeline": "1",
				},
			},
			want1: &book.Sheet{
				Name:   book.MetasheetName,
				MaxRow: 2,
				MaxCol: 5,
				Rows: [][]string{
					{
						"Nameline",
						"Nested",
						"Sep",
						"Sheet",
						"Typeline",
					},
					{
						"1",
						"true",
						"|",
						"ServerConf",
						"1",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := genMetasheet(tt.args.tableauNode)
			if (err != nil) != tt.wantErr {
				t.Errorf("genMetasheet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("genMetasheet() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("genMetasheet() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}

func Test_addMetaNodeAttr(t *testing.T) {
	type args struct {
		attrMap *tableaupb.XMLNode_AttrMap
		name    string
		val     string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Common Case",
			args: args{
				attrMap: newOrderedAttrMap(),
				name:    "Num",
				val:     "int32",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addMetaNodeAttr(tt.args.attrMap, tt.args.name, tt.args.val)
		})
	}
}

func Test_addDataNodeAttr(t *testing.T) {
	type args struct {
		metaMap *tableaupb.XMLNode_AttrMap
		dataMap *tableaupb.XMLNode_AttrMap
		name    string
		val     string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Common Case",
			args: args{
				metaMap: newOrderedAttrMap(),
				dataMap: newOrderedAttrMap(),
				name:    "Num",
				val:     "100",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addDataNodeAttr(tt.args.metaMap, tt.args.dataMap, tt.args.name, tt.args.val)
		})
	}
}

func Test_readXMLFile(t *testing.T) {
	// 	doc := `
	// <?xml version='1.0' encoding='UTF-8'?>
	// <!--
	// <@TABLEAU>
	// 	<Item Sheet="Server" />
	// </@TABLEAU>

	// <Server>
	// 	<Weight Num="map&lt;uint32,Weight&gt;"/>
	// </Server>
	// -->

	// <Server>
	// 	<Weight Num="1"/>
	// 	<Weight Num="2"/>
	// </Server>
	// `
	// 		root, _ := xmlquery.Parse(strings.NewReader(doc))
	// 		newBook := book.NewBook(`Test.xml`, `Test.xml`, nil)

	type args struct {
		metasheet, content string
		newBook            *book.Book
	}
	tests := []struct {
		name    string
		args    args
		want    *tableaupb.XMLBook
		wantErr bool
	}{
		// TODO:
		// {
		// 	name: "MapConf",
		// 	args: args{
		// 		root: root,
		// 		newBook: newBook,
		// 	},
		// 	want: &tableaupb.XMLBook{
		// 		SheetMap: map[string]int32{
		// 			"Server": 0,
		// 		},
		// 		SheetList: []*tableaupb.XMLSheet{
		// 			{
		// 				Meta: &tableaupb.XMLNode{
		// 					Name: "Server",
		// 					Path: "Server",
		// 					AttrMap: newOrderedAttrMap(),
		// 					ChildMap: map[string]*tableaupb.XMLNode_IndexList{
		// 						"Weight": {
		// 							Indexes: []int32{
		// 								0,
		// 							},
		// 						},
		// 					},
		// 					ChildList: []*tableaupb.XMLNode{
		// 						{
		// 							Name: "Weight",
		// 							AttrMap: &tableaupb.XMLNode_AttrMap{
		// 								Map: map[string]int32{
		// 									"Num": 0,
		// 								},
		// 								List: []*tableaupb.XMLNode_AttrMap_Attr{
		// 									{
		// 										Name: "Num",
		// 										Value: "map<uint32,Weight>",
		// 									},
		// 								},
		// 							},
		// 						},
		// 					},
		// 				},
		// 			},
		// 		},
		// 	},
		// },
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readXMLFile(tt.args.metasheet, tt.args.content, tt.args.newBook)
			if (err != nil) != tt.wantErr {
				t.Errorf("readXMLFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readXMLFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_hasChild(t *testing.T) {
	doc := `
<Conf>
	<Client Open="true">
		<Num>100</Num>	
		<Item ID="123" />
	</Client>
	
	<Server>
		<!-- Comments -->
	
	</Server>
</Conf>
`
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	node1 := xmlquery.FindOne(root, "Conf/Client")
	node2 := xmlquery.FindOne(root, "Conf/Client/Num")
	node3 := xmlquery.FindOne(root, "Conf/Client/Item")
	node4 := xmlquery.FindOne(root, "Conf/Server")

	type args struct {
		n *xmlquery.Node
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Conf/Client",
			args: args{
				n: node1,
			},
			want: true,
		},
		{
			name: "Conf/Client/Num",
			args: args{
				n: node2,
			},
			want: false,
		},
		{
			name: "Conf/Client/Item",
			args: args{
				n: node3,
			},
			want: false,
		},
		{
			name: "Conf/Server",
			args: args{
				n: node4,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasChild(tt.args.n); got != tt.want {
				t.Errorf("hasChild() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getTextContent(t *testing.T) {
	doc := `
<Conf>
	<Client Open="true">
		<Num>100</Num>	
		<Item ID="123" />
	</Client>
	
	<Server>
		<!-- Comments -->
	
	</Server>
</Conf>
`
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	node1 := xmlquery.FindOne(root, "Conf/Client")
	node2 := xmlquery.FindOne(root, "Conf/Client/Num")
	node3 := xmlquery.FindOne(root, "Conf/Client/Item")
	node4 := xmlquery.FindOne(root, "Conf/Server")

	type args struct {
		n *xmlquery.Node
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Conf/Client",
			args: args{
				n: node1,
			},
			want: "",
		},
		{
			name: "Conf/Client/Num",
			args: args{
				n: node2,
			},
			want: "100",
		},
		{
			name: "Conf/Client/Item",
			args: args{
				n: node3,
			},
			want: "",
		},
		{
			name: "Conf/Server",
			args: args{
				n: node4,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getTextContent(tt.args.n); got != tt.want {
				t.Errorf("getTextContent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_matchMetasheet(t *testing.T) {
	doc := `
<?xml version='1.0' encoding='UTF-8'?>
<!--
<@TABLEAU>
	<Item Sheet="Server" />
</@TABLEAU>

<Server>
	<Weight Num="map<uint32, Weight>"/>
</Server>
-->

<!--
		Comments
		Comments
		Comments	
-->

<Server>
	<Weight Num="1"/>
	<Weight Num="2"/>
</Server>
`
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "Whole document",
			args: args{
				s: doc,
			},
			want: []string{
				`<!--
<@TABLEAU>
	<Item Sheet="Server" />
</@TABLEAU>

<Server>
	<Weight Num="map<uint32, Weight>"/>
</Server>
-->`,
				`<@TABLEAU>
	<Item Sheet="Server" />
</@TABLEAU>

<Server>
	<Weight Num="map<uint32, Weight>"/>
</Server>
`,
				`>
	<Item Sheet="Server" />
</@TABLEAU>`,
				`
	<Item Sheet="Server" />
`,
				` Sheet="Server"`,
				`</Server>
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMetasheet(tt.args.s); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchMetasheet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_splitRawXML(t *testing.T) {
	doc := `<?xml version='1.0' encoding='UTF-8'?>
<!--
<@TABLEAU>
	<Item Sheet="Server" />
</@TABLEAU>

<Server>
	<Weight Num="map<uint32, Weight>"/>
</Server>
-->

<Server>
	<Weight Num="1"/>
	<Weight Num="2"/>
</Server>`

	type args struct {
		rawXML string
	}
	tests := []struct {
		name          string
		args          args
		wantMetasheet string
		wantContent   string
	}{
		{
			name: "Whole document",
			args: args{
				rawXML: doc,
			},
			wantMetasheet: `<?xml version='1.0' encoding='UTF-8'?>
<ATABLEAU>
	<Item Sheet="Server" />
</ATABLEAU>

<Server>
	<Weight Num="map&lt;uint32, Weight&gt;"/>
</Server>
`,
			wantContent: `<?xml version='1.0' encoding='UTF-8'?>


<Server>
	<Weight Num="1"/>
	<Weight Num="2"/>
</Server>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMetasheet, gotContent := splitRawXML(tt.args.rawXML)
			if gotMetasheet != tt.wantMetasheet {
				t.Errorf("splitRawXML() gotMetasheet = %v, want %v", gotMetasheet, tt.wantMetasheet)
			}
			if gotContent != tt.wantContent {
				t.Errorf("splitRawXML() gotContent = %v, want %v", gotContent, tt.wantContent)
			}
		})
	}
}

func Test_matchSheetBlock(t *testing.T) {
	doc := `<?xml version='1.0' encoding='UTF-8'?>

<Server>
	<Weight Num="1"/>
	{{ if a == 1 }}
	<Weight Num="2">
	{{ else }}
	<Weight Num="3">
	{{ endif }}
		<Param value="1" />
	</Weight>
</Server>

<Client>
	<Weight Num="1"/>
</Client>`

	type args struct {
		xml       string
		sheetName string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "General",
			args: args{
				xml:       doc,
				sheetName: "Server",
			},
			want: []string{
				`<Server>
	<Weight Num="1"/>
	{{ if a == 1 }}
	<Weight Num="2">
	{{ else }}
	<Weight Num="3">
	{{ endif }}
		<Param value="1" />
	</Weight>
</Server>`,
				`>
	<Weight Num="1"/>
	{{ if a == 1 }}
	<Weight Num="2">
	{{ else }}
	<Weight Num="3">
	{{ endif }}
		<Param value="1" />
	</Weight>
</Server>`,
				`	</Weight>
`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchSheetBlock(tt.args.xml, tt.args.sheetName); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("matchSheetBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
