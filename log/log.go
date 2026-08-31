// Refer:
//
//	https://github.com/go-eden/slf4go
//	https://github.com/go-eden/slf4go-zap
package log

import (
	"fmt"
	"sync"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
	"github.com/tableauio/tableau/log/driver/customdriver"
	"github.com/tableauio/tableau/log/driver/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is the minimal Printf-style logging interface expected by
// SetLogger. *zap.SugaredLogger satisfies it directly; other logging
// systems (e.g. slog, logrus) can be plugged in via a thin adapter.
type Logger = customdriver.Logger

var defaultLogger *sugaredLogger

var gOpts *Options
var atomicLevel zap.AtomicLevel

// mu guards hasCustomLogger, which is read/written by SetLogger and Init.
var mu sync.Mutex
var hasCustomLogger bool

func init() {
	defaultLogger = &sugaredLogger{
		level: core.DebugLevel,
	}
	gOpts = &Options{}
	atomicLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
}

// SetLogger installs a user-provided logger (e.g. *zap.SugaredLogger, or a
// custom adapter around slog/logrus/etc.) as tableau's log destination.
//
// Once set, subsequent calls to Init (triggered internally by
// tableau.GenProto/GenConf via log.Options) will no longer install the
// built-in zap-based driver, so the custom logger keeps taking effect.
func SetLogger(logger Logger) {
	mu.Lock()
	hasCustomLogger = true
	mu.Unlock()
	SetDriver(customdriver.New(logger))
}

// Init initializes the built-in zap-based logger from opts.
//
// NOTE: if a custom logger has already been installed via SetLogger, Init
// only updates the log level used for LevelEnabled, and leaves the custom
// logger driver untouched.
func Init(opts *Options) error {
	gOpts = opts // remember as global options.

	if err := atomicLevel.UnmarshalText([]byte(opts.Level)); err != nil {
		return fmt.Errorf("illegal log level: %s", opts.Level)
	}

	mu.Lock()
	skip := hasCustomLogger
	mu.Unlock()
	if skip {
		return nil
	}

	logger, err := zapdriver.NewLogger(opts.Mode, opts.Level, opts.Filename, opts.Sink)
	if err != nil {
		return err
	}
	SetDriver(zapdriver.NewWithLogger(atomicLevel, logger))
	return nil
}

func Mode() string {
	return gOpts.Mode
}

func Level() string {
	return gOpts.Level
}

func LevelEnabled(lvl zapcore.Level) bool {
	return atomicLevel.Enabled(lvl)
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
