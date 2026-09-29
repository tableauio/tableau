package importer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingWorkbookReader struct {
	err    error
	closed bool
}

func (*failingWorkbookReader) SheetNames() []string { return []string{"Item"} }

func (r *failingWorkbookReader) ReadRows(string) ([][]string, error) { return nil, r.err }

func (r *failingWorkbookReader) Close() error {
	r.closed = true
	return nil
}

func TestCachedExcelFallsBackToExcelize(t *testing.T) {
	directErr := errors.New("direct reader rejected worksheet")
	direct := &failingWorkbookReader{err: directErr}
	content, err := os.ReadFile("testdata/Test.xlsx")
	require.NoError(t, err)
	filename := filepath.Join(t.TempDir(), "Test.xlsx")
	require.NoError(t, os.WriteFile(filename, content, 0o600))
	cached := &cachedExcel{
		filename:  filename,
		rawReader: direct,
	}
	t.Cleanup(func() {
		if cached.excelizeFile != nil {
			_ = cached.excelizeFile.Close()
		}
	})

	require.Equal(t, "xlsx", cached.readerName())
	rows, err := cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.True(t, direct.closed)
	require.Nil(t, cached.rawReader)
	require.NotNil(t, cached.excelizeFile)
	require.Equal(t, "excelize", cached.readerName())

	file, err := cached.openExcelize()
	require.NoError(t, err)
	require.Same(t, cached.excelizeFile, file)
	rows, err = cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
}

func TestCachedExcelReportsFallbackFailure(t *testing.T) {
	directErr := errors.New("direct reader failed")
	direct := &failingWorkbookReader{err: directErr}
	cached := &cachedExcel{
		filename:  filepath.Join(t.TempDir(), "missing.xlsx"),
		rawReader: direct,
	}

	_, err := cached.readRows("Item")
	require.ErrorIs(t, err, directErr)
	require.Error(t, err)
	require.Same(t, direct, cached.rawReader)
}
