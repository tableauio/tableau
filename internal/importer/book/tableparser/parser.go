package tableparser

import (
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

// RangeDataRows ranges data rows in the table.
func RangeDataRows(table book.Tabler, header *Header, fn func(*book.Row) error) error {
	columns, lookupTable, err := parseColumns(table, header)
	if err != nil {
		return err
	}
	dataRow := table.BeginRow() + header.DataRow - 1
	return rangeDataRowsInRange(table, columns, lookupTable, header, dataRow, table.EndRow(), fn)
}

// RangeDataRowsInBlock ranges data rows in [blockBegin, blockEnd) of the table.
//
// Compared to RangeDataRows, the caller controls the row range, which is useful
// for sharding the data rows across goroutines. The header (name/type rows) is
// still parsed once from the table itself, so the column lookup table is
// consistent across blocks.
//
// NOTE: blockBegin/blockEnd must lie within [dataRow, EndRow()) where dataRow
// is `table.BeginRow() + header.DataRow - 1`. The previous-row chain (used for
// adjacent_key auto-population) restarts at nil at the block boundary, so
// callers MUST ensure the sheet does not rely on cross-block prev linkage
// (e.g. AdjacentKey is not enabled).
func RangeDataRowsInBlock(table book.Tabler, header *Header, blockBegin, blockEnd int, fn func(*book.Row) error) error {
	columns, lookupTable, err := parseColumns(table, header)
	if err != nil {
		return err
	}
	return rangeDataRowsInRange(table, columns, lookupTable, header, blockBegin, blockEnd, fn)
}

// DataRowRange returns the [begin, end) data row range of the table according
// to the header settings. It can be used by callers that need to shard the
// data rows for parallel processing.
func DataRowRange(table book.Tabler, header *Header) (int, int) {
	return table.BeginRow() + header.DataRow - 1, table.EndRow()
}

// RangeDataRowsByIndices ranges data rows in the explicit `rowIndices` list of
// the table, in the given order.
//
// Compared to RangeDataRowsInBlock (which iterates a contiguous half-open
// interval), this variant lets callers pick any subset of data rows -- useful
// for hash-partitioned parallel parsing where rows sharing the same outer key
// must be processed by the same goroutine. Callers SHOULD pass `rowIndices` in
// ascending order; the iteration faithfully follows the slice order, so any
// non-ascending input would scramble within-shard row order and break the
// invariant that "the merged result equals the serial result".
//
// The previous-row chain (used for adjacent_key auto-population) restarts at
// nil at every call, so callers MUST ensure the sheet does not rely on
// cross-row prev linkage (e.g. AdjacentKey is not enabled).
func RangeDataRowsByIndices(table book.Tabler, header *Header, rowIndices []int, fn func(*book.Row) error) error {
	columns, lookupTable, err := parseColumns(table, header)
	if err != nil {
		return err
	}
	return rangeDataRowsAtIndices(table, columns, lookupTable, header, rowIndices, fn)
}

func rangeDataRowsAtIndices(table book.Tabler, columns map[int]*book.Column, lookupTable book.ColumnLookupTable, header *Header, rowIndices []int, fn func(*book.Row) error) error {
	var prev *book.Row
	for _, row := range rowIndices {
		curr := book.NewRow(row, prev, lookupTable)
		for col := table.BeginCol(); col < table.EndCol(); col++ {
			data, err := table.Cell(row, col)
			if err != nil {
				return xerrors.WrapKV(err)
			}
			curr.AddCell(columns[col], data, header.AdjacentKey)
		}
		ignored, err := curr.Ignored()
		if err != nil {
			return err
		}
		if ignored {
			curr.Free()
			continue
		}
		err = fn(curr)
		if err != nil {
			return err
		}
		if prev != nil {
			prev.Free()
		}
		prev = curr
	}
	if prev != nil {
		prev.Free()
	}
	return nil
}

// ScanColumn reads the raw cell strings of the named column for every data
// row in [beginRow, endRow), returning a slice with one entry per row. It is
// intended for cheap pre-scans that need to bucket rows by a single column
// (e.g. parallel sharders that hash on the top-level outer-key column).
//
// The returned slice is parallel to [beginRow, endRow): index i corresponds
// to row beginRow+i. ScanColumn does not honour `ignored` rows -- the caller
// already accepts that the partition merely needs to be a function of (row,
// key) and that any row-level filtering will happen later, inside the worker.
//
// If the named column is absent, ScanColumn returns an error so the caller
// fails loudly rather than silently bucket all rows into a single shard.
func ScanColumn(table book.Tabler, header *Header, name string, beginRow, endRow int) ([]string, error) {
	_, lookupTable, err := parseColumns(table, header)
	if err != nil {
		return nil, err
	}
	col, ok := lookupTable[name]
	if !ok {
		return nil, xerrors.E2014(name)
	}
	values := make([]string, endRow-beginRow)
	for row := beginRow; row < endRow; row++ {
		data, cellErr := table.Cell(row, col)
		if cellErr != nil {
			return nil, xerrors.WrapKV(cellErr)
		}
		values[row-beginRow] = data
	}
	return values, nil
}

func rangeDataRowsInRange(table book.Tabler, columns map[int]*book.Column, lookupTable book.ColumnLookupTable, header *Header, beginRow, endRow int, fn func(*book.Row) error) error {
	var prev *book.Row
	for row := beginRow; row < endRow; row++ {
		curr := book.NewRow(row, prev, lookupTable)
		for col := table.BeginCol(); col < table.EndCol(); col++ {
			data, err := table.Cell(row, col)
			if err != nil {
				return xerrors.WrapKV(err)
			}
			curr.AddCell(columns[col], data, header.AdjacentKey)
		}
		ignored, err := curr.Ignored()
		if err != nil {
			return err
		}
		if ignored {
			curr.Free()
			continue
		}
		err = fn(curr)
		if err != nil {
			return err
		}
		if prev != nil {
			prev.Free()
		}
		prev = curr
	}
	if prev != nil {
		prev.Free()
	}
	return nil
}

func parseColumns(table book.Tabler, header *Header) (map[int]*book.Column, book.ColumnLookupTable, error) {
	nameRow := table.BeginRow() + header.NameRow - 1
	typeRow := table.BeginRow() + header.TypeRow - 1
	columns := make(map[int]*book.Column, table.ColSize())
	lookupTable := make(book.ColumnLookupTable, table.ColSize())
	for col := table.BeginCol(); col < table.EndCol(); col++ {
		// parse names
		nameCell, err := table.Cell(nameRow, col)
		if err != nil {
			return nil, nil, xerrors.WrapKV(err, table.Position(nameRow, col))
		}
		name := book.ExtractFromCell(nameCell, header.NameLine)
		if name != "" {
			// parse lookup table
			if foundCol, ok := lookupTable[name]; ok {
				return nil, nil, xerrors.E0003(name, table.Position(nameRow, foundCol), table.Position(nameRow, col))
			}
			lookupTable[name] = col
		}
		// parse types
		typeCell, err := table.Cell(typeRow, col)
		if err != nil {
			return nil, nil, xerrors.WrapKV(err)
		}
		typ := book.ExtractFromCell(typeCell, header.TypeLine)
		columns[col] = &book.Column{
			Col:  col,
			Name: name,
			Type: typ,
		}
	}
	return columns, lookupTable, nil
}
