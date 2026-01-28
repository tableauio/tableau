package log

import (
	"os"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
)

type LoggerIface interface {
	// Debug uses fmt.Sprint to construct and log a message.
	Debug(args ...any)

	// Info uses fmt.Sprint to construct and log a message.
	Info(args ...any)

	// Warn uses fmt.Sprint to construct and log a message.
	Warn(args ...any)

	// Error uses fmt.Sprint to construct and log a message.
	Error(args ...any)

	// DPanic uses fmt.Sprint to construct and log a message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanic(args ...any)

	// Panic uses fmt.Sprint to construct and log a message, then panics.
	Panic(args ...any)

	// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
	Fatal(args ...any)

	// Debugf uses fmt.Sprintf to log a templated message.
	Debugf(format string, args ...any)

	// Infof uses fmt.Sprintf to log a templated message.
	Infof(format string, args ...any)

	// Warnf uses fmt.Sprintf to log a templated message.
	Warnf(format string, args ...any)

	// Errorf uses fmt.Sprintf to log a templated message.
	Errorf(format string, args ...any)

	// DPanicf uses fmt.Sprintf to log a templated message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanicf(format string, args ...any)

	// Panicf uses fmt.Sprintf to log a templated message, then panics.
	Panicf(format string, args ...any)

	// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
	Fatalf(format string, args ...any)
}

type Logger struct {
	level  core.Level
	driver driver.Driver
}

func (l *Logger) Debug(args ...any) {
	l.log(core.DebugLevel, "", args, nil)
}

func (l *Logger) Info(args ...any) {
	l.log(core.InfoLevel, "", args, nil)
}

func (l *Logger) Warn(args ...any) {
	l.log(core.WarnLevel, "", args, nil)
}

func (l *Logger) Error(args ...any) {
	l.log(core.ErrorLevel, "", args, nil)
}

func (l *Logger) DPanic(args ...any) {
	l.log(core.DPanicLevel, "%+v", args, nil)
	// TODO: panic only in development
	os.Exit(-1)
}

func (l *Logger) Panic(args ...any) {
	l.log(core.PanicLevel, "%+v", args, nil)
	os.Exit(-1)
}

func (l *Logger) Fatal(args ...any) {
	l.log(core.FatalLevel, "%+v", args, nil)
	os.Exit(-1)
}

func (l *Logger) Debugf(format string, args ...any) {
	l.log(core.DebugLevel, format, args, nil)
}

func (l *Logger) Infof(format string, args ...any) {
	l.log(core.InfoLevel, format, args, nil)
}

func (l *Logger) Warnf(format string, args ...any) {
	l.log(core.WarnLevel, format, args, nil)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.log(core.ErrorLevel, format, args, nil)
}

func (l *Logger) DPanicf(format string, args ...any) {
	l.log(core.DPanicLevel, format, args, nil)
	// TODO: panic only in development
	panic("log panic")
}

func (l *Logger) Panicf(format string, args ...any) {
	l.log(core.PanicLevel, format, args, nil)
	panic("log panic")
}

func (l *Logger) Fatalf(format string, args ...any) {
	l.log(core.FatalLevel, format, args, nil)
	os.Exit(-1)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func (l *Logger) Debugw(msg string, keysAndValues ...any) {
	l.log(core.DebugLevel, msg, nil, keysAndValues)
}

// Infow logs a message with some additional context.
func (l *Logger) Infow(msg string, keysAndValues ...any) {
	l.log(core.InfoLevel, msg, nil, keysAndValues)
}

// Warnw logs a message with some additional context.
func (l *Logger) Warnw(msg string, keysAndValues ...any) {
	l.log(core.WarnLevel, msg, nil, keysAndValues)
}

// Errorw logs a message with some additional context.
func (l *Logger) Errorw(msg string, keysAndValues ...any) {
	l.log(core.ErrorLevel, msg, nil, keysAndValues)
}

// DPanicw logs a message with some additional context.
func (l *Logger) DPanicw(msg string, keysAndValues ...any) {
	l.log(core.DPanicLevel, msg, nil, keysAndValues)
}

// Panicw logs a message with some additional context.
func (l *Logger) Panicw(msg string, keysAndValues ...any) {
	l.log(core.PanicLevel, msg, nil, keysAndValues)
}

// Fatalw logs a message with some additional context.
func (l *Logger) Fatalw(msg string, keysAndValues ...any) {
	l.log(core.FatalLevel, msg, nil, keysAndValues)
}

func (l *Logger) log(lvl core.Level, format string, fmtArgs []any, kvs []any) {
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
