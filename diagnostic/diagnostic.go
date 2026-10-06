// Package diagnostic exposes tableau's structured error descriptions.
// It shares extraction, scope inheritance, localization, and rendering with
// tableauc, so callers do not need to interpret internal error wrappers.
package diagnostic

import "github.com/tableauio/tableau/internal/x/xerrors"

// Desc is tableau's structured error description. A single error has Fields;
// an aggregate has Children, each with its own workbook, worksheet, and cell.
// String renders the localized summary without debugging fields or stacks.
// JSON contains fields and children, without the underlying errors or stacks.
type Desc = xerrors.Desc

// NewDesc describes err using the same rules as tableauc. Returns nil for nil
// err. Standard wrappers and joins are supported; innermost field values win.
func NewDesc(err error) *Desc {
	return xerrors.NewDesc(err)
}
