package importer

import (
	"errors"
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
	cached := &cachedExcel{
		filename: "testdata/Test.xlsx",
		reader:   direct,
	}
	t.Cleanup(func() { require.NoError(t, cached.close()) })

	require.Equal(t, "xlsx", cached.readerName())
	rows, err := cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.True(t, direct.closed)
	require.Nil(t, cached.reader)
	require.NotNil(t, cached.file)
	require.Equal(t, "excelize", cached.readerName())

	file, err := cached.openExcelize()
	require.NoError(t, err)
	require.Same(t, cached.file, file)
	rows, err = cached.readRows("Item")
	require.NoError(t, err)
	require.NotEmpty(t, rows)
}

func TestCachedExcelReportsFallbackFailure(t *testing.T) {
	directErr := errors.New("direct reader failed")
	direct := &failingWorkbookReader{err: directErr}
	cached := &cachedExcel{
		filename: filepath.Join(t.TempDir(), "missing.xlsx"),
		reader:   direct,
	}
	t.Cleanup(func() { require.NoError(t, cached.close()) })

	_, err := cached.readRows("Item")
	require.ErrorIs(t, err, directErr)
	require.Error(t, err)
	require.False(t, direct.closed)
	require.Same(t, direct, cached.reader)
}
