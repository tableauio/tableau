package log

import (
	"os"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
)

// sugaredLogger is tableau's built-in logger implementation, which dispatches
// records to a pluggable driver.Driver (default: zap-based; can be replaced
// by a user-provided logger via SetLogger).
type sugaredLogger struct {
	level  core.Level
	driver driver.Driver
}

func (l *sugaredLogger) Debug(args ...any) {
	l.log(core.DebugLevel, "", args, nil)
}

func (l *sugaredLogger) Info(args ...any) {
	l.log(core.InfoLevel, "", args, nil)
}

func (l *sugaredLogger) Warn(args ...any) {
	l.log(core.WarnLevel, "", args, nil)
}

func (l *sugaredLogger) Error(args ...any) {
	l.log(core.ErrorLevel, "", args, nil)
}

func (l *sugaredLogger) DPanic(args ...any) {
	l.log(core.DPanicLevel, "%+v", args, nil)
	// TODO: panic only in development
	os.Exit(-1)
}

func (l *sugaredLogger) Panic(args ...any) {
	l.log(core.PanicLevel, "%+v", args, nil)
	os.Exit(-1)
}

func (l *sugaredLogger) Fatal(args ...any) {
	l.log(core.FatalLevel, "%+v", args, nil)
	os.Exit(-1)
}

func (l *sugaredLogger) Debugf(format string, args ...any) {
	l.log(core.DebugLevel, format, args, nil)
}

func (l *sugaredLogger) Infof(format string, args ...any) {
	l.log(core.InfoLevel, format, args, nil)
}

func (l *sugaredLogger) Warnf(format string, args ...any) {
	l.log(core.WarnLevel, format, args, nil)
}

func (l *sugaredLogger) Errorf(format string, args ...any) {
	l.log(core.ErrorLevel, format, args, nil)
}

func (l *sugaredLogger) DPanicf(format string, args ...any) {
	l.log(core.DPanicLevel, format, args, nil)
	// TODO: panic only in development
	panic("log panic")
}

func (l *sugaredLogger) Panicf(format string, args ...any) {
	l.log(core.PanicLevel, format, args, nil)
	panic("log panic")
}

func (l *sugaredLogger) Fatalf(format string, args ...any) {
	l.log(core.FatalLevel, format, args, nil)
	os.Exit(-1)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func (l *sugaredLogger) Debugw(msg string, keysAndValues ...any) {
	l.log(core.DebugLevel, msg, nil, keysAndValues)
}

// Infow logs a message with some additional context.
func (l *sugaredLogger) Infow(msg string, keysAndValues ...any) {
	l.log(core.InfoLevel, msg, nil, keysAndValues)
}

// Warnw logs a message with some additional context.
func (l *sugaredLogger) Warnw(msg string, keysAndValues ...any) {
	l.log(core.WarnLevel, msg, nil, keysAndValues)
}

// Errorw logs a message with some additional context.
func (l *sugaredLogger) Errorw(msg string, keysAndValues ...any) {
	l.log(core.ErrorLevel, msg, nil, keysAndValues)
}

// DPanicw logs a message with some additional context.
func (l *sugaredLogger) DPanicw(msg string, keysAndValues ...any) {
	l.log(core.DPanicLevel, msg, nil, keysAndValues)
}

// Panicw logs a message with some additional context.
func (l *sugaredLogger) Panicw(msg string, keysAndValues ...any) {
	l.log(core.PanicLevel, msg, nil, keysAndValues)
}

// Fatalw logs a message with some additional context.
func (l *sugaredLogger) Fatalw(msg string, keysAndValues ...any) {
	l.log(core.FatalLevel, msg, nil, keysAndValues)
}

func (l *sugaredLogger) log(lvl core.Level, format string, fmtArgs []any, kvs []any) {
	// If logging at this level is completely disabled, skip the overhead of
	// string formatting.
	// if lvl < DPanicLevel {
	// 	return
	// }

	if l.driver == nil {
		return
	}

	r := &core.Record{
		Level:  lvl,
		Format: &format,
		Args:   fmtArgs,

		KVs: kvs,
	}

	l.driver.Print(r)
}
