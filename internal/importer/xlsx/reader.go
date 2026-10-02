// Package xlsx reads raw cell values from selected OOXML worksheets.
package xlsx

import (
	"archive/zip"
	"errors"
	"strings"
	"sync"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

const sharedStringsPath = "xl/sharedStrings.xml"

// ErrSheetNotFound reports that a workbook has no sheet with the requested name.
var ErrSheetNotFound = errors.New("sheet not found")

// Reader reads worksheet rows without inflating unrelated ZIP parts or
// decoding worksheets into a general-purpose document model.
type Reader struct {
	archive *zip.ReadCloser
	entries map[string]*zip.File

	sheetNames  []string
	sheets      map[string]*zip.File
	sharedEntry *zip.File
	sharedOnce  sync.Once
	shared      []string
	sharedErr   error
	indexOnce   sync.Once
	sharedIndex *sharedStringIndex
	indexErr    error
}

// Open opens an XLSX archive and indexes its worksheets.
func Open(filename string) (*Reader, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	reader := &Reader{
		archive: archive,
		entries: make(map[string]*zip.File, len(archive.File)),
		sheets:  make(map[string]*zip.File),
	}
	for _, entry := range archive.File {
		name := normalizePartName(entry.Name)
		if name == "." || name == ".." || strings.HasPrefix(name, "../") {
			return nil, errors.Join(xerrors.Newf("invalid XLSX entry path %q", entry.Name), archive.Close())
		}
		if _, exists := reader.entries[name]; exists {
			return nil, errors.Join(xerrors.Newf("duplicate XLSX entry %q", entry.Name), archive.Close())
		}
		reader.entries[name] = entry
	}
	if err := reader.loadParts(); err != nil {
		return nil, errors.Join(err, archive.Close())
	}
	return reader, nil
}

// SheetNames returns worksheet names in workbook order.
func (r *Reader) SheetNames() []string {
	return append([]string(nil), r.sheetNames...)
}

// ReadRows reads all populated rows from the named worksheet.
func (r *Reader) ReadRows(sheetName string) ([][]string, error) {
	entry := r.sheets[sheetName]
	if entry == nil {
		return nil, ErrSheetNotFound
	}
	shared, err := r.loadSharedStrings()
	if err != nil {
		return nil, err
	}
	return readWorksheetRows(entry, sheetName, shared, 0)
}

// ReadRowsN reads at most rowLimit rows from the named worksheet. A zero limit
// reads all populated rows. Shared-string values are decoded on demand; ReadRows
// retains complete-table loading for full configuration imports.
func (r *Reader) ReadRowsN(sheetName string, rowLimit uint) ([][]string, error) {
	entry := r.sheets[sheetName]
	if entry == nil {
		return nil, ErrSheetNotFound
	}
	rows := newWorksheetRows(nil, rowLimit)
	rows.sharedValue = r.sharedStringValue
	return readWorksheetWithRows(entry, sheetName, rows)
}

func (r *Reader) sharedStringValue(index int) (string, error) {
	shared, err := r.loadSharedStringIndex()
	if err != nil {
		return "", err
	}
	return shared.value(index)
}

func (r *Reader) loadSharedStringIndex() (*sharedStringIndex, error) {
	r.indexOnce.Do(func() {
		r.sharedIndex = &sharedStringIndex{}
		if r.sharedEntry == nil {
			return
		}
		data, err := readEntry(r.sharedEntry)
		if err == nil {
			r.sharedIndex, err = newSharedStringIndex(data)
		}
		if err != nil {
			r.indexErr = xerrors.Wrapf(err, "decode XLSX shared strings")
		}
	})
	return r.sharedIndex, r.indexErr
}

func (r *Reader) loadSharedStrings() ([]string, error) {
	r.sharedOnce.Do(func() {
		if r.sharedEntry == nil {
			return
		}
		if _, err := validatedEntrySize(r.sharedEntry); err != nil {
			r.sharedErr = err
			return
		}
		source, err := r.sharedEntry.Open()
		if err != nil {
			r.sharedErr = err
			return
		}
		sharedStrings, readErr := readSharedStrings(source)
		err = errors.Join(readErr, source.Close())
		if err != nil {
			r.sharedErr = xerrors.Wrapf(err, "decode XLSX shared strings")
			return
		}
		r.shared = sharedStrings
	})
	return r.shared, r.sharedErr
}

// Close closes the XLSX archive.
func (r *Reader) Close() error {
	if r == nil || r.archive == nil {
		return nil
	}
	return r.archive.Close()
}
