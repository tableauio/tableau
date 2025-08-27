package importer

import (
	"context"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/xerrors"
)

func TestNewExcelImporter(t *testing.T) {
	type args struct {
		ctx        context.Context
		filename   string
		sheetNames []string
		parser     book.SheetParser
		mode       ImporterMode
		cloned     bool
	}
	tests := []struct {
		name       string
		args       args
		wantSheets []*book.Sheet
		wantErr    bool
		errcode    string
	}{
		{
			name: "normal",
			args: args{
				ctx:        context.Background(),
				filename:   "testdata/Test.xlsx",
				sheetNames: []string{"Item"},
			},
			wantSheets: []*book.Sheet{
				book.NewTableSheet("Item", [][]string{
					{"ID", "Name"},
					{"1", "Pike"},
					{"2", "Thompson"},
				}),
			},
			wantErr: false,
		},
		{
			name: "normal",
			args: args{
				ctx:      context.Background(),
				filename: "testdata/Test_NotFound.xlsx",
			},
			wantSheets: nil,
			wantErr:    true,
			errcode:    "E3002",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExcelImporter(tt.args.ctx, tt.args.filename, tt.args.sheetNames, tt.args.parser, tt.args.mode, tt.args.cloned)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewExcelImporter() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil {
				if !reflect.DeepEqual(got.GetSheets(), tt.wantSheets) {
					t.Errorf("NewExcelImporter() = %v, want %v", got.GetSheets(), tt.wantSheets)
				}
			} else {
				assert.Equal(t, xerrors.NewDesc(err).ErrCode(), tt.errcode)
			}
		})
	}
}
