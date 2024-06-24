// Refer:
//
//	https://github.com/go-eden/slf4go
//	https://github.com/go-eden/slf4go-zap
package log

import (
	"fmt"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
	"github.com/tableauio/tableau/log/driver/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var defaultLogger *Logger

var gOpts *Options

func init() {
	defaultLogger = &Logger{
		level: core.DebugLevel,
		// driver: &defaultdriver.DefaultDriver{
		// 	CallerSkip: 1,
		// },
		driver: zapdriver.New(zap.NewDevelopmentConfig(), zap.AddCallerSkip(4)),
	}
	gOpts = &Options{}
}

func Init(opts *Options) error {
	gOpts = opts // remember as global options.

	zapLogger, err := zapdriver.NewLogger(opts.Mode, opts.Level, opts.Filename, opts.Sink)
	if err != nil {
		return err
	}
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		return fmt.Errorf("illegal log level: %s", opts.Level)
	}
	SetDriver(zapdriver.NewWithLogger(zapLevel, zapLogger))
	return nil
}

func Mode() string {
	return gOpts.Mode
}

func Level() string {
	return gOpts.Level
}

func SetDriver(driver driver.Driver) {
	defaultLogger.driver = driver
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...any) {
	defaultLogger.Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...any) {
	defaultLogger.Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...any) {
	defaultLogger.Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...any) {
	defaultLogger.Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanic(args ...any) {
	defaultLogger.DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...any) {
	defaultLogger.Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...any) {
	defaultLogger.Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...any) {
	defaultLogger.Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...any) {
	defaultLogger.Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...any) {
	defaultLogger.Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...any) {
	defaultLogger.Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...any) {
	defaultLogger.DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...any) {
	defaultLogger.Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...any) {
	defaultLogger.Fatalf(template, args...)
}

// Debugw logs a message with some additional context. The variadic key-value
// pairs are treated as they are in With.
func Debugw(msg string, keysAndValues ...any) {
	defaultLogger.Debugw(msg, keysAndValues...)
}

// Infow logs a message with some additional context.
func Infow(msg string, keysAndValues ...any) {
	defaultLogger.Infow(msg, keysAndValues...)
}

// Warnw logs a message with some additional context.
func Warnw(msg string, keysAndValues ...any) {
	defaultLogger.Warnw(msg, keysAndValues...)
}

// Errorw logs a message with some additional context.
func Errorw(msg string, keysAndValues ...any) {
	defaultLogger.Errorw(msg, keysAndValues...)
}

// DPanicw logs a message with some additional context.
func DPanicw(msg string, keysAndValues ...any) {
	defaultLogger.DPanicw(msg, keysAndValues...)
}

// Panicw logs a message with some additional context.
func Panicw(msg string, keysAndValues ...any) {
	defaultLogger.Panicw(msg, keysAndValues...)
}

// Fatalw logs a message with some additional context.
func Fatalw(msg string, keysAndValues ...any) {
	defaultLogger.Fatalw(msg, keysAndValues...)
}
