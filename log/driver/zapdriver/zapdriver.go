package zapdriver

import (
	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type ZapDriver struct {
	logger *zap.Logger
	level  zapcore.Level
}

// SkipUntilTrueCaller is the skip level which prints out the actual caller instead of slf4go or slf4go-zap wrappers
const SkipUntilTrueCaller = 4

func init() {
	dr := New(zap.NewProductionConfig(), []zap.Option{zap.AddCallerSkip(SkipUntilTrueCaller)})
	driver.RegisteDriver(dr)
}

// New creates the driver using the provided config wrapper
func New(config zap.Config, opts []zap.Option) *ZapDriver {
	logger, err := config.Build(opts...)
	if err != nil {
		panic(err)
	}
	return &ZapDriver{
		logger: logger,
		level:  config.Level.Level(),
	}
}

func NewWithLogger(level zapcore.Level, logger *zap.Logger) *ZapDriver {
	return &ZapDriver{
		logger: logger,
		level:  level,
	}
}

func (*ZapDriver) Name() string {
	return "zap"
}

func (d *ZapDriver) Print(r *core.Record) {
	logger := d.logger
	// append fields
	if r.Fields != nil {
		fields := make([]zap.Field, 0, len(r.Fields))
		for k, v := range r.Fields {
			fields = append(fields, zap.Any(k, v))
		}
		logger = d.logger.With(fields...)
	}

	defer logger.Sync()
	switch r.Level {
	case core.DebugLevel:
		if r.Format == nil {
			logger.Sugar().Debug(r.Args...)
		} else {
			logger.Sugar().Debugf(*r.Format, r.Args...)
		}
	case core.InfoLevel:
		if r.Format == nil {
			logger.Sugar().Info(r.Args...)
		} else {
			logger.Sugar().Infof(*r.Format, r.Args...)
		}
	case core.WarnLevel:
		if r.Format == nil {
			logger.Sugar().Warn(r.Args...)
		} else {
			logger.Sugar().Warnf(*r.Format, r.Args...)
		}
	case core.ErrorLevel:
		if r.Format == nil {
			logger.Sugar().Error(r.Args...)
		} else {
			logger.Sugar().Errorf(*r.Format, r.Args...)
		}
	case core.PanicLevel:
		if r.Format == nil {
			logger.Sugar().Panic(r.Args...)
		} else {
			logger.Sugar().Panicf(*r.Format, r.Args...)
		}
	case core.FatalLevel:
		if r.Format == nil {
			logger.Sugar().Fatal(r.Args...)
		} else {
			logger.Sugar().Fatalf(*r.Format, r.Args...)
		}
	}
}

func (d *ZapDriver) GetLevel(logger string) core.Level {
	switch d.level {
	case zap.DebugLevel:
		return core.DebugLevel
	case zap.InfoLevel:
		return core.InfoLevel
	case zap.WarnLevel:
		return core.WarnLevel
	case zap.ErrorLevel:
		return core.ErrorLevel
	case zap.DPanicLevel:
		return core.PanicLevel
	case zap.PanicLevel:
		return core.PanicLevel
	case zap.FatalLevel:
		return core.FatalLevel
	default:
		return core.DebugLevel
	}
}
