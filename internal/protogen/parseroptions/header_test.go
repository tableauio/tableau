package parseroptions

import (
	"reflect"
	"testing"

	"github.com/tableauio/tableau/options"
	"github.com/tableauio/tableau/proto/tableaupb"
)

func TestMergeHeader(t *testing.T) {
	type args struct {
		sheetOpts  *tableaupb.WorksheetOptions
		bookOpts   *tableaupb.WorkbookOptions
		globalOpts *options.HeaderOption
	}
	tests := []struct {
		name string
		args args
		want *Header
	}{
		{
			name: "global",
			args: args{
				sheetOpts: nil,
				bookOpts:  nil,
				globalOpts: &options.HeaderOption{
					NameRow:  100,
					TypeRow:  200,
					NoteRow:  300,
					DataRow:  400,
					NameLine: 100,
					TypeLine: 200,
					NoteLine: 300,
					Sep:      "global-sep",
					Subsep:   "global-subsep",
				},
			},
			want: &Header{
				NameRow:  100,
				TypeRow:  200,
				NoteRow:  300,
				DataRow:  400,
				NameLine: 100,
				TypeLine: 200,
				NoteLine: 300,
				Sep:      "global-sep",
				Subsep:   "global-subsep",
			},
		},
		{
			name: "book",
			args: args{
				sheetOpts: nil,
				bookOpts: &tableaupb.WorkbookOptions{
					Namerow:  10,
					Typerow:  20,
					Noterow:  30,
					Datarow:  40,
					Nameline: 10,
					Typeline: 20,
					Noteline: 30,
					Sep:      "book-sep",
					Subsep:   "book-subsep",
				},
				globalOpts: nil,
			},
			want: &Header{
				NameRow:  10,
				TypeRow:  20,
				NoteRow:  30,
				DataRow:  40,
				NameLine: 10,
				TypeLine: 20,
				NoteLine: 30,
				Sep:      "book-sep",
				Subsep:   "book-subsep",
			},
		},
		{
			name: "book: with global",
			args: args{
				sheetOpts: nil,
				bookOpts: &tableaupb.WorkbookOptions{
					Namerow:  10,
					Typerow:  20,
					Noterow:  30,
					Datarow:  40,
					Nameline: 10,
					Typeline: 20,
					Noteline: 30,
					Sep:      "book-sep",
					Subsep:   "book-subsep",
				},
				globalOpts: &options.HeaderOption{
					NameRow:  100,
					TypeRow:  200,
					NoteRow:  300,
					DataRow:  400,
					NameLine: 100,
					TypeLine: 200,
					NoteLine: 300,
					Sep:      "global-sep",
					Subsep:   "global-subsep",
				},
			},
			want: &Header{
				NameRow:  10,
				TypeRow:  20,
				NoteRow:  30,
				DataRow:  40,
				NameLine: 10,
				TypeLine: 20,
				NoteLine: 30,
				Sep:      "book-sep",
				Subsep:   "book-subsep",
			},
		},
		{
			name: "sheet",
			args: args{
				sheetOpts: &tableaupb.WorksheetOptions{
					Namerow:  1,
					Typerow:  2,
					Noterow:  3,
					Datarow:  4,
					Nameline: 1,
					Typeline: 2,
					Noteline: 3,
					Sep:      "sheet-sep",
					Subsep:   "sheet-subsep",
				},
				bookOpts:   nil,
				globalOpts: nil,
			},
			want: &Header{
				NameRow:  1,
				TypeRow:  2,
				NoteRow:  3,
				DataRow:  4,
				NameLine: 1,
				TypeLine: 2,
				NoteLine: 3,
				Sep:      "sheet-sep",
				Subsep:   "sheet-subsep",
			},
		},
		{
			name: "sheet: with book",
			args: args{
				sheetOpts: &tableaupb.WorksheetOptions{
					Namerow:  1,
					Typerow:  2,
					Noterow:  3,
					Datarow:  4,
					Nameline: 1,
					Typeline: 2,
					Noteline: 3,
					Sep:      "sheet-sep",
					Subsep:   "sheet-subsep",
				},
				bookOpts: &tableaupb.WorkbookOptions{
					Namerow:  10,
					Typerow:  20,
					Noterow:  30,
					Datarow:  40,
					Nameline: 10,
					Typeline: 20,
					Noteline: 30,
					Sep:      "book-sep",
					Subsep:   "book-subsep",
				},
				globalOpts: nil,
			},
			want: &Header{
				NameRow:  1,
				TypeRow:  2,
				NoteRow:  3,
				DataRow:  4,
				NameLine: 1,
				TypeLine: 2,
				NoteLine: 3,
				Sep:      "sheet-sep",
				Subsep:   "sheet-subsep",
			},
		},
		{
			name: "sheet: with book and global",
			args: args{
				sheetOpts: &tableaupb.WorksheetOptions{
					Namerow:  1,
					Typerow:  2,
					Noterow:  3,
					Datarow:  4,
					Nameline: 1,
					Typeline: 2,
					Noteline: 3,
					Sep:      "sheet-sep",
					Subsep:   "sheet-subsep",
				},
				bookOpts: &tableaupb.WorkbookOptions{
					Namerow:  1,
					Typerow:  2,
					Noterow:  3,
					Datarow:  4,
					Nameline: 1,
					Typeline: 2,
					Noteline: 3,
					Sep:      "book-sep",
					Subsep:   "book-subsep",
				},
				globalOpts: &options.HeaderOption{
					NameRow:  100,
					TypeRow:  200,
					NoteRow:  300,
					DataRow:  400,
					NameLine: 100,
					TypeLine: 200,
					NoteLine: 300,
					Sep:      "global-sep",
					Subsep:   "global-subsep",
				},
			},
			want: &Header{
				NameRow:  1,
				TypeRow:  2,
				NoteRow:  3,
				DataRow:  4,
				NameLine: 1,
				TypeLine: 2,
				NoteLine: 3,
				Sep:      "sheet-sep",
				Subsep:   "sheet-subsep",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergeHeader(tt.args.sheetOpts, tt.args.bookOpts, tt.args.globalOpts); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeHeader() = %v, want %v", got, tt.want)
			}
		})
	}
}
