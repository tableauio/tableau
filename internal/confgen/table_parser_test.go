package confgen

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/metasheet"
	"github.com/tableauio/tableau/proto/tableaupb"
	"github.com/tableauio/tableau/proto/tableaupb/internalpb"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
	"google.golang.org/protobuf/proto"
)

func newTableParserForTest() *sheetParser {
	return NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
		book.MetabookOptions(),
		book.MetasheetOptions(context.Background()),
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"2", "2", "20"},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate shop",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"1", "2", "20"},
						{"1", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate goods",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"2", "1", "20"},
						{"3", "1", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate shop and goods",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"1", "1", "20"},
						{"1", "1", "30"},
					}),
			},
			wantErr: true,
			errcode: "E2005",
		},
		{
			name:   "duplicate col name",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "GoodsID", "Price"},
						{"1", "1", "1", "10"},
						{"2", "2", "2", "20"},
						{"3", "3", "3", "30"},
					}),
			},
			wantErr: true,
			errcode: "E0003",
		},
		{
			name:   "duplicate empty col name",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "", "GoodsID", "", "Price"},
						{"1", "x", "1", "x", "10"},
						{"2", "x", "2", "x", "20"},
						{"3", "x", "3", "x", "30"},
					}),
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"2", "2", "20"},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "one empty key",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"", "1", "10"},
						{"2", "2", "20"},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "multiple empty keys",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"", "1", "10"},
						{"", "2", "20"},
						{"3", "3", "30"},
					}),
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"1", "1", "10"},
						{"2", "2", "20"},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "one empty row",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"", "", ""},
						{"2", "2", "20"},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "multiple empty rows",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"", "", ""},
						{"", "", ""},
						{"3", "3", "30"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "empty key with empty row",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"MallConf",
					[][]string{
						{"ShopID", "GoodsID", "Price"},
						{"", "1", "10"},
						{"", "", ""},
						{"", "", ""},
					}),
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"RewardConf",
					[][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "2", "20"},
						{"2", "1", "10", "2", "20"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate item",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"RewardConf",
					[][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "1", "20"},
						{"2", "1", "10", "2", "20"},
					}),
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"RewardConf",
					[][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "2", "20"},
						{"2", "1", "10", "2", "20"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "one empty key",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"RewardConf",
					[][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "", "20"},
						{"2", "1", "10", "2", "20"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "multiple empty keys",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"RewardConf",
					[][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num", "Item3ID", "Item3Num"},
						{"1", "1", "10", "", "20", "", "30"},
						{"2", "1", "10", "2", "20", "3", "30"},
					}),
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
	type args struct {
		path string
	}
	tests := []struct {
		name    string
		parser  *sheetParser
		args    args
		wantErr bool
		wantMsg proto.Message
	}{
		{
			name: "parse yaml metasheet",
			parser: NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
				book.MetabookOptions(),
				book.MetasheetOptions(context.Background()),
				&SheetParserExtInfo{
					InputDir:       "",
					SubdirRewrites: map[string]string{},
					BookFormat:     format.YAML,
				}),
			args:    args{path: "./testdata/Metasheet.yaml"},
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
		{
			name: "parse xml metasheet",
			parser: NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
				book.MetabookOptions(),
				book.MetasheetOptions(context.Background()),
				&SheetParserExtInfo{
					InputDir:       "",
					SubdirRewrites: map[string]string{},
					BookFormat:     format.XML,
				}),
			args:    args{path: "./testdata/Metasheet.xml"},
			wantErr: false,
			wantMsg: &internalpb.Metabook{
				MetasheetMap: map[string]*internalpb.Metasheet{
					"ItemConf": {
						Sheet:      "ItemConf",
						OrderedMap: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp, err := importer.New(context.Background(), tt.args.path, importer.Parser(tt.parser))
			if err != nil {
				t.Fatal(err)
			}
			sheet := imp.GetSheet(metasheet.DefaultMetasheetName)
			if sheet == nil {
				t.Fatalf("metasheet not found")
			}
			msg := &internalpb.Metabook{}
			err = tt.parser.Parse(msg, sheet)
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

func TestTableParser_parseWithSheetAndBookSep(t *testing.T) {
	parserWithBookSep := NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
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

	parserWithSheetAndBookSep := NewExtendedSheetParser(context.Background(), "protoconf", "Asia/Shanghai",
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
				sheet: book.NewTableSheet(
					"SimpleIncellMap",
					[][]string{
						{"Item"},
						{"1:10,2:20,3:30"},
						{"4:40,5:50"},
					}),
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
				sheet: book.NewTableSheet(
					"SimpleIncellMap",
					[][]string{
						{"Item"},
						{"1=10;2=20;3=30"},
						{"4=40;5=50"},
					}),
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
				sheet: book.NewTableSheet(
					"IncellStructList",
					[][]string{
						{"Item"},
						{"1:10,2:20,3:30"},
						{"4:40,5:50"},
					}),
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
				sheet: book.NewTableSheet(
					"IncellStructList",
					[][]string{
						{"Item"},
						{"1=10;2=20;3=30"},
						{"4=40;5=50"},
					}),
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

func TestTableParser_parseVerticalUniqueFieldStructList(t *testing.T) {
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"UniqueFieldInVerticalStructList",
					[][]string{
						{"ID", "Name", "Desc"},
						{"1", "Apple", "A kind of delicious fruit."},
						{"2", "Orange", "A kind of sour fruit."},
						{"3", "Banana", "A kind of calorie-rich fruit."},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate id",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"UniqueFieldInVerticalStructList",
					[][]string{
						{"ID", "Name", "Desc"},
						{"1", "Apple", "A kind of delicious fruit."},
						{"1", "Orange", "A kind of sour fruit."},
						{"3", "Banana", "A kind of calorie-rich fruit."},
					}),
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name:   "duplicate name",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"UniqueFieldInVerticalStructList",
					[][]string{
						{"ID", "Name", "Desc"},
						{"1", "Apple", "A kind of delicious fruit."},
						{"2", "Banana", "A kind of sour fruit."},
						{"3", "Banana", "A kind of calorie-rich fruit."},
					}),
			},
			wantErr: true,
			errcode: "E2022",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.UniqueFieldInVerticalStructList{}, tt.args.sheet)
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

func TestTableParser_parseVerticalUniqueFieldStructMap(t *testing.T) {
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
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"VerticalUniqueFieldStructMap",
					[][]string{
						{"MainID", "MainName", "SubID", "SubName"},
						{"1001", "BackPack", "1", "Gold"},
						{"1001", "", "2", "Diamond"},
						{"1001", "", "3", "Ticket"},
						{"1001", "", "4", "Point"},
						{"1002", "Equip", "1", "Weapon"},
						{"1002", "", "2", "Gold"},
					}),
			},
			wantErr: false,
		},
		{
			name:   "duplicate main name",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"VerticalUniqueFieldStructMap",
					[][]string{
						{"MainID", "MainName", "SubID", "SubName"},
						{"1001", "BackPack", "1", "Gold"},
						{"1001", "", "2", "Diamond"},
						{"1001", "", "3", "Ticket"},
						{"1001", "", "4", "Point"},
						{"1002", "BackPack", "1", "Weapon"},
						{"1002", "", "2", "Gold"},
					}),
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name:   "duplicate sub name",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"VerticalUniqueFieldStructMap",
					[][]string{
						{"MainID", "MainName", "SubID", "SubName"},
						{"1001", "BackPack", "1", "Gold"},
						{"1001", "", "2", "Diamond"},
						{"1001", "", "3", "Ticket"},
						{"1001", "", "4", "Ticket"},
						{"1002", "Equip", "1", "Weapon"},
						{"1002", "", "2", "Gold"},
					}),
			},
			wantErr: true,
			errcode: "E2022",
		},
		{
			name:   "duplicate sub id",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"VerticalUniqueFieldStructMap",
					[][]string{
						{"MainID", "MainName", "SubID", "SubName"},
						{"1001", "BackPack", "1", "Gold"},
						{"1001", "", "2", "Diamond"},
						{"1001", "", "3", "Ticket"},
						{"1001", "", "3", "Point"},
						{"1002", "Equip", "1", "Weapon"},
						{"1002", "", "2", "Gold"},
					}),
			},
			wantErr: true,
			errcode: "E2005",
		},
		{
			name:   "duplicate incell map key",
			parser: newTableParserForTest(),
			args: args{
				sheet: book.NewTableSheet(
					"VerticalUniqueFieldStructMap",
					[][]string{
						{"MainID", "MainName", "MainKV", "SubID", "SubName"},
						{"1001", "BackPack", "1:10,2:20,2:30", "1", "Gold"},
					}),
			},
			wantErr: true,
			errcode: "E2005",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.VerticalUniqueFieldStructMap{}, tt.args.sheet)
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
