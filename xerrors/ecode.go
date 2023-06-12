package xerrors

// Error code space sections:
//	(0000, 0999]: common error
//  [1000, 1999]: protogen error
//  [2000, 2999]: confgen error
//	[3000, 3999]: importer error
//  [4000, ~]: reserved

// E0001 describes sheet not found in book.
func E0001(sheetName, BookName string) error {
	return renderEcode("E0001", map[string]interface{}{
		"SheetName": sheetName,
		"BookName":  BookName,
	})
}

// E2001 describes field prop "refer" not configured correctly.
func E2001(refer string, messageName string) error {
	return renderEcode("E2001", map[string]interface{}{
		"Refer":       refer,
		"MessageName": messageName,
	})
}

// E2002 describes field value not in referred space.
func E2002(value string, refer string) error {
	return renderEcode("E2002", map[string]interface{}{
		"Value": value,
		"Refer": refer,
	})
}

// E2002 describes illegal sequence number.
func E2003(value string, sequence int64) error {
	return renderEcode("E2003", map[string]interface{}{
		"Value":    value,
		"Sequence": sequence,
	})
}

// E2004 describes value is out of range.
func E2004(value interface{}, vrange string) error {
	return renderEcode("E2004", map[string]interface{}{
		"Value": value,
		"Range": vrange,
	})
}

// E2005 describes map key is not unique.
func E2005(key interface{}) error {
	return renderEcode("E2005", map[string]interface{}{
		"Key": key,
	})
}

// E2006 describes enum value not defined in enum type.
func E2006(value, enumName interface{}) error {
	return renderEcode("E2006", map[string]interface{}{
		"Value":    value,
		"EnumName": enumName,
	})
}

// E2007 describes invalid datetime format.
func E2007(value, err interface{}) error {
	return renderEcode("E2007", map[string]interface{}{
		"Value": value,
		"Error": err,
	})
}

// E2008 describes invalid duration format.
func E2008(value, err interface{}) error {
	return renderEcode("E2008", map[string]interface{}{
		"Value": value,
		"Error": err,
	})
}

// E2009 describes duplicate key.
func E2009(key, fieldName interface{}) error {
	return renderEcode("E2009", map[string]interface{}{
		"Key":       key,
		"FieldName": fieldName,
	})
}

// E2010 describes union type and value field mismatch.
func E2010(typeValue, fieldNumber interface{}) error {
	return renderEcode("E2010", map[string]interface{}{
		"TypeValue":   typeValue,
		"FieldNumber": fieldNumber,
	})
}

// E2011 describes field presence required but cell not filled.
func E2011() error {
	return renderEcode("E2011", nil)
}

// E2012 describes invalid syntax of numerical type.
func E2012(fieldType, value interface{}, err error) error {
	if err == nil {
		return nil
	}
	return renderEcode("E2012", map[string]interface{}{
		"FieldType": fieldType,
		"Value":     value,
		"Error":     err,
	})
}

// E2013 describes invalid syntax of boolean type.
func E2013(value interface{}, err error) error {
	if err == nil {
		return nil
	}
	return renderEcode("E2013", map[string]interface{}{
		"Value": value,
		"Error": err,
	})
}
