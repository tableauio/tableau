package xlsx

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path"
	"strings"

	"github.com/tableauio/tableau/internal/x/xerrors"
)

const defaultWorkbookPath = "xl/workbook.xml"

type workbookDocument struct {
	Sheets []workbookSheet `xml:"sheets>sheet"`
}

type workbookSheet struct {
	Name string `xml:"name,attr"`
	RID  string `xml:"id,attr"`
}

type relationshipDocument struct {
	Items []relationship `xml:"Relationship"`
}

type relationship struct {
	ID         string `xml:"Id,attr"`
	Target     string `xml:"Target,attr"`
	Type       string `xml:"Type,attr"`
	TargetMode string `xml:"TargetMode,attr"`
}

func (r *Reader) loadParts() error {
	workbookPath, err := r.findWorkbookPath()
	if err != nil {
		return err
	}
	var workbook workbookDocument
	if err := r.readXML(workbookPath, &workbook); err != nil {
		return err
	}

	relationshipsPath := path.Join(path.Dir(workbookPath), "_rels", path.Base(workbookPath)+".rels")
	relationships, err := r.readRelationships(relationshipsPath)
	if err != nil {
		return err
	}
	worksheetTargets := make(map[string]string, len(relationships.Items))
	relationshipTypes := make(map[string]string, len(relationships.Items))
	sharedStringsFound := false
	for _, relationship := range relationships.Items {
		if strings.EqualFold(relationship.TargetMode, "External") {
			continue
		}
		if relationship.ID == "" {
			return xerrors.New("workbook relationship has an empty ID")
		}
		if _, exists := relationshipTypes[relationship.ID]; exists {
			return xerrors.Newf("duplicate workbook relationship %q", relationship.ID)
		}
		relationshipTypes[relationship.ID] = relationship.Type
		target := resolvePartPath(workbookPath, relationship.Target)
		switch {
		case hasRelationshipType(relationship.Type, "worksheet"):
			worksheetTargets[relationship.ID] = target
		case hasRelationshipType(relationship.Type, "sharedStrings"):
			if sharedStringsFound {
				return xerrors.New("duplicate shared strings relationship")
			}
			sharedStringsFound = true
			r.sharedEntry = r.entry(target)
			if r.sharedEntry == nil {
				return xerrors.Newf("shared strings relationship %q not found", relationship.ID)
			}
		}
	}
	if !sharedStringsFound {
		r.sharedEntry = r.entry(sharedStringsPath)
	}

	for _, sheet := range workbook.Sheets {
		if sheet.Name == "" {
			return xerrors.New("worksheet has an empty name")
		}
		if _, exists := r.sheets[sheet.Name]; exists {
			return xerrors.Newf("duplicate worksheet name %q", sheet.Name)
		}
		if kind, exists := relationshipTypes[sheet.RID]; exists && !hasRelationshipType(kind, "worksheet") {
			return fmt.Errorf("%w: sheet %q has relationship type %q", ErrUnsupported, sheet.Name, kind)
		}
		entry := r.entry(worksheetTargets[sheet.RID])
		if entry == nil {
			return xerrors.Newf("worksheet %q relationship %q not found", sheet.Name, sheet.RID)
		}
		r.sheetNames = append(r.sheetNames, sheet.Name)
		r.sheets[sheet.Name] = entry
	}
	return nil
}

func (r *Reader) findWorkbookPath() (string, error) {
	const rootRelationshipsPath = "_rels/.rels"
	if r.entry(rootRelationshipsPath) == nil {
		return defaultWorkbookPath, nil
	}
	relationships, err := r.readRelationships(rootRelationshipsPath)
	if err != nil {
		return "", err
	}
	for _, relationship := range relationships.Items {
		if !strings.EqualFold(relationship.TargetMode, "External") &&
			hasRelationshipType(relationship.Type, "officeDocument") {
			return resolvePartPath("", relationship.Target), nil
		}
	}
	return defaultWorkbookPath, nil
}

func hasRelationshipType(value, name string) bool {
	return value == name || strings.HasSuffix(value, "/"+name)
}

func (r *Reader) readRelationships(name string) (*relationshipDocument, error) {
	var relationships relationshipDocument
	if err := r.readXML(name, &relationships); err != nil {
		return nil, err
	}
	return &relationships, nil
}

func (r *Reader) readXML(name string, value any) error {
	entry := r.entry(name)
	if entry == nil {
		return xerrors.Newf("XLSX entry %q not found", name)
	}
	data, err := readEntry(entry)
	if err != nil {
		return err
	}
	if err := newXMLDecoder(bytes.NewReader(data)).Decode(value); err != nil {
		return xerrors.Wrapf(err, "decode XLSX entry %q", name)
	}
	return nil
}

func (r *Reader) entry(name string) *zip.File {
	if name == "" {
		return nil
	}
	return r.entries[normalizePartName(name)]
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
