package tableparser

import (
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/xerrors"
)

// RangeDataRows ranges data rows in the table.
func RangeDataRows(table book.Tabler, header *Header, sheetName string, fn func(*book.Row) error) error {
	columns, lookupTable, err := parseColumns(table, header)
	if err != nil {
		return err
	}
	var prev *book.Row
	// [datarow, endRow]: data rows
	dataRow := table.BeginRow() + header.DataRow - 1
	for row := dataRow; row < table.EndRow(); row++ {
		curr := book.NewRow(row, prev, sheetName, lookupTable)
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
