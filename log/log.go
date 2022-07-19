// Refer:
// 	https://github.com/go-eden/slf4go
// 	https://github.com/go-eden/slf4go-zap
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

func init() {
	defaultLogger = &Logger{
		level: core.DebugLevel,
		// driver: &defaultdriver.DefaultDriver{
		// 	CallerSkip: 1,
		// },
		driver: zapdriver.New(zap.NewDevelopmentConfig(), []zap.Option{zap.AddCallerSkip(4)}),
	}
}

func Init(opts *Options) error {
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

func SetDriver(driver driver.Driver) {
	defaultLogger.driver = driver
}

// Debug uses fmt.Sprint to construct and log a message.
func Debug(args ...interface{}) {
	defaultLogger.Debug(args...)
}

// Info uses fmt.Sprint to construct and log a message.
func Info(args ...interface{}) {
	defaultLogger.Info(args...)
}

// Warn uses fmt.Sprint to construct and log a message.
func Warn(args ...interface{}) {
	defaultLogger.Warn(args...)
}

// Error uses fmt.Sprint to construct and log a message.
func Error(args ...interface{}) {
	defaultLogger.Error(args...)
}

// DPanic uses fmt.Sprint to construct and log a message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanic(args ...interface{}) {
	defaultLogger.DPanic(args...)
}

// Panic uses fmt.Sprint to construct and log a message, then panics.
func Panic(args ...interface{}) {
	defaultLogger.Panic(args...)
}

// Fatal uses fmt.Sprint to construct and log a message, then calls os.Exit.
func Fatal(args ...interface{}) {
	defaultLogger.Fatal(args...)
}

// Debugf uses fmt.Sprintf to log a templated message.
func Debugf(template string, args ...interface{}) {
	defaultLogger.Debugf(template, args...)
}

// Infof uses fmt.Sprintf to log a templated message.
func Infof(template string, args ...interface{}) {
	defaultLogger.Infof(template, args...)
}

// Warnf uses fmt.Sprintf to log a templated message.
func Warnf(template string, args ...interface{}) {
	defaultLogger.Warnf(template, args...)
}

// Errorf uses fmt.Sprintf to log a templated message.
func Errorf(template string, args ...interface{}) {
	defaultLogger.Errorf(template, args...)
}

// DPanicf uses fmt.Sprintf to log a templated message. In development, the
// logger then panics. (See DPanicLevel for details.)
func DPanicf(template string, args ...interface{}) {
	defaultLogger.DPanicf(template, args...)
}

// Panicf uses fmt.Sprintf to log a templated message, then panics.
func Panicf(template string, args ...interface{}) {
	defaultLogger.Panicf(template, args...)
}

// Fatalf uses fmt.Sprintf to log a templated message, then calls os.Exit.
func Fatalf(template string, args ...interface{}) {
	defaultLogger.Fatalf(template, args...)
}
