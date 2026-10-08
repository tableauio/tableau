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
	normalized := tableau.Normalize(wrapped)
	var serr *tableau.Error
	require.ErrorAs(t, normalized, &serr)
	require.ErrorIs(t, normalized, cause)
	assert.Equal(t, "Task.xlsx", serr.GetValue(tableau.KeyBookName))

	snapshot := tableau.Inspect(normalized)
	require.NotNil(t, snapshot)
	require.ErrorIs(t, snapshot, cause)
	assert.Equal(t, "error[E0005]: custom check failed\nWorkbook: Task.xlsx\nWorksheet: TaskConfig\nReason: condition missing\n", snapshot.Error())
	snapshot.Details[0].Source.Workbook = "Other.xlsx"
	assert.Equal(t, "Task.xlsx", serr.GetValue(tableau.KeyBookName))

	assert.Nil(t, tableau.Inspect(nil))
	assert.Nil(t, tableau.Normalize(nil))
	assert.Nil(t, tableau.WrapKV(nil))
	assert.Nil(t, tableau.E0005(nil))
	assert.Same(t, cause, tableau.Normalize(cause))
	assert.Same(t, normalized, tableau.Normalize(normalized))
}
