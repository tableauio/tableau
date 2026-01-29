package log

import (
	"github.com/tableauio/tableau/log/driver/zapdriver"
	"go.uber.org/zap"
)

var defaultLogger LoggerIface = zap.NewNop().Sugar()

func Init(opts *Options) error {
	logger, err := zapdriver.NewLogger(opts.Mode, opts.Level, opts.Filename, opts.Sink)
	if err != nil {
		return err
	}
	SetLogger(logger.Sugar())
	return nil
}

func SetLogger(logger LoggerIface) {
	defaultLogger = logger
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
