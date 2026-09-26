package importer

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
	xlsxWorkbookPath      = "xl/workbook.xml"
	xlsxSharedStringsPath = "xl/sharedStrings.xml"
	// Larger worksheet parts fall back to Excelize's temporary-file reader.
	maxXLSXEntrySize = 512 << 20
)

// xlsxReader reads raw cell values directly from selected OOXML worksheet
// parts. It avoids inflating unrelated ZIP entries and the reflection cost of
// decoding every cell into Excelize's general-purpose worksheet model.
type xlsxReader struct {
	archive       *zip.ReadCloser
	entries       map[string]*zip.File
	sheets        []xlsxSheet
	sharedOnce    sync.Once
	sharedStrings []string
	sharedErr     error
}

type xlsxSheet struct {
	name  string
	entry *zip.File
}

type xlsxWorkbook struct {
	Sheets []xlsxWorkbookSheet `xml:"sheets>sheet"`
}

type xlsxWorkbookSheet struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"id,attr"`
}

type xlsxRelationships struct {
	Items []xlsxRelationship `xml:"Relationship"`
}

type xlsxRelationship struct {
	ID     string `xml:"Id,attr"`
	Target string `xml:"Target,attr"`
	Type   string `xml:"Type,attr"`
}

type xlsxSharedStringTable struct {
	Items []xlsxStringItem `xml:"si"`
}

type xlsxStringItem struct {
	Text *xlsxText     `xml:"t"`
	Runs []xlsxTextRun `xml:"r"`
}

type xlsxTextRun struct {
	Text *xlsxText `xml:"t"`
}

type xlsxText struct {
	Value string `xml:",chardata"`
}

func openXLSXReader(filename string) (*xlsxReader, error) {
	archive, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	reader := &xlsxReader{
		archive: archive,
		entries: make(map[string]*zip.File, len(archive.File)),
	}
	for _, entry := range archive.File {
		name := strings.ToLower(strings.ReplaceAll(entry.Name, "\\", "/"))
		reader.entries[name] = entry
	}
	if err := reader.loadSheets(); err != nil {
		return nil, errors.Join(err, archive.Close())
	}
	return reader, nil
}

func (r *xlsxReader) loadSheets() error {
	workbookPath := xlsxWorkbookPath
	if rootRels, err := r.readRelationships("_rels/.rels"); err == nil {
		for _, rel := range rootRels.Items {
			if strings.HasSuffix(rel.Type, "/officeDocument") {
				workbookPath = resolveXLSXPath("", rel.Target)
				break
			}
		}
	}
	var workbook xlsxWorkbook
	if err := r.readXML(workbookPath, &workbook); err != nil {
		return err
	}
	relsPath := path.Join(path.Dir(workbookPath), "_rels", path.Base(workbookPath)+".rels")
	rels, err := r.readRelationships(relsPath)
	if err != nil {
		return err
	}
	targets := make(map[string]string, len(rels.Items))
	for _, rel := range rels.Items {
		targets[rel.ID] = resolveXLSXPath(workbookPath, rel.Target)
	}
	for _, sheet := range workbook.Sheets {
		entry := r.entry(targets[sheet.RID])
		if entry == nil {
			return fmt.Errorf("worksheet %q relationship %q not found", sheet.Name, sheet.RID)
		}
		r.sheets = append(r.sheets, xlsxSheet{name: sheet.Name, entry: entry})
	}
	return nil
}

func (r *xlsxReader) readRelationships(name string) (*xlsxRelationships, error) {
	var rels xlsxRelationships
	if err := r.readXML(name, &rels); err != nil {
		return nil, err
	}
	return &rels, nil
}

func (r *xlsxReader) readXML(name string, value any) error {
	entry := r.entry(name)
	if entry == nil {
		return fmt.Errorf("XLSX entry %q not found", name)
	}
	data, err := readZipEntry(entry)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode XLSX entry %q: %w", name, err)
	}
	return nil
}

func (r *xlsxReader) entry(name string) *zip.File {
	return r.entries[strings.ToLower(strings.TrimPrefix(path.Clean(strings.ReplaceAll(name, "\\", "/")), "/"))]
}

func (r *xlsxReader) SheetNames() []string {
	names := make([]string, len(r.sheets))
	for i, sheet := range r.sheets {
		names[i] = sheet.name
	}
	return names
}

func (r *xlsxReader) ReadRows(sheetName string) ([][]string, error) {
	var entry *zip.File
	for _, sheet := range r.sheets {
		if sheet.name == sheetName {
			entry = sheet.entry
			break
		}
	}
	if entry == nil {
		return nil, ErrSheetNotFound
	}
	data, err := readZipEntry(entry)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(data, []byte("<![CDATA[")) || bytes.Contains(data, []byte("<!DOCTYPE")) {
		return nil, fmt.Errorf("unsupported XML construct in worksheet %q", sheetName)
	}
	sharedStrings, err := r.loadSharedStrings()
	if err != nil {
		return nil, err
	}
	return parseXLSXRows(data, sharedStrings)
}

func (r *xlsxReader) loadSharedStrings() ([]string, error) {
	r.sharedOnce.Do(func() {
		entry := r.entry(xlsxSharedStringsPath)
		if entry == nil {
			return
		}
		data, err := readZipEntry(entry)
		if err != nil {
			r.sharedErr = err
			return
		}
		var table xlsxSharedStringTable
		if err := xml.Unmarshal(data, &table); err != nil {
			r.sharedErr = fmt.Errorf("decode XLSX shared strings: %w", err)
			return
		}
		r.sharedStrings = make([]string, len(table.Items))
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
			r.sharedStrings[i] = decodeExcelEscapes(text.String())
		}
	})
	return r.sharedStrings, r.sharedErr
}

func (r *xlsxReader) Close() error {
	return r.archive.Close()
}

func resolveXLSXPath(base, target string) string {
	target = strings.ReplaceAll(target, "\\", "/")
	if strings.HasPrefix(target, "/") {
		return strings.TrimPrefix(path.Clean(target), "/")
	}
	return path.Clean(path.Join(path.Dir(base), target))
}

func readZipEntry(entry *zip.File) ([]byte, error) {
	if entry.UncompressedSize64 > maxXLSXEntrySize {
		return nil, fmt.Errorf("XLSX entry %q exceeds the %d-byte in-memory limit", entry.Name, maxXLSXEntrySize)
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
