package printer_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tableauio/tableau/internal/printer"
	"github.com/tableauio/tableau/internal/x/xfs"
)

func init() {
	os.MkdirAll("_out", xfs.DefaultDirPerm)
}

func TestPrinter(t *testing.T) {
	p := printer.New()
	p.P("test")
	assert.Equal(t, "test\n", p.String())
	assert.Equal(t, []byte("test\n"), p.Bytes())
	err := p.Save("_out/printer.txt")
	assert.NoError(t, err)
	err = p.SaveWithGoFormat("_out/printer_go.txt")
	assert.NoError(t, err)
}

func TestIndent(t *testing.T) {
	str := printer.Indent(1)
	assert.Equal(t, "  ", str)
	str = printer.Indent(2)
	assert.Equal(t, "    ", str)
}
