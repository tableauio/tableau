package xerrors

// Error code space sections:
//	[0000, 0999]: common system error
//  [1000, 1999]: protogen error
//  [2000, 2999]: confgen error
//	[3000, 3999]: importer error
//  [4000, ~]: reserved

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
