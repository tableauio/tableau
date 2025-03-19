package confgen

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

var testParser *sheetParser

func init() {
	testParser = NewExtendedSheetParser("protoconf", "Asia/Shanghai",
		book.MetabookOptions(),
		book.MetasheetOptions(),
		&SheetParserExtInfo{
			InputDir:       "",
			SubdirRewrites: map[string]string{},
			BookFormat:     format.CSV,
		})
}

func TestTableParser_parseVerticalMapWithDuplicateKey(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		errcode string
	}{
		{
			name:   "no duplicate key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"2", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate shop",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"1", "2", "20"},
							{"1", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate goods",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"2", "1", "20"},
							{"3", "1", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate shop and goods",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"1", "1", "20"},
							{"1", "1", "30"},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2005",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.MallConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
		})
	}
}

func TestTableParser_parseVerticalMapWithEmptyKey(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		errcode string
	}{
		{
			name:   "no empty key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"2", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "one empty key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"", "1", "10"},
							{"2", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "multiple empty keys",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"", "1", "10"},
							{"", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2017",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.MallConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
		})
	}
}

func TestTableParser_parseVerticalMapWithEmptyRow(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		errcode string
	}{
		{
			name:   "no empty row",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"1", "1", "10"},
							{"2", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "one empty row",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"", "", ""},
							{"2", "2", "20"},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "multiple empty rows",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"", "", ""},
							{"", "", ""},
							{"3", "3", "30"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "empty key with empty row",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "MallConf",
					Table: &book.Table{
						MaxRow: 4,
						MaxCol: 3,
						Rows: [][]string{
							{"ShopID", "GoodsID", "Price"},
							{"", "1", "10"},
							{"", "", ""},
							{"", "", ""},
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.MallConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
		})
	}
}

func TestTableParser_parseHorizontalMapWithDuplicateKey(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		errcode string
	}{
		{
			name:   "no duplicate key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "RewardConf",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
							{"1", "1", "10", "2", "20"},
							{"2", "1", "10", "2", "20"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "duplicate item",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "RewardConf",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
							{"1", "1", "10", "1", "20"},
							{"2", "1", "10", "2", "20"},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2005",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.RewardConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
		})
	}
}

