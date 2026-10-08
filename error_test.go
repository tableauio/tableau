package tableau_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
)

func TestErrorReporting(t *testing.T) {
	cause := errors.New("condition missing")
	wrapped := tableau.WrapKV(tableau.E0005(cause),
		tableau.KeyBookName, "Task.xlsx", tableau.KeySheetName, "TaskConfig")
	serr := tableau.Inspect(wrapped)
	require.NotNil(t, serr)
	require.ErrorIs(t, serr, cause)
	assert.Equal(t, "Task.xlsx", serr.GetValue(tableau.KeyBookName))

	snapshot := tableau.Inspect(serr)
	require.NotNil(t, snapshot)
	require.ErrorIs(t, snapshot, cause)
	assert.Equal(t, "error[E0005]: custom check failed\nWorkbook: Task.xlsx\nWorksheet: TaskConfig\nReason: condition missing\n", snapshot.Error())
	snapshot.Details[0].Source.Workbook = "Other.xlsx"
	assert.Equal(t, "Task.xlsx", serr.GetValue(tableau.KeyBookName))

	assert.Nil(t, tableau.Inspect(nil))
	assert.Nil(t, tableau.WrapKV(nil))
	assert.Nil(t, tableau.E0005(nil))
	plain := tableau.Inspect(cause)
	require.NotNil(t, plain)
	require.ErrorIs(t, plain, cause)
	require.Len(t, plain.Details, 1)
	assert.Equal(t, cause.Error(), plain.Details[0].Message)
}
