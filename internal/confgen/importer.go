package confgen

import (
	"context"

	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xerrors"
	"github.com/tableauio/tableau/internal/x/xproto"
)

// NewImporter reads a workbook's metasheet using the internal Tableau schema.
// Parsing failures retain their structured source details and original causes.
func NewImporter(ctx context.Context, workbookPath string) (importer.Importer, error) {
	parser := NewSheetParser(ctx, xproto.InternalProtoPackage, "", book.MetasheetOptions(ctx))
	imp, err := importer.New(ctx, workbookPath, importer.Parser(parser))
	return imp, xerrors.Normalize(err)
}