func TestTableParser_parseHorizontalMapWithEmptyKey(t *testing.T) {
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		errcode string
	}{
		{
			name:   "no empty key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "RewardConf",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
							{"1", "1", "10", "2", "20"},
							{"2", "1", "10", "2", "20"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "one empty key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "RewardConf",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 5,
						Rows: [][]string{
							{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
							{"1", "1", "10", "", "20"},
							{"2", "1", "10", "2", "20"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "multiple empty keys",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name: "RewardConf",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 7,
						Rows: [][]string{
							{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num", "Item3ID", "Item3Num"},
							{"1", "1", "10", "", "20", "", "30"},
							{"2", "1", "10", "2", "20", "3", "30"},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2017",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.RewardConf{}, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				if tt.errcode != "" {
					desc := xerrors.NewDesc(err)
					require.Equal(t, tt.errcode, desc.ErrCode())
				}
			}
		})
	}
}

func TestTableParser_parseDocumentMetasheet(t *testing.T) {
	path := "./testdata/Metasheet.yaml"
	parser := NewExtendedSheetParser("protoconf", "Asia/Shanghai",
		book.MetabookOptions(),
		book.MetasheetOptions(),
		&SheetParserExtInfo{
			InputDir:       "",
			SubdirRewrites: map[string]string{},
			BookFormat:     format.YAML,
		})
	imp, err := importer.New(path, importer.Parser(parser))
	if err != nil {
		t.Fatal(err)
	}
	sheet := imp.GetSheet(book.MetasheetName)
	if sheet == nil {
		t.Fatalf("metasheet not found")
	}
	type args struct {
		sheet *book.Sheet
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		wantMsg proto.Message
	}{
		{
			name:    "parse document metasheet",
			parser:  parser,
			args:    args{sheet: sheet},
			wantErr: false,
			wantMsg: &internalpb.Metabook{
				MetasheetMap: map[string]*internalpb.Metasheet{
					"HeroConf": {
						Sheet: "HeroConf",
					},
					"ItemConf": {
						Sheet:      "ItemConf",
						Alias:      "ItemAliasConf",
						OrderedMap: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &internalpb.Metabook{}
			err := tt.parser.Parse(msg, tt.args.sheet)
			if (err != nil) != tt.wantErr {
				t.Errorf("sheetParser.Parse() error = %s, wantErr %v", xerrors.NewDesc(err), tt.wantErr)
			}
			fmt.Println("sheet:", sheet)
			fmt.Println("metabook:", msg)
			if !proto.Equal(msg, tt.wantMsg) {
				t.Errorf("\ngotMsg: %v\nwantMsg: %v", msg, tt.wantMsg)
			}
		})
	}
}

func TestParser_parseWithSheetAndBookSep(t *testing.T) {
	parserWithBookSep := NewExtendedSheetParser("protoconf", "Asia/Shanghai",
		&tableaupb.WorkbookOptions{
			Sep:    ",",
			Subsep: ":",
		},
		&tableaupb.WorksheetOptions{
			Namerow: 1,
			Datarow: 2,
		},
		&SheetParserExtInfo{
			SubdirRewrites: map[string]string{},
			BookFormat:     format.YAML,
		})

	parserWithSheetAndBookSep := NewExtendedSheetParser("protoconf", "Asia/Shanghai",
		&tableaupb.WorkbookOptions{
			Sep:    ",",
			Subsep: ":",
		},
		&tableaupb.WorksheetOptions{
			Namerow: 1,
			Datarow: 2,
			Sep:     ";",
			Subsep:  "=",
		},
		&SheetParserExtInfo{
			SubdirRewrites: map[string]string{},
			BookFormat:     format.YAML,
		})

	type args struct {
		sheet *book.Sheet
		msg   proto.Message
	}
	tests := []struct {
		name   string
		parser *sheetParser
		args   args
		want   proto.Message
	}{
		{
			name:   "incell map: book-level sep and subsep",
			parser: parserWithBookSep,
			args: args{
				sheet: &book.Sheet{
					Name: "SimpleIncellMap",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 1,
						Rows: [][]string{
							{"Item"},
							{"1:10,2:20,3:30"},
							{"4:40,5:50"},
						},
					},
				},
				msg: &unittestpb.SimpleIncellMap{},
			},
			want: &unittestpb.SimpleIncellMap{
				ItemMap: map[int32]int32{
					1: 10,
					2: 20,
					3: 30,
					4: 40,
					5: 50,
				},
			},
		},
		{
			name:   "incell map: sheet-level and book-level sep and subsep",
			parser: parserWithSheetAndBookSep,
			args: args{
				sheet: &book.Sheet{
					Name: "SimpleIncellMap",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 1,
						Rows: [][]string{
							{"Item"},
							{"1=10;2=20;3=30"},
							{"4=40;5=50"},
						},
					},
				},
				msg: &unittestpb.SimpleIncellMap{},
			},
			want: &unittestpb.SimpleIncellMap{
				ItemMap: map[int32]int32{
					1: 10,
					2: 20,
					3: 30,
					4: 40,
					5: 50,
				},
			},
		},
		{
			name:   "incell struct list: book-level sep and subsep",
			parser: parserWithBookSep,
			args: args{
				sheet: &book.Sheet{
					Name: "IncellStructList",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 1,
						Rows: [][]string{
							{"Item"},
							{"1:10,2:20,3:30"},
							{"4:40,5:50"},
						},
					},
				},
				msg: &unittestpb.IncellStructList{},
			},
			want: &unittestpb.IncellStructList{
				ItemList: []*unittestpb.Item{
					{Id: 1, Num: 10},
					{Id: 2, Num: 20},
					{Id: 3, Num: 30},
					{Id: 4, Num: 40},
					{Id: 5, Num: 50},
				},
			},
		},
		{
			name:   "incell struct list: sheet-level and book-level sep and subsep",
			parser: parserWithSheetAndBookSep,
			args: args{
				sheet: &book.Sheet{
					Name: "IncellStructList",
					Table: &book.Table{
						MaxRow: 3,
						MaxCol: 1,
						Rows: [][]string{
							{"Item"},
							{"1=10;2=20;3=30"},
							{"4=40;5=50"},
						},
					},
				},
				msg: &unittestpb.IncellStructList{},
			},
			want: &unittestpb.IncellStructList{
				ItemList: []*unittestpb.Item{
					{Id: 1, Num: 10},
					{Id: 2, Num: 20},
					{Id: 3, Num: 30},
					{Id: 4, Num: 40},
					{Id: 5, Num: 50},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(tt.args.msg, tt.args.sheet)
			assert.NoError(t, err)
			if !proto.Equal(tt.want, tt.args.msg) {
				t.Errorf("parser.parseWithSheetAndBookSep() = %v, want %v", tt.args.msg, tt.want)
			}
		})
	}
}
