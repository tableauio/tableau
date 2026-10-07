package xerrors_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau"
	"github.com/tableauio/tableau/internal/x/xerrors"
)

// TestPublicWrapKVCaller checks the facade reports the application call site
// and preserves that stack when further metadata is attached.
func TestPublicWrapKVCaller(t *testing.T) {
	cause := errors.New("condition missing")
	_, file, line, ok := runtime.Caller(0)
	wrapped := tableau.WrapKV(cause, tableau.KeyBookName, "Main.xlsx")
	require.True(t, ok)

	var tracer interface{ StackTrace() xerrors.StackTrace }
	require.ErrorAs(t, wrapped, &tracer)
	trace := tracer.StackTrace()
	require.NotEmpty(t, trace)
	frame, _ := runtime.CallersFrames([]uintptr{uintptr(trace[0])}).Next()
	assert.Equal(t, file, frame.File)
	assert.Equal(t, line+1, frame.Line)
	assert.Equal(t, "github.com/tableauio/tableau/internal/x/xerrors_test.TestPublicWrapKVCaller", frame.Function)

	rewrapped := tableau.WrapKV(wrapped, tableau.KeySheetName, "MainSheet")
	var rewrappedTracer interface{ StackTrace() xerrors.StackTrace }
	require.ErrorAs(t, rewrapped, &rewrappedTracer)
	assert.Same(t, tracer, rewrappedTracer)
	require.ErrorIs(t, rewrapped, cause)
	serr := tableau.Inspect(rewrapped)
	require.ErrorIs(t, serr, cause)
	assert.Equal(t, "Main.xlsx", serr.GetValue(tableau.KeyBookName))
	assert.Equal(t, "MainSheet", serr.GetValue(tableau.KeySheetName))
}

func TestCustomCheckError(t *testing.T) {
	assert.Nil(t, tableau.E2032(nil))
	cause := errors.New("condition missing")
	plain := fmt.Errorf("task 42: %w", cause)
	coded := xerrors.E2003("1", 3)
	sourced := &tableau.Error{Details: []*tableau.ErrorDetail{{
		Message: "invalid target",
		Source:  &tableau.SourceLocation{Workbook: "Shard.xlsx", Worksheet: "TaskConfig", PrimaryWorkbook: "Task.xlsx"},
	}}}
	joined := errors.Join(plain, coded, sourced)
	wrapped := tableau.E2032(joined)
	serr := tableau.Inspect(wrapped)
	require.Len(t, serr.Details, 3)
	assert.Equal(t, "E2032", serr.Details[0].Code)
	assert.Equal(t, "task 42: condition missing", serr.Details[0].Message)
	assert.Equal(t, "E2003", serr.Details[1].Code)
	assert.Equal(t, tableau.Inspect(coded).Details[0].Message, serr.Details[1].Message)
	assert.Equal(t, "E2032", serr.Details[2].Code)
	assert.Equal(t, "invalid target", serr.Details[2].Message)
	assert.Equal(t, sourced.Details[0].Source, serr.Details[2].Source)
	assert.Empty(t, sourced.Details[0].Code, "classification must not change the input")
	for _, target := range []error{joined, cause, coded, sourced, xerrors.ErrE2032, xerrors.ErrE2003} {
		require.ErrorIs(t, wrapped, target)
		require.ErrorIs(t, serr, target)
	}
	assert.Equal(t, "error[E2032]: custom check failed\nReason: task 42: condition missing\n",
		tableau.Inspect(tableau.E2032(plain)).Error())

	encoded, err := json.Marshal(serr)
	require.NoError(t, err)
	var decoded tableau.Error
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	assert.Equal(t, serr.Error(), decoded.Error())
}

func TestPublicE2032Caller(t *testing.T) {
	cause := errors.New("condition missing")
	_, file, line, ok := runtime.Caller(0)
	wrapped := tableau.E2032(cause)
	require.True(t, ok)
	var tracer interface{ StackTrace() xerrors.StackTrace }
	require.ErrorAs(t, wrapped, &tracer)
	trace := tracer.StackTrace()
	require.NotEmpty(t, trace)
	frame, _ := runtime.CallersFrames([]uintptr{uintptr(trace[0])}).Next()
	assert.Equal(t, file, frame.File)
	assert.Equal(t, line+1, frame.Line)
	assert.Equal(t, "github.com/tableauio/tableau/internal/x/xerrors_test.TestPublicE2032Caller", frame.Function)

	rewrapped := tableau.E2032(tableau.WrapKV(wrapped, tableau.KeyBookName, "Task.xlsx"))
	var rewrappedTracer interface{ StackTrace() xerrors.StackTrace }
	require.ErrorAs(t, rewrapped, &rewrappedTracer)
	assert.Same(t, tracer, rewrappedTracer)
	require.ErrorIs(t, rewrapped, cause)
	require.Len(t, tableau.Inspect(rewrapped).Details, 1)
}

func TestCustomCheckErrorPreservesSparseCode(t *testing.T) {
	original := &tableau.Error{Details: []*tableau.ErrorDetail{{Code: "E2012", Message: "invalid value"}}}
	for _, err := range []error{original, errors.Join(original, errors.New("condition missing"))} {
		serr := tableau.Inspect(tableau.E2032(err))
		require.NotEmpty(t, serr.Details)
		assert.Equal(t, "E2012", serr.Details[0].Code)
		assert.Empty(t, serr.Details[0].Description, "an existing code must not inherit the custom-check description")
		assert.Equal(t, "invalid value", serr.Details[0].Message)
	}
}
