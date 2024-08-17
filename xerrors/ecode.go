package xerrors

// Error code space sections:
//	[0001, 0999]: common error
//  [1000, 1999]: protogen error
//  [2000, 2999]: confgen error
//	[3000, 3999]: importer error
//  [4000, ~]: reserved

// E0001: sheet not found in book.
func E0001(sheetName, bookName string) error {
	return renderEcode("E0001", map[string]any{
		"SheetName": sheetName,
		"BookName":  bookName,
	})
}

// E0002: cannot unmarshal file content to given proto.Message.
func E0002(filename, messageName, errstr string, lines string) error {
	return renderEcode("E0002", map[string]any{
		"Filename":    filename,
		"MessageName": messageName,
		"Error":       errstr,
		"Lines":       lines,
	})
}

// E1000: column name conflicts in name row.
func E1000(name, positon1, positon2 string) error {
	return renderEcode("E1000", map[string]any{
		"Name":      name,
		"Position1": positon1,
		"Position2": positon2,
	})
}

// E2000: integer overflow.
func E2000(typ, value string, min, max any) error {
	return renderEcode("E2000", map[string]any{
		"Type":  typ,
		"Value": value,
		"Min":   min,
		"Max":   max,
	})
}

// E2001: field prop "refer" not configured correctly.
func E2001(refer string, messageName string) error {
	return renderEcode("E2001", map[string]any{
		"Refer":       refer,
		"MessageName": messageName,
	})
}

// E2002: field value not in referred space.
func E2002(value string, refer string) error {
	return renderEcode("E2002", map[string]any{
		"Value": value,
		"Refer": refer,
	})
}

// E2002: illegal sequence number.
func E2003(value string, sequence int64) error {
	return renderEcode("E2003", map[string]any{
		"Value":    value,
		"Sequence": sequence,
	})
}

// E2004: value is out of range.
func E2004(value any, vrange string) error {
	return renderEcode("E2004", map[string]any{
		"Value": value,
		"Range": vrange,
	})
}

// E2005: map key is not unique.
func E2005(key any) error {
	return renderEcode("E2005", map[string]any{
		"Key": key,
	})
}

// E2006: enum value not defined in enum type.
func E2006(value, enumName any) error {
	return renderEcode("E2006", map[string]any{
		"Value":    value,
		"EnumName": enumName,
	})
}

// E2007: invalid datetime format.
func E2007(value, err any) error {
	return renderEcode("E2007", map[string]any{
		"Value": value,
		"Error": err,
	})
}

// E2008: invalid duration format.
func E2008(value, err any) error {
	return renderEcode("E2008", map[string]any{
		"Value": value,
		"Error": err,
	})
}

// E2009: duplicate key.
func E2009(key, fieldName any) error {
	return renderEcode("E2009", map[string]any{
		"Key":       key,
		"FieldName": fieldName,
	})
}

// E2010: union type and value field mismatch.
func E2010(typeValue, fieldNumber any) error {
	return renderEcode("E2010", map[string]any{
		"TypeValue":   typeValue,
		"FieldNumber": fieldNumber,
	})
}

// E2011: field presence required but cell not filled.
func E2011() error {
	return renderEcode("E2011", nil)
}

// E2012: invalid syntax of numerical type.
func E2012(fieldType, value any, err error) error {
	if err == nil {
		return nil
	}
	return renderEcode("E2012", map[string]any{
		"FieldType": fieldType,
		"Value":     value,
		"Error":     err,
	})
}

// E2013: invalid syntax of boolean type.
func E2013(value any, err error) error {
	if err == nil {
		return nil
	}
	return renderEcode("E2013", map[string]any{
		"Value": value,
		"Error": err,
	})
}

// E2014: sheet column not found.
func E2014(column string) error {
	return renderEcode("E2014", map[string]any{
		"Column": column,
	})
}

// E2015: referred sheet column not found.
func E2015(column, bookName, sheetName string) error {
	return renderEcode("E2015", map[string]any{
		"Column":    column,
		"BookName":  bookName,
		"SheetName": sheetName,
	})
}

// E2016: list elements are not present continuously.
func E2016(firstNonePresentIndex, nextPresentIndex int) error {
	return renderEcode("E2016", map[string]any{
		"FirstNonePresentIndex": firstNonePresentIndex,
		"NextPresentIndex":      nextPresentIndex,
	})
}

// E2017: map contains multiple empty keys.
func E2017(mapType string) error {
	return renderEcode("E2017", map[string]any{
		"MapType": mapType,
	})
}

// E2018: map key not exist.
func E2018(keyName string) error {
	return renderEcode("E2018", map[string]any{
		"KeyName": keyName,
	})
}

// E3000: no workbook file found about sheet specifier.
func E3000(sheetSpecifier, pattern string) error {
	return renderEcode("E3000", map[string]any{
		"SheetSpecifier": sheetSpecifier,
		"Pattern":        pattern,
	})
}

// E3001: no worksheet found in workbook.
func E3001(sheetName, bookName string) error {
	return renderEcode("E3001", map[string]any{
		"SheetName": sheetName,
		"BookName":  bookName,
	})
}
