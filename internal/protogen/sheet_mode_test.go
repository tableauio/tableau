package protogen

import (
	"testing"

	"github.com/tableauio/tableau/internal/importer/book"
)

func TestVerticalPositioner_Position(t *testing.T) {
	// Simulate a non-transposed struct type sheet:
	//      A(col0)  B(col1)
	// R1:  Name     Type       <- header (namerow=1)
	// R2:  Field0   int32      <- data row 0 (datarow=2)
	// R3:  Field1   string     <- data row 1
	// R4:  Field2   bool       <- data row 2
	// ...
	// R6:  Field4   float      <- data row 4
	table := book.NewTable([][]string{
		{"Name", "Type"},     // row 0 (header)
		{"Field0", "int32"},  // row 1 (data row 0)
		{"Field1", "string"}, // row 2 (data row 1)
		{"Field2", "bool"},   // row 3 (data row 2)
		{"Field3", "int64"},  // row 4 (data row 3)
		{"Field4", "float"},  // row 5 (data row 4)
	})

	p := &verticalPositioner{
		basePositioner: basePositioner{tabler: table},
		dataRow:        table.BeginRow() + 1, // datarow=2 (1-based), 0-based: 1
	}

	tests := []struct {
		name   string
		row    int // virtual header row index: 0=Name, 1=Type, 2=Note
		col    int // cursor (field index)
		expect string
	}{
		{
			name:   "NameRow-cursor0",
			row:    0,
			col:    0,
			expect: "A2", // dataRow(1)+cursor(0)=row1, colMap["Name"]=0=A => A2
		},
		{
			name:   "TypeRow-cursor0",
			row:    1,
			col:    0,
			expect: "B2", // dataRow(1)+cursor(0)=row1, colMap["Type"]=1=B => B2
		},
		{
			name:   "NameRow-cursor4",
			row:    0,
			col:    4,
			expect: "A6", // dataRow(1)+cursor(4)=row5, colMap["Name"]=0=A => A6
		},
		{
			name:   "TypeRow-cursor4",
			row:    1,
			col:    4,
			expect: "B6", // dataRow(1)+cursor(4)=row5, colMap["Type"]=1=B => B6
		},
		{
			name:   "NoteRow-missing-returns-empty",
			row:    2, // Note column does not exist in colMap
			col:    0,
			expect: "", // should return empty string
		},
		{
			name:   "negative-row-returns-empty",
			row:    -1, // e.g., NoteRow=0 means row-1=-1
			col:    0,
			expect: "", // should return empty string
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("verticalPositioner.Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestVerticalPositioner_Position_Disordered(t *testing.T) {
	// Simulate a non-transposed struct type sheet with disordered columns:
	//      A(col0)  B(col1)
	// R1:  Type     Name       <- header (columns swapped!)
	// R2:  int32    ID         <- data row 0
	// R3:  string   Name       <- data row 1
	table := book.NewTable([][]string{
		{"Type", "Name"},     // row 0 (header)
		{"int32", "ID"},      // row 1 (data row 0)
		{"string", "MyName"}, // row 2 (data row 1)
	})

	p := &verticalPositioner{
		basePositioner: basePositioner{tabler: table},
		dataRow:        table.BeginRow() + 1, // datarow=2 (1-based), 0-based: 1
	}

	tests := []struct {
		name   string
		row    int
		col    int
		expect string
	}{
		{
			name:   "NameRow-cursor0",
			row:    0,
			col:    0,
			expect: "B2", // dataRow(1)+cursor(0)=row1, colMap["Name"]=1=B => B2
		},
		{
			name:   "TypeRow-cursor0",
			row:    1,
			col:    0,
			expect: "A2", // dataRow(1)+cursor(0)=row1, colMap["Type"]=0=A => A2
		},
		{
			name:   "NameRow-cursor1",
			row:    0,
			col:    1,
			expect: "B3", // dataRow(1)+cursor(1)=row2, colMap["Name"]=1=B => B3
		},
		{
			name:   "TypeRow-cursor1",
			row:    1,
			col:    1,
			expect: "A3", // dataRow(1)+cursor(1)=row2, colMap["Type"]=0=A => A3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("verticalPositioner.Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestVerticalPositioner_Position_Transposed(t *testing.T) {
	// Simulate a transposed struct type sheet:
	// The underlying table is:
	//      A(col0)  B(col1)  C(col2)  D(col3)  E(col4)  F(col5)
	// R1:  Name     Field0   Field1   Field2   Field3   Field4
	// R2:  Type     int32    string   bool     int64    float
	//
	// After transposing, the virtual layout becomes:
	//      A(col0)  B(col1)
	// R1:  Name     Type
	// R2:  Field0   int32
	// R3:  Field1   string
	// ...
	// R6:  Field4   float
	//
	// But Position on TransposedTable swaps row/col back to original coordinates.
	table := book.NewTable([][]string{
		{"Name", "Field0", "Field1", "Field2", "Field3", "Field4"},
		{"Type", "int32", "string", "bool", "int64", "float"},
	})
	transposed := table.Transpose()

	p := &verticalPositioner{
		basePositioner: basePositioner{tabler: transposed},
		dataRow:        transposed.BeginRow() + 1, // datarow=2 (1-based), 0-based in transposed: 1
	}

	tests := []struct {
		name   string
		row    int
		col    int
		expect string
	}{
		{
			name:   "NameRow-cursor0",
			row:    0,
			col:    0,
			expect: "B1", // transposed.Position(1, 0) => table.Position(0, 1) => B1
		},
		{
			name:   "TypeRow-cursor0",
			row:    1,
			col:    0,
			expect: "B2", // transposed.Position(1, 1) => table.Position(1, 1) => B2
		},
		{
			name:   "NameRow-cursor4",
			row:    0,
			col:    4,
			expect: "F1", // transposed.Position(5, 0) => table.Position(0, 5) => F1
		},
		{
			name:   "TypeRow-cursor4",
			row:    1,
			col:    4,
			expect: "F2", // transposed.Position(5, 1) => table.Position(1, 5) => F2
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("verticalPositioner(transposed).Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestUnionFieldPositioner_Position(t *testing.T) {
	// Simulate a non-transposed union type sheet with ordered columns:
	//      A(col0)  B(col1)  C(col2)  D(col3)  E(col4)  F(col5)
	// R1:  Name     Alias    Type     Field1   Field2   Field3   <- header
	// R2:  Value0   Alias0   Type0    <cell>   <cell>   <cell>   <- value 0
	// R3:  Value1   Alias1   Type1    <cell>   <cell>   <cell>   <- value 1
	// ...
	// R228: Value226 Alias226 Type226 <cell>   <cell>   <cell>   <- value 226
	rows := make([][]string, 228)
	rows[0] = []string{"Name", "Alias", "Type", "Field1", "Field2", "Field3"} // header row
	table := book.NewTable(rows)

	tests := []struct {
		name     string
		valueRow int
		row      int // unused in unionFieldPositioner
		col      int // cursor (field index)
		expect   string
	}{
		{
			name:     "value0-field0",
			valueRow: 1, // datarow=2 (1-based), BeginRow(0)+1+0=1
			row:      0,
			col:      0,
			expect:   "D2", // col=0 -> "Field1" -> colMap["Field1"]=3 -> Position(1, 3) => D2
		},
		{
			name:     "value0-field2",
			valueRow: 1,
			row:      0,
			col:      2,
			expect:   "F2", // col=2 -> "Field3" -> colMap["Field3"]=5 -> Position(1, 5) => F2
		},
		{
			name:     "value226-field0-D228",
			valueRow: 227, // BeginRow(0)+1+226=227
			row:      0,
			col:      0,
			expect:   "D228", // col=0 -> "Field1" -> colMap["Field1"]=3 -> Position(227, 3) => D228
		},
		{
			name:     "value226-field1-E228",
			valueRow: 227,
			row:      0,
			col:      1,
			expect:   "E228", // col=1 -> "Field2" -> colMap["Field2"]=4 -> Position(227, 4) => E228
		},
		{
			name:     "row-param-ignored",
			valueRow: 227,
			row:      1, // different row param, should still produce same result
			col:      0,
			expect:   "D228", // row param is ignored, col=0 -> "Field1" -> Position(227, 3) => D228
		},
		{
			name:     "missing-field-returns-empty",
			valueRow: 1,
			row:      0,
			col:      3, // col=3 -> "Field4" -> not in colMap -> ""
			expect:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &unionFieldPositioner{
				basePositioner: basePositioner{tabler: table},
				valueRow:       tt.valueRow,
			}
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("unionFieldPositioner.Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestUnionFieldPositioner_Position_Disordered(t *testing.T) {
	// Simulate a non-transposed union type sheet with disordered columns:
	//      A(col0)  B(col1)  C(col2)  D(col3)  E(col4)
	// R1:  Field3   Name     Field2   Alias    Field1   <- header (disordered!)
	// R2:  <cell>   Value0   <cell>   Alias0   <cell>   <- value 0
	// R3:  <cell>   Value1   <cell>   Alias1   <cell>   <- value 1
	table := book.NewTable([][]string{
		{"Field3", "Name", "Field2", "Alias", "Field1"},
		{"cell00", "Value0", "cell01", "Alias0", "cell02"},
		{"cell10", "Value1", "cell11", "Alias1", "cell12"},
	})

	tests := []struct {
		name     string
		valueRow int
		row      int
		col      int
		expect   string
	}{
		{
			name:     "value0-field0-maps-to-Field1-col4",
			valueRow: 1,
			row:      0,
			col:      0,
			expect:   "E2", // col=0 -> "Field1" -> colMap["Field1"]=4 -> Position(1, 4) => E2
		},
		{
			name:     "value0-field1-maps-to-Field2-col2",
			valueRow: 1,
			row:      0,
			col:      1,
			expect:   "C2", // col=1 -> "Field2" -> colMap["Field2"]=2 -> Position(1, 2) => C2
		},
		{
			name:     "value0-field2-maps-to-Field3-col0",
			valueRow: 1,
			row:      0,
			col:      2,
			expect:   "A2", // col=2 -> "Field3" -> colMap["Field3"]=0 -> Position(1, 0) => A2
		},
		{
			name:     "value1-field0-maps-to-Field1-col4",
			valueRow: 2,
			row:      0,
			col:      0,
			expect:   "E3", // col=0 -> "Field1" -> colMap["Field1"]=4 -> Position(2, 4) => E3
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &unionFieldPositioner{
				basePositioner: basePositioner{tabler: table},
				valueRow:       tt.valueRow,
			}
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("unionFieldPositioner.Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestUnionFieldPositioner_Position_Transposed(t *testing.T) {
	// Simulate a transposed union type sheet:
	// The underlying table is:
	//      A(col0)  B(col1)  C(col2)  ...
	// R1:  Name     Value0   Value1   ...
	// R2:  Alias    Alias0   Alias1   ...
	// R3:  Type     Type0    Type1    ...
	// R4:  Field1   <cell>   <cell>   ...
	// R5:  Field2   <cell>   <cell>   ...
	//
	// After transposing, virtual layout:
	//      A       B       C       D       E
	// R1:  Name    Alias   Type    Field1  Field2
	// R2:  Value0  Alias0  Type0   <cell>  <cell>
	// R3:  Value1  Alias1  Type1   <cell>  <cell>
	//
	// TransposedTable.Position(row, col) => table.Position(col, row)

	table := book.NewTable([][]string{
		{"Name", "Value0", "Value1"},
		{"Alias", "Alias0", "Alias1"},
		{"Type", "Type0", "Type1"},
		{"Field1", "cell00", "cell10"},
		{"Field2", "cell01", "cell11"},
	})
	transposed := table.Transpose()

	tests := []struct {
		name     string
		valueRow int
		row      int
		col      int
		expect   string
	}{
		{
			name:     "value0-field0",
			valueRow: 1, // BeginRow(0)+1+0=1
			row:      0,
			col:      0,
			// col=0 -> "Field1" -> colMap["Field1"]=3 -> transposed.Position(1, 3) => table.Position(3, 1) => B4
			expect: "B4",
		},
		{
			name:     "value0-field1",
			valueRow: 1,
			row:      0,
			col:      1,
			// col=1 -> "Field2" -> colMap["Field2"]=4 -> transposed.Position(1, 4) => table.Position(4, 1) => B5
			expect: "B5",
		},
		{
			name:     "value1-field0",
			valueRow: 2, // BeginRow(0)+1+1=2
			row:      0,
			col:      0,
			// col=0 -> "Field1" -> colMap["Field1"]=3 -> transposed.Position(2, 3) => table.Position(3, 2) => C4
			expect: "C4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &unionFieldPositioner{
				basePositioner: basePositioner{tabler: transposed},
				valueRow:       tt.valueRow,
			}
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("unionFieldPositioner(transposed).Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestBasePositioner_ColIndex(t *testing.T) {
	tests := []struct {
		name   string
		tabler book.Tabler
		expect map[string]int
	}{
		{
			name: "struct-header-Name-Type",
			tabler: book.NewTable([][]string{
				{"Name", "Type"},
				{"ID", "uint32"},
			}),
			expect: map[string]int{"Name": 0, "Type": 1},
		},
		{
			name: "struct-header-Name-Type-Note",
			tabler: book.NewTable([][]string{
				{"Name", "Type", "Note"},
				{"ID", "uint32", "some note"},
			}),
			expect: map[string]int{"Name": 0, "Type": 1, "Note": 2},
		},
		{
			name: "struct-header-disordered",
			tabler: book.NewTable([][]string{
				{"Note", "Type", "Name"},
				{"some note", "int32", "ID"},
			}),
			expect: map[string]int{"Note": 0, "Type": 1, "Name": 2},
		},
		{
			name: "union-header",
			tabler: book.NewTable([][]string{
				{"Name", "Alias", "Type", "Field1", "Field2", "Field3"},
				{"v1", "a1", "t1", "f1", "f2", "f3"},
			}),
			expect: map[string]int{"Name": 0, "Alias": 1, "Type": 2, "Field1": 3, "Field2": 4, "Field3": 5},
		},
		{
			name: "union-header-disordered",
			tabler: book.NewTable([][]string{
				{"Field3", "Name", "Field2", "Alias", "Field1"},
				{"f3", "v1", "f2", "a1", "f1"},
			}),
			expect: map[string]int{"Field3": 0, "Name": 1, "Field2": 2, "Alias": 3, "Field1": 4},
		},
		{
			name: "empty-cells-skipped",
			tabler: book.NewTable([][]string{
				{"Name", "", "Type"},
				{"ID", "", "uint32"},
			}),
			expect: map[string]int{"Name": 0, "Type": 2},
		},
		{
			name: "transposed",
			tabler: book.NewTable([][]string{
				{"Name", "ID", "Num"},
				{"Type", "uint32", "int32"},
			}).Transpose(),
			// Transposed virtual header row: GetRow(0) => ["Name", "Type"]
			expect: map[string]int{"Name": 0, "Type": 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &basePositioner{tabler: tt.tabler}
			for k, v := range tt.expect {
				gotV, ok := bp.colIndex(k)
				if !ok {
					t.Errorf("basePositioner.colIndex(%q) not found, want %v", k, v)
				} else if gotV != v {
					t.Errorf("basePositioner.colIndex(%q) = %v, want %v", k, gotV, v)
				}
			}
			// Verify that a non-existent column returns false
			if _, ok := bp.colIndex("NonExistent"); ok {
				t.Errorf("basePositioner.colIndex(%q) should return false", "NonExistent")
			}
		})
	}
}
