package book

import (
	"testing"
)

var testTable, testTable2 *Table

func init() {
	testTable = NewTable([][]string{
		{"1", "2", "3"},
		{"11", "12", "13"},
		{},
		{"31", "32", "33"},
		{"41", "42"},
	})
	testTable2 = NewTable([][]string{
		{"1", "11", "", "31", "41"},
		{"2", "12", "", "32", "42"},
		{"3", "13", "", "33"},
	})
}
func TestTable_isRowEmpty(t *testing.T) {
	type args struct {
		row int
	}
	tests := []struct {
		name  string
		table *Table
		args  args
		want  bool
	}{
		{
			name:  "empty-row",
			table: testTable,
			args: args{
				row: 2,
			},
			want: true,
		},
		{
			name:  "not-found-empty-row",
			table: testTable,
			args: args{
				row: 999,
			},
			want: true,
		},
		{
			name:  "none-empty-row",
			table: testTable,
			args: args{
				row: 0,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.table.isRowEmpty(tt.args.row); got != tt.want {
				t.Errorf("Table.isRowEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_FindBlockEndRow(t *testing.T) {
	type args struct {
		startRow int
	}
	tests := []struct {
		name string
		tr   *Table
		args args
		want int
	}{
		{
			name: "find-block-end-row",
			tr:   testTable,
			args: args{
				startRow: 0,
			},
			want: 2,
		},
		{
			name: "start-row-is-empty",
			tr:   testTable,
			args: args{
				startRow: 2,
			},
			want: 2,
		},
		{
			name: "last-row-not-empty",
			tr:   testTable,
			args: args{
				startRow: 3,
			},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tr.FindBlockEndRow(tt.args.startRow); got != tt.want {
				t.Errorf("Table.FindBlockEndRow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_isColEmpty(t *testing.T) {
	type args struct {
		col int
	}
	tests := []struct {
		name  string
		table *Table
		args  args
		want  bool
	}{
		{
			name:  "empty-col",
			table: testTable2,
			args: args{
				col: 2,
			},
			want: true,
		},
		{
			name:  "not-found-empty-col",
			table: testTable2,
			args: args{
				col: 999,
			},
			want: true,
		},
		{
			name:  "none-empty-col",
			table: testTable2,
			args: args{
				col: 0,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.table.isColEmpty(tt.args.col); got != tt.want {
				t.Errorf("Table.isColEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_findBlockEndCol(t *testing.T) {
	type args struct {
		startCol int
	}
	tests := []struct {
		name string
		tr   *Table
		args args
		want int
	}{
		{
			name: "find-block-end-col",
			tr:   testTable2,
			args: args{
				startCol: 0,
			},
			want: 2,
		},
		{
			name: "start-col-is-empty",
			tr:   testTable2,
			args: args{
				startCol: 2,
			},
			want: 2,
		},
		{
			name: "last-col-not-empty",
			tr:   testTable2,
			args: args{
				startCol: 3,
			},
			want: 5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.tr.findBlockEndCol(tt.args.startCol); got != tt.want {
				t.Errorf("Table.findBlockEndCol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTable_Position(t *testing.T) {
	type args struct {
		row int
		col int
	}
	tests := []struct {
		name  string
		table Tabler
		args  args
		want  string
	}{
		{
			name:  "table",
			table: &Table{},
			args: args{
				row: 0,
				col: 4,
			},
			want: "E1",
		},
		{
			name:  "transposed table",
			table: &TransposedTable{},
			args: args{
				row: 0,
				col: 4,
			},
			want: "A5",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.table.Position(tt.args.row, tt.args.col)
			if got != tt.want {
				t.Errorf("Position() = %v, want %v", got, tt.want)
			}
		})
	}
}
