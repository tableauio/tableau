package confgen

import (
	"context"

	"github.com/tableauio/tableau/internal/importer"
	"github.com/tableauio/tableau/internal/importer/book"
	"github.com/tableauio/tableau/internal/x/xproto"
)

// NewImporter reads a workbook's metasheet using the internal Tableau schema.
func NewImporter(ctx context.Context, workbookPath string) (importer.Importer, error) {
	parser := NewSheetParser(ctx, xproto.InternalProtoPackage, "", book.MetasheetOptions(ctx))
	return importer.New(ctx, workbookPath, importer.Parser(parser))
}
