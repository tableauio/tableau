package xerrors_test

import (
	"errors"
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
