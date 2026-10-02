package xlsx

import (
	"archive/zip"
	"errors"
	"io"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

// Reject larger parts rather than forwarding an unbounded allocation to the
// compatibility reader.
const maxEntrySize = 512 << 20

func readEntry(entry *zip.File) ([]byte, error) {
	size, err := validatedEntrySize(entry)
	if err != nil {
		return nil, err
	}
	source, err := entry.Open()
	if err != nil {
		return nil, err
	}
	data := make([]byte, size)
	_, readErr := io.ReadFull(source, data)
	if readErr == nil {
		readErr = ensureEntryExhausted(source, entry.Name)
	}
	return data, errors.Join(readErr, source.Close())
}

func validatedEntrySize(entry *zip.File) (int, error) {
	if entry.UncompressedSize64 > maxEntrySize {
		return 0, xerrors.Newf("XLSX entry %q exceeds the %d-byte size limit", entry.Name, maxEntrySize)
	}
	return int(entry.UncompressedSize64), nil
}

func ensureEntryExhausted(source io.Reader, entryName string) error {
	var extra [1]byte
	count, err := source.Read(extra[:])
	if err != nil && err != io.EOF {
		return err
	}
	if count != 0 || err == nil {
		return xerrors.Newf("XLSX entry %q size differs from its ZIP header", entryName)
	}
	return nil
}
