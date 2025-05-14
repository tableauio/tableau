package zapdriver

import (
	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	_oddNumberErrMsg    = "Ignored key without a value."
	_nonStringKeyErrMsg = "Ignored key-value pairs with non-string keys."
)

type ZapDriver struct {
	logger *zap.Logger
	level  zapcore.Level
}

// SkipUntilTrueCaller is the skip level which prints out the actual caller instead of slf4go or slf4go-zap wrappers
const SkipUntilTrueCaller = 4

func init() {
	dr := New(zap.NewProductionConfig(), zap.AddCallerSkip(SkipUntilTrueCaller))
	driver.RegisteDriver(dr)
}

// New creates the driver using the provided config wrapper
func New(config zap.Config, opts ...zap.Option) *ZapDriver {
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

// refer: https://github.com/uber-go/zap/blob/v1.21.0/sugar.go#L249
func (d *ZapDriver) sweetenFields(args []any) []zap.Field {
	if len(args) == 0 {
		return nil
	}

	// Allocate enough space for the worst case; if users pass only structured
	// fields, we shouldn't penalize them with extra allocations.
	fields := make([]zap.Field, 0, len(args))
	var invalid invalidPairs

	for i := 0; i < len(args); {
		// This is a strongly-typed field. Consume it and move on.
		if f, ok := args[i].(zap.Field); ok {
			fields = append(fields, f)
			i++
			continue
		}

		// Make sure this element isn't a dangling key.
		if i == len(args)-1 {
			d.logger.Error(_oddNumberErrMsg, zap.Any("ignored", args[i]))
			break
		}

		// Consume this value and the next, treating them as a key-value pair. If the
		// key isn't a string, add this pair to the slice of invalid pairs.
		key, val := args[i], args[i+1]
		if keyStr, ok := key.(string); !ok {
			// Subsequent errors are likely, so allocate once up front.
			if cap(invalid) == 0 {
				invalid = make(invalidPairs, 0, len(args)/2)
			}
			invalid = append(invalid, invalidPair{i, key, val})
		} else {
			fields = append(fields, zap.Any(keyStr, val))
		}
		i += 2
	}

	// If we encountered any invalid key-value pairs, log an error.
	if len(invalid) > 0 {
		d.logger.Error(_nonStringKeyErrMsg, zap.Array("invalid", invalid))
	}
	return fields
}

func (d *ZapDriver) Print(r *core.Record) {
	logger := d.logger
	// append key value pairs to the message
	if r.KVs != nil {
		fields := d.sweetenFields(r.KVs)
		logger = d.logger.With(fields...)
	}

	defer func() {
		// For console: sync /dev/stderr: invalid argument
		// TODO: fix it
		_ = logger.Sync()
	}()
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

type invalidPair struct {
	position   int
	key, value any
}

func (p invalidPair) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddInt64("position", int64(p.position))
	zap.Any("key", p.key).AddTo(enc)
	zap.Any("value", p.value).AddTo(enc)
	return nil
}

type invalidPairs []invalidPair

func (ps invalidPairs) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	var err error
	for i := range ps {
		err = multierr.Append(err, enc.AppendObject(ps[i]))
	}
	return err
}
