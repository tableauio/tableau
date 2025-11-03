package printer

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strings"
)

// Indent indents each depth two spaces "  ".
func Indent(depth int) string {
	return strings.Repeat("  ", depth)
}

type Printer struct {
	buf bytes.Buffer
}

// New creates a new printer.
func New() *Printer {
	return &Printer{}
}

// P prints a line to the printer. It converts each parameter to a
// string following the same rules as fmt.Print. It never inserts spaces
// between parameters.
func (p *Printer) P(v ...any) {
	for _, x := range v {
		fmt.Fprint(&p.buf, x)
	}
	fmt.Fprintln(&p.buf)
}

// Bytes returns the bytes content of printer.
func (p *Printer) Bytes() []byte {
	return p.buf.Bytes()
}

// String returns the string content of printer.
func (p *Printer) String() string {
	return p.buf.String()
}

// Save saves the printer content to a file.
func (p *Printer) Save(filename string) error {
	return save(filename, p.Bytes())
}

// SaveWithGoFormat saves the printer content to a file with go format.
func (p *Printer) SaveWithGoFormat(filename string) error {
	// Format the source code
	formatted, err := format.Source(p.Bytes())
	if err != nil {
		return err
	}
	return save(filename, formatted)
}

func save(filename string, content []byte) error {
	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(content)
	return err
}
