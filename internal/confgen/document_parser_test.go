package confgen

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/format"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/proto/tableaupb/unittestpb"
	"github.com/tableauio/tableau/xerrors"
)

var docTestParser *sheetParser

func init() {
	docTestParser = NewExtendedSheetParser("protoconf", "Asia/Shanghai",
		book.MetabookOptions(),
		book.MetasheetOptions(),
		&SheetParserExtInfo{
			InputDir:       "",
			SubdirRewrites: map[string]string{},
			BookFormat:     format.YAML,
		})
}

func TestDocParser_parseFieldNotFound(t *testing.T) {
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
			parser: docTestParser,
			args: args{
				sheet: &book.Sheet{
					Name: "YamlScalarConf",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "YamlScalarConf",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Name: "",
								Children: []*book.Node{
									{
										Name:  "ID",
										Value: "1",
									},
									{
										Name:  "Num",
										Value: "10",
									},
									{
										Name:  "Value",
										Value: "20",
									},
									{
										Name:  "Weight",
										Value: "30",
									},
									{
										Name:  "Percentage",
										Value: "0.5",
									},
									{
										Name:  "Ratio",
										Value: "1.5",
									},
									{
										Name:  "Name",
										Value: "Apple",
									},
									{
										Name:  "Blob",
										Value: "VGFibGVhdQ==",
									},
									{
										Name:  "OK",
										Value: "true",
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "field-not-found",
			parser: docTestParser,
			args: args{
				sheet: &book.Sheet{
					Name: "YamlScalarConf",
					Document: &book.Node{
						Kind: book.DocumentNode,
						Name: "YamlScalarConf",
						Children: []*book.Node{
							{
								Kind: book.MapNode,
								Name: "",
								Children: []*book.Node{
									{
										Name:  "ID",
										Value: "1",
									},
									{
										Name:  "Num",
										Value: "10",
									},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errcode: "E2014",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parser.Parse(&unittestpb.YamlScalarConf{}, tt.args.sheet)
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
