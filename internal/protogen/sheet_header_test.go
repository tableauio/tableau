package protogen

import (
	"testing"

	"github.com/tableauio/tableau/proto/tableaupb"
)

var testSheetHeader *sheetHeader

func init() {
	testSheetHeader = &sheetHeader{
		meta: &tableaupb.Metasheet{
			Namerow: 1,
			Typerow: 2,
			Noterow: 3,
		},
		namerow: []string{"ID", "Value", "", "Kind"},
		typerow: []string{"map<int32, Item>", "int32", "", "int32"},
		noterow: []string{"Item's ID", "Item's value", "", "Item's kind"},
	}
}

func Test_sheetHeader_getValidNameCell(t *testing.T) {
	cursor1 := 1
	cursor2 := 2
	type args struct {
		cursor *int
	}
	tests := []struct {
		name string
		sh   *sheetHeader
		args args
		want string
	}{
		{
			name: "cursor-1",
			sh:   testSheetHeader,
			args: args{
				cursor: &cursor1,
			},
			want: "Value",
		},
		{
			name: "cursor-2-empty-cell",
			sh:   testSheetHeader,
			args: args{
				cursor: &cursor2,
			},
			want: "Kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sh.getValidNameCell(tt.args.cursor); got != tt.want {
				t.Errorf("sheetHeader.getValidNameCell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sheetHeader_getNameCell(t *testing.T) {
	type args struct {
		cursor int
	}
	tests := []struct {
		name string
		sh   *sheetHeader
		args args
		want string
	}{
		{
			name: "cursor-1",
			sh:   testSheetHeader,
			args: args{
				cursor: 1,
			},
			want: "Value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sh.getNameCell(tt.args.cursor); got != tt.want {
				t.Errorf("sheetHeader.getNameCell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sheetHeader_getTypeCell(t *testing.T) {
	type args struct {
		cursor int
	}
	tests := []struct {
		name string
		sh   *sheetHeader
		args args
		want string
	}{
		{
			name: "cursor-0",
			sh:   testSheetHeader,
			args: args{
				cursor: 0,
			},
			want: "map<int32, Item>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sh.getTypeCell(tt.args.cursor); got != tt.want {
				t.Errorf("sheetHeader.getTypeCell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_sheetHeader_getNoteCell(t *testing.T) {
	type args struct {
		cursor int
	}
	tests := []struct {
		name string
		sh   *sheetHeader
		args args
		want string
	}{
		{
			name: "cursor-3",
			sh:   testSheetHeader,
			args: args{
				cursor: 3,
			},
			want: "Item's kind",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.sh.getNoteCell(tt.args.cursor); got != tt.want {
				t.Errorf("sheetHeader.getNoteCell() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getCell(t *testing.T) {
	type args struct {
		row    []string
		cursor int
		line   int32
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "first-line",
			args: args{
				row:    []string{"11", "12", "13"},
				cursor: 1,
				line:   0,
			},
			want: "12",
		},
		{
			name: "second-line",
			args: args{
				row:    []string{"11\n21", "12\n22", "13\n23"},
				cursor: 1,
				line:   2,
			},
			want: "22",
		},
		{
			name: "not-found",
			args: args{
				row:    []string{"11\n21", "12\n22", "13\n23"},
				cursor: 4,
				line:   2,
			},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCell(tt.args.row, tt.args.cursor, tt.args.line); got != tt.want {
				t.Errorf("getCell() = %v, want %v", got, tt.want)
			}
		})
	}
}
