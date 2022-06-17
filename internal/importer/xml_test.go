package importer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
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
		// TODO: Add test cases.
		{
			name: "standard",
			args: args{
				doc: `
<Conf>
    <Server Type="map<enum<.ServerType>, Server>" Value="int32"/>
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
<!-- @TABLEAU -->
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
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	xmlMeta, _, _ := readXMLFile(root, nil)
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
		// TODO: Add test cases.
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
		// TODO: Add test cases.
		{
			name: "scalar type",
			args: args{
				s: `<AAA bb="bool" cc="int64" dd="enum<.EnumType>" >`,
			},
			want: []string{
				`bb="bool"`, `bb`, `bool`, ``,
			},
		},
		// TODO: Add test cases.
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
<!-- @TABLEAU
<Server>
    <MapConf>
        <Weight Num="map&lt;uint32,Weight&gt;"/>
    </MapConf>
</Server>
-->
`
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	xmlMeta, _, _ := readXMLFile(root, nil)
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
		// TODO: Add test cases.
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
<!-- @TABLEAU
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
	root, _ := xmlquery.Parse(strings.NewReader(doc))
	xmlMeta, _, _ := readXMLFile(root, nil)
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
		// TODO: Add test cases.
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
			want: `{MapConf}[]<int64>`,
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
		// TODO: Add test cases.
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
		// TODO: Add test cases.
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
