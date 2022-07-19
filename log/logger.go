package log

import (
	"os"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
)

type LoggerIface interface {
	// Debug uses fmt.Sprint to construct and log a message.
	Debug(args ...interface{})

	// Info uses fmt.Sprint to construct and log a message.
	Info(args ...interface{})

	// Warn uses fmt.Sprint to construct and log a message.
	Warn(args ...interface{})

	// Error uses fmt.Sprint to construct and log a message.
	Error(args ...interface{})

	// DPanic uses fmt.Sprint to construct and log a message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanic(args ...interface{})

	// Panic uses fmt.Sprint to construct and log a message, then panics.
	Panic(args ...interface{})

	// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
	Fatal(args ...interface{})

	// Debugf uses fmt.Sprintf to log a templated message.
	Debugf(format string, args ...interface{})

	// Infof uses fmt.Sprintf to log a templated message.
	Infof(format string, args ...interface{})

	// Warnf uses fmt.Sprintf to log a templated message.
	Warnf(format string, args ...interface{})

	// Errorf uses fmt.Sprintf to log a templated message.
	Errorf(format string, args ...interface{})

	// DPanicf uses fmt.Sprintf to log a templated message. In development, the
	// logger then panics. (See DPanicLevel for details.)
	DPanicf(format string, args ...interface{})

	// Panicf uses fmt.Sprintf to log a templated message, then panics.
	Panicf(format string, args ...interface{})

	// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
	Fatalf(format string, args ...interface{})
}

type Logger struct {
	level  core.Level
	driver driver.Driver
}

func (l *Logger) Debug(args ...interface{}) {
	l.log(core.DebugLevel, "", args, nil)
}

func (l *Logger) Info(args ...interface{}) {
	l.log(core.InfoLevel, "", args, nil)
}

func (l *Logger) Warn(args ...interface{}) {
	l.log(core.WarnLevel, "", args, nil)
}

func (l *Logger) Error(args ...interface{}) {
	l.log(core.ErrorLevel, "", args, nil)
}

func (l *Logger) DPanic(args ...interface{}) {
	l.log(core.DPanicLevel, "", args, nil)
	// TODO: panic only in development
	panic("log panic")
}

func (l *Logger) Panic(args ...interface{}) {
	l.log(core.PanicLevel, "", args, nil)
	panic("log panic")
}

func (l *Logger) Fatal(args ...interface{}) {
	l.log(core.FatalLevel, "", args, nil)
	os.Exit(-1)
}

func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(core.DebugLevel, format, args, nil)
}

func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(core.InfoLevel, format, args, nil)
}

func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(core.WarnLevel, format, args, nil)
}

func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(core.ErrorLevel, format, args, nil)
}

func (l *Logger) DPanicf(format string, args ...interface{}) {
	l.log(core.DPanicLevel, format, args, nil)
	// TODO: panic only in development
	panic("log panic")
}

func (l *Logger) Panicf(format string, args ...interface{}) {
	l.log(core.PanicLevel, format, args, nil)
	panic("log panic")
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(core.FatalLevel, format, args, nil)
	os.Exit(-1)
}

func (s *Logger) log(lvl core.Level, format string, fmtArgs []interface{}, context []interface{}) {
	// If logging at this level is completely disabled, skip the overhead of
	// string formatting.
	// if lvl < DPanicLevel {
	// 	return
	// }

	r := &core.Record{
		Level:  lvl,
		Format: &format,
		Args:   fmtArgs,
	}

	s.driver.Print(r)
}
