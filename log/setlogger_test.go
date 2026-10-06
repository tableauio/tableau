package log

import (
	"bytes"
	"fmt"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tableauio/tableau/log/driver/customdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// restoreLoggerState snapshots log package's mutable global state before a
// SetLogger test, and restores it on cleanup, so that tests can run in any
// order without interfering with each other.
func restoreLoggerState(t *testing.T) {
	t.Helper()
	origDriver := defaultLogger.driver
	origOpts := gOpts
	origLevel := atomicLevel.Level()
	mu.Lock()
	origHasCustom := hasCustomLogger
	mu.Unlock()

	t.Cleanup(func() {
		defaultLogger.driver = origDriver
		gOpts = origOpts
		atomicLevel.SetLevel(origLevel)
		mu.Lock()
		hasCustomLogger = origHasCustom
		mu.Unlock()
	})
}

// *zap.SugaredLogger satisfies Logger (Debugf/Infof/.../Fatalf) directly,
// so it can be installed via SetLogger without any adapter.
func TestSetLogger_Zap(t *testing.T) {
	restoreLoggerState(t)

	core, observed := observer.New(zapcore.DebugLevel)
	SetLogger(zap.New(core).Sugar())

	Info("hello")
	Infow("infow msg", "key1", "value1")
	Errorf("errorf msg: %d", 42)

	entries := observed.All()
	require.Len(t, entries, 3)

	assert.Equal(t, zapcore.InfoLevel, entries[0].Level)
	assert.Equal(t, "hello", entries[0].Message)

	assert.Equal(t, zapcore.InfoLevel, entries[1].Level)
	assert.Equal(t, "infow msg key1=value1", entries[1].Message)

	assert.Equal(t, zapcore.ErrorLevel, entries[2].Level)
	assert.Equal(t, "errorf msg: 42", entries[2].Message)
}

func TestSetLogger_ZapPanic(t *testing.T) {
	restoreLoggerState(t)

	core, observed := observer.New(zapcore.DebugLevel)
	SetLogger(zap.New(core).Sugar())

	assert.Panics(t, func() {
		Panic("boom")
	})
	entries := observed.All()
	require.Len(t, entries, 1)
	assert.Equal(t, zapcore.PanicLevel, entries[0].Level)
	assert.Equal(t, "boom", entries[0].Message)
}

// TestSetLogger_NotOverriddenByInit verifies that once a custom logger is
// installed via SetLogger, a subsequent Init call (as done internally by
// tableau.Generate/GenProto/GenConf) does not replace it with the built-in
// zap-based driver.
func TestSetLogger_NotOverriddenByInit(t *testing.T) {
	restoreLoggerState(t)

	core, observed := observer.New(zapcore.DebugLevel)
	SetLogger(zap.New(core).Sugar())

	err := Init(&Options{Mode: "FULL", Level: "INFO", Sink: "CONSOLE"})
	require.NoError(t, err)
	assert.Equal(t, "INFO", Level())

	_, ok := defaultLogger.driver.(*customdriver.CustomDriver)
	assert.True(t, ok, "driver should remain the custom one after Init")

	Info("still routed to custom logger")
	entries := observed.All()
	require.Len(t, entries, 1)
	assert.Equal(t, "still routed to custom logger", entries[0].Message)
}

// slogAdapter adapts a *slog.Logger to the Logger interface, demonstrating
// how to integrate a logging system (such as slog) that doesn't natively
// expose Printf-style methods. Since Logger only requires the 7 Xxxf
// methods, the adapter is a thin, mechanical wrapper.
type slogAdapter struct {
	l *slog.Logger
}

var _ Logger = (*slogAdapter)(nil)

func newSlogAdapter(l *slog.Logger) *slogAdapter {
	return &slogAdapter{l: l}
}

func (a *slogAdapter) Debugf(format string, args ...any) { a.l.Debug(fmt.Sprintf(format, args...)) }
func (a *slogAdapter) Infof(format string, args ...any)  { a.l.Info(fmt.Sprintf(format, args...)) }
func (a *slogAdapter) Warnf(format string, args ...any)  { a.l.Warn(fmt.Sprintf(format, args...)) }
func (a *slogAdapter) Errorf(format string, args ...any) { a.l.Error(fmt.Sprintf(format, args...)) }
func (a *slogAdapter) DPanicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.l.Error(msg)
	panic(msg)
}
func (a *slogAdapter) Panicf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.l.Error(msg)
	panic(msg)
}
func (a *slogAdapter) Fatalf(format string, args ...any) { a.l.Error(fmt.Sprintf(format, args...)) }

func TestSetLogger_Slog(t *testing.T) {
	restoreLoggerState(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	SetLogger(newSlogAdapter(slog.New(handler)))

	Infow("infow msg", "key1", "value1")
	Errorf("errorf msg: %d", 42)

	out := buf.String()
	assert.Contains(t, out, "level=INFO")
	assert.Contains(t, out, `msg="infow msg key1=value1"`)
	assert.Contains(t, out, "level=ERROR")
	assert.Contains(t, out, "errorf msg: 42")
}

func TestSetLogger_SlogPanic(t *testing.T) {
	restoreLoggerState(t)

	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	SetLogger(newSlogAdapter(slog.New(handler)))

	assert.Panics(t, func() {
		Panic("boom")
	})
	assert.Contains(t, buf.String(), "boom")
}
