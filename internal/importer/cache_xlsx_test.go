package importer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/internal/importer/xlsx"
)

type failingWorkbookReader struct {
	err    error
	closed bool
}

func (*failingWorkbookReader) SheetNames() []string { return []string{"Item"} }

func (r *failingWorkbookReader) ReadRows(string, uint) ([][]string, error) { return nil, r.err }

func (r *failingWorkbookReader) Close() error {
	r.closed = true
	return nil
}

func TestCachedExcelFallsBackToExcelize(t *testing.T) {
	directErr := errors.Join(xlsx.ErrUnsupported, errors.New("direct reader rejected worksheet"))
	direct := &failingWorkbookReader{err: directErr}
	content, err := os.ReadFile("testdata/Test.xlsx")
	require.NoError(t, err)
	filename := filepath.Join(t.TempDir(), "Test.xlsx")
	require.NoError(t, os.WriteFile(filename, content, 0o600))
	cached := &cachedExcel{
		filename: filename,
		reader:   direct,
		useXLSX:  true,
	}
	t.Cleanup(func() {
		_ = cached.reader.Close()
	})

	require.Equal(t, "xlsx", cached.readerName())
	rows, err := cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.True(t, direct.closed)
	require.NotEqual(t, direct, cached.reader)
	require.Equal(t, "excelize", cached.readerName())

	fallback := cached.reader
	rows, err = cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.Equal(t, fallback, cached.reader)
}

func TestCachedExcelReportsFallbackFailure(t *testing.T) {
	directErr := errors.Join(xlsx.ErrUnsupported, errors.New("direct reader failed"))
	direct := &failingWorkbookReader{err: directErr}
	cached := &cachedExcel{
		filename: filepath.Join(t.TempDir(), "missing.xlsx"),
		reader:   direct,
		useXLSX:  true,
	}

	_, err := cached.readRows("Item")
	require.ErrorIs(t, err, directErr)
	require.Error(t, err)
	require.Same(t, direct, cached.reader)
	require.False(t, direct.closed)
}

func TestCachedExcelDoesNotRetryDataErrors(t *testing.T) {
	directErr := errors.New("invalid shared string index")
	direct := &failingWorkbookReader{err: directErr}
	cached := &cachedExcel{
		filename: filepath.Join(t.TempDir(), "missing.xlsx"),
		reader:   direct,
		useXLSX:  true,
	}
	_, err := cached.readRows("Item")
	require.ErrorIs(t, err, directErr)
	require.False(t, direct.closed)
	require.Same(t, direct, cached.reader)
}
