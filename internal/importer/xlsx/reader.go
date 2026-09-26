// Package xlsx reads raw cell values from selected OOXML worksheets.
package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
)

const (
	defaultWorkbookPath = "xl/workbook.xml"
	sharedStringsPath   = "xl/sharedStrings.xml"
	// Larger parts use the caller's compatibility reader instead of allocating
	// an unbounded in-memory buffer.
	maxEntrySize = 512 << 20
)

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
}

type workbook struct {
	Sheets []workbookSheet `xml:"sheets>sheet"`
}

type workbookSheet struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"id,attr"`
}

type relationships struct {
	Items []relationship `xml:"Relationship"`
}

type relationship struct {
	ID         string `xml:"Id,attr"`
	Target     string `xml:"Target,attr"`
	Type       string `xml:"Type,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

type sharedStringTable struct {
	Items []stringItem `xml:"si"`
}

type stringItem struct {
	Text *textValue `xml:"t"`
	Runs []textRun  `xml:"r"`
}

type textRun struct {
	Text *textValue `xml:"t"`
}

type textValue struct {
	Value string `xml:",chardata"`
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
			return nil, errors.Join(fmt.Errorf("invalid XLSX entry path %q", entry.Name), archive.Close())
		}
		if _, exists := reader.entries[name]; exists {
			return nil, errors.Join(fmt.Errorf("duplicate XLSX entry %q", entry.Name), archive.Close())
		}
		reader.entries[name] = entry
	}
	if err := reader.loadParts(); err != nil {
		return nil, errors.Join(err, archive.Close())
	}
	return reader, nil
}

func (r *Reader) loadParts() error {
	workbookPath, err := r.findWorkbookPath()
	if err != nil {
		return err
	}
	var book workbook
	if err := r.readXML(workbookPath, &book); err != nil {
		return err
	}

	relsPath := path.Join(path.Dir(workbookPath), "_rels", path.Base(workbookPath)+".rels")
	rels, err := r.readRelationships(relsPath)
	if err != nil {
		return err
	}
	targets := make(map[string]string, len(rels.Items))
	ids := make(map[string]struct{}, len(rels.Items))
	sharedFound := false
	for _, rel := range rels.Items {
		if strings.EqualFold(rel.TargetMode, "External") {
			continue
		}
		if rel.ID == "" {
			return errors.New("workbook relationship has an empty ID")
		}
		if _, exists := ids[rel.ID]; exists {
			return fmt.Errorf("duplicate workbook relationship %q", rel.ID)
		}
		ids[rel.ID] = struct{}{}
		target := resolvePartPath(workbookPath, rel.Target)
		switch {
		case hasRelationshipType(rel.Type, "worksheet"):
			targets[rel.ID] = target
		case hasRelationshipType(rel.Type, "sharedStrings"):
			if sharedFound {
				return errors.New("duplicate shared strings relationship")
			}
			sharedFound = true
			r.sharedEntry = r.entry(target)
			if r.sharedEntry == nil {
				return fmt.Errorf("shared strings relationship %q not found", rel.ID)
			}
		}
	}
	if !sharedFound {
		r.sharedEntry = r.entry(sharedStringsPath)
	}

	for _, sheet := range book.Sheets {
		if sheet.Name == "" {
			return errors.New("worksheet has an empty name")
		}
		if _, exists := r.sheets[sheet.Name]; exists {
			return fmt.Errorf("duplicate worksheet name %q", sheet.Name)
		}
		entry := r.entry(targets[sheet.RID])
		if entry == nil {
			return fmt.Errorf("worksheet %q relationship %q not found", sheet.Name, sheet.RID)
		}
		r.sheetNames = append(r.sheetNames, sheet.Name)
		r.sheets[sheet.Name] = entry
	}
	return nil
}

func (r *Reader) findWorkbookPath() (string, error) {
	const rootRelsPath = "_rels/.rels"
	if r.entry(rootRelsPath) == nil {
		return defaultWorkbookPath, nil
	}
	rels, err := r.readRelationships(rootRelsPath)
	if err != nil {
		return "", err
	}
	for _, rel := range rels.Items {
		if !strings.EqualFold(rel.TargetMode, "External") && hasRelationshipType(rel.Type, "officeDocument") {
			return resolvePartPath("", rel.Target), nil
		}
	}
	return defaultWorkbookPath, nil
}

func hasRelationshipType(value, name string) bool {
	return value == name || strings.HasSuffix(value, "/"+name)
}

func (r *Reader) readRelationships(name string) (*relationships, error) {
	var rels relationships
	if err := r.readXML(name, &rels); err != nil {
		return nil, err
	}
	return &rels, nil
}

func (r *Reader) readXML(name string, value any) error {
	entry := r.entry(name)
	if entry == nil {
		return fmt.Errorf("XLSX entry %q not found", name)
	}
	data, err := readEntry(entry)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode XLSX entry %q: %w", name, err)
	}
	return nil
}

func (r *Reader) entry(name string) *zip.File {
	if name == "" {
		return nil
	}
	return r.entries[normalizePartName(name)]
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
	data, err := readEntry(entry)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(data, []byte("<![CDATA[")) || bytes.Contains(data, []byte("<!DOCTYPE")) {
		return nil, fmt.Errorf("unsupported XML construct in worksheet %q", sheetName)
	}
	shared, err := r.loadSharedStrings()
	if err != nil {
		return nil, err
	}
	return parseRows(data, shared)
}

func (r *Reader) loadSharedStrings() ([]string, error) {
	r.sharedOnce.Do(func() {
		if r.sharedEntry == nil {
			return
		}
		data, err := readEntry(r.sharedEntry)
		if err != nil {
			r.sharedErr = err
			return
		}
		var table sharedStringTable
		if err := xml.Unmarshal(data, &table); err != nil {
			r.sharedErr = fmt.Errorf("decode XLSX shared strings: %w", err)
			return
		}
		r.shared = make([]string, len(table.Items))
		for i, item := range table.Items {
			var text strings.Builder
			if item.Text != nil {
				text.WriteString(item.Text.Value)
			}
			for _, run := range item.Runs {
				if run.Text != nil {
					text.WriteString(run.Text.Value)
				}
			}
			r.shared[i] = decodeEscapes(text.String())
		}
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

func normalizePartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	return strings.ToLower(strings.TrimPrefix(path.Clean(name), "/"))
}

func resolvePartPath(base, target string) string {
	target = strings.ReplaceAll(target, "\\", "/")
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(path.Dir(base), target))
}

func readEntry(entry *zip.File) ([]byte, error) {
	if entry.UncompressedSize64 > maxEntrySize {
		return nil, fmt.Errorf("XLSX entry %q exceeds the %d-byte in-memory limit", entry.Name, maxEntrySize)
	}
	file, err := entry.Open()
	if err != nil {
		return nil, err
	}
	data := make([]byte, int(entry.UncompressedSize64))
	_, readErr := io.ReadFull(file, data)
	if readErr == nil {
		var extra [1]byte
		if n, err := file.Read(extra[:]); n != 0 || (err != nil && err != io.EOF) {
			readErr = fmt.Errorf("XLSX entry %q size differs from its ZIP header", entry.Name)
		}
	}
	closeErr := file.Close()
	return data, errors.Join(readErr, closeErr)
}
