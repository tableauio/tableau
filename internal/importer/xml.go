package importer

import (
	"github.com/tableauio/tableau/internal/importer/book"
)

type XMLImporter struct {
	*book.Book
}

func NewXMLImporter(filename string, sheets []string, parser book.SheetParser, mode ImporterMode, cloned bool, primaryBookName string) (*XMLImporter, error) {
	var book *book.Book
	// var err error
	// if mode == Protogen {
	// 	book, err = readYAMLBookWithOnlySchemaSheet(filename, parser)
	// 	if err != nil {
	// 		return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
	// 	}
	// 	if err := book.ParseMetaAndPurge(); err != nil {
	// 		return nil, errors.WithMessage(err, "failed to parse metasheet")
	// 	}
	// } else {
	// 	book, err = readYAMLBook(filename, parser)
	// 	if err != nil {
	// 		return nil, errors.WithMessagef(err, "failed to read csv book: %s", filename)
	// 	}
	// }

	// log.Debugf("book: %+v", book)
	return &XMLImporter{
		Book: book,
	}, nil
}
