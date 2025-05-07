package book

import (
	"testing"
)

var testTable *Table

func init() {
	testTable = NewTable([][]string{
		{"1", "2", "3"},
		{"11", "12", "13"},
		{},
		{"31", "32", "33"},
		{"41", "42"},
	})
}
func TestTable_IsRowEmpty(t *testing.T) {
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
			if got := tt.table.IsRowEmpty(tt.args.row); got != tt.want {
				t.Errorf("Table.IsRowEmpty() = %v, want %v", got, tt.want)
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
