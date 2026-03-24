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
		tabler:  table,
		dataRow: table.BeginRow() + 1, // datarow=2 (1-based), 0-based: 1
	}

	tests := []struct {
		name   string
		row    int // virtual header row index: 0=Name col, 1=Type col
		col    int // cursor (field index)
		expect string
	}{
		{
			name:   "NameRow-cursor0",
			row:    0,
			col:    0,
			expect: "A2", // dataRow(1)+cursor(0)=row1, col0=A => A2
		},
		{
			name:   "TypeRow-cursor0",
			row:    1,
			col:    0,
			expect: "B2", // dataRow(1)+cursor(0)=row1, col1=B => B2
		},
		{
			name:   "NameRow-cursor4",
			row:    0,
			col:    4,
			expect: "A6", // dataRow(1)+cursor(4)=row5, col0=A => A6
		},
		{
			name:   "TypeRow-cursor4",
			row:    1,
			col:    4,
			expect: "B6", // dataRow(1)+cursor(4)=row5, col1=B => B6
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
		tabler:  transposed,
		dataRow: transposed.BeginRow() + 1, // datarow=2 (1-based), 0-based in transposed: 1
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
	// Simulate a non-transposed union type sheet:
	//      A(col0)  B(col1)  C(col2)  D(col3)  E(col4)  F(col5)
	// R1:  Name     Alias    Type     Field1   Field2   Field3    <- header
	// R2:  Value0   Alias0   Type0    <cell>   <cell>   <cell>   <- value 0
	// R3:  Value1   Alias1   Type1    <cell>   <cell>   <cell>   <- value 1
	// ...
	// R228: Value226 Alias226 Type226 <cell>   <cell>   <cell>   <- value 226
	table := book.NewTable(make([][]string, 228)) // 228 rows

	tests := []struct {
		name          string
		valueRow      int
		fieldStartCol int
		row           int // unused in unionFieldPositioner
		col           int // cursor (field index)
		expect        string
	}{
		{
			name:          "value0-field0",
			valueRow:      1, // datarow=2 (1-based), BeginRow(0)+1+0=1
			fieldStartCol: 3, // Field1 at col D
			row:           0,
			col:           0,
			expect:        "D2", // Position(1, 3) => D2
		},
		{
			name:          "value0-field2",
			valueRow:      1,
			fieldStartCol: 3,
			row:           0,
			col:           2,
			expect:        "F2", // Position(1, 5) => F2
		},
		{
			name:          "value226-field0-D228",
			valueRow:      227, // BeginRow(0)+1+226=227
			fieldStartCol: 3,
			row:           0,
			col:           0,
			expect:        "D228", // Position(227, 3) => D228
		},
		{
			name:          "value226-field1-E228",
			valueRow:      227,
			fieldStartCol: 3,
			row:           0,
			col:           1,
			expect:        "E228", // Position(227, 4) => E228
		},
		{
			name:          "row-param-ignored",
			valueRow:      227,
			fieldStartCol: 3,
			row:           1, // different row param, should still produce same result
			col:           0,
			expect:        "D228", // row param is ignored, Position(227, 3) => D228
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &unionFieldPositioner{
				tabler:        table,
				valueRow:      tt.valueRow,
				fieldStartCol: tt.fieldStartCol,
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
		name          string
		valueRow      int
		fieldStartCol int
		row           int
		col           int
		expect        string
	}{
		{
			name:          "value0-field0",
			valueRow:      1, // BeginRow(0)+1+0=1
			fieldStartCol: 3, // Field1 at virtual col 3
			row:           0,
			col:           0,
			// transposed.Position(1, 3) => table.Position(3, 1) => B4
			expect: "B4",
		},
		{
			name:          "value0-field1",
			valueRow:      1,
			fieldStartCol: 3,
			row:           0,
			col:           1,
			// transposed.Position(1, 4) => table.Position(4, 1) => B5
			expect: "B5",
		},
		{
			name:          "value1-field0",
			valueRow:      2, // BeginRow(0)+1+1=2
			fieldStartCol: 3,
			row:           0,
			col:           0,
			// transposed.Position(2, 3) => table.Position(3, 2) => C4
			expect: "C4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &unionFieldPositioner{
				tabler:        transposed,
				valueRow:      tt.valueRow,
				fieldStartCol: tt.fieldStartCol,
			}
			got := p.Position(tt.row, tt.col)
			if got != tt.expect {
				t.Errorf("unionFieldPositioner(transposed).Position(%d, %d) = %v, want %v", tt.row, tt.col, got, tt.expect)
			}
		})
	}
}

func TestFindFieldStartCol(t *testing.T) {
	tests := []struct {
		name   string
		tabler book.Tabler
		expect int
	}{
		{
			name: "standard-union-header-with-type",
			tabler: book.NewTable([][]string{
				{"Name", "Alias", "Type", "Field1", "Field2"},
				{"v1", "a1", "t1", "f1\nint32", "f2\nstring"},
			}),
			expect: 3, // Field1 at col D (index 3)
		},
		{
			name: "union-header-without-number-and-type",
			tabler: book.NewTable([][]string{
				{"Name", "Alias", "Field1", "Field2"},
				{"v1", "a1", "f1\nint32", "f2\nstring"},
			}),
			expect: 2, // Field1 at col C (index 2)
		},
		{
			name: "union-header-with-number",
			tabler: book.NewTable([][]string{
				{"Number", "Name", "Alias", "Type", "Field1", "Field2"},
				{"1", "v1", "a1", "t1", "f1\nint32", "f2\nstring"},
			}),
			expect: 4, // Field1 at col E (index 4)
		},
		{
			name: "no-field-columns-fallback",
			tabler: book.NewTable([][]string{
				{"Name", "Alias", "Type"},
				{"v1", "a1", "t1"},
			}),
			expect: 0, // fallback to 0
		},
		{
			name: "transposed-union-header",
			tabler: book.NewTable([][]string{
				// Underlying table (before transpose):
				//      A       B       C
				// R1:  Name    v1      v2
				// R2:  Alias   a1      a2
				// R3:  Type    t1      t2
				// R4:  Field1  f1      f3
				// R5:  Field2  f2      f4
				{"Name", "v1", "v2"},
				{"Alias", "a1", "a2"},
				{"Type", "t1", "t2"},
				{"Field1", "f1", "f3"},
				{"Field2", "f2", "f4"},
			}).Transpose(),
			// Transposed virtual header row (GetRow(BeginRow=0)):
			// GetRow(0) => table.getCol(0) => ["Name", "Alias", "Type", "Field1", "Field2"]
			// Field1 at index 3
			expect: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findFieldStartCol(tt.tabler)
			if got != tt.expect {
				t.Errorf("findFieldStartCol() = %v, want %v", got, tt.expect)
			}
		})
	}
}
