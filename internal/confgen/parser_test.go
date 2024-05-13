package confgen

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
)

var testParser *sheetParser

func init() {
	testParser = NewSheetParser("protoconf", "Asia/Shanghai", book.MetasheetOptions())
}

func TestParser_parseVerticalMapWithDuplicateKey(t *testing.T) {
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
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "duplicate shop",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "duplicate goods",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "duplicate shop and goods",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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

func TestParser_parseVerticalMapWithEmptyKey(t *testing.T) {
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
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "one empty key",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "multiple empty keys",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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

func TestParser_parseVerticalMapWithEmptyRow(t *testing.T) {
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
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "one empty row",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "multiple empty rows",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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
			wantErr: false,
		},
		{
			name:   "empty key with empty row",
			parser: testParser,
			args: args{
				sheet: &book.Sheet{
					Name:   "MallConf",
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

func TestParser_parseHorizonalMapWithDuplicateKey(t *testing.T) {
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
					Name:   "RewardConf",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "2", "20"},
						{"2", "1", "10", "2", "20"},
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
					Name:   "RewardConf",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "1", "20"},
						{"2", "1", "10", "2", "20"},
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

func TestParser_parseHorizonalMapWithEmptyKey(t *testing.T) {
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
					Name:   "RewardConf",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "2", "20"},
						{"2", "1", "10", "2", "20"},
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
					Name:   "RewardConf",
					MaxRow: 3,
					MaxCol: 5,
					Rows: [][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num"},
						{"1", "1", "10", "", "20"},
						{"2", "1", "10", "2", "20"},
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
					Name:   "RewardConf",
					MaxRow: 3,
					MaxCol: 7,
					Rows: [][]string{
						{"RewardID", "Item1ID", "Item1Num", "Item2ID", "Item2Num", "Item3ID", "Item3Num"},
						{"1", "1", "10", "", "20", "", "30"},
						{"2", "1", "10", "2", "20", "3", "30"},
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
