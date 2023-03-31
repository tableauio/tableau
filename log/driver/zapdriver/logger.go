package zapdriver

import (
	"fmt"
	"os"
	"strings"

	"github.com/tableauio/tableau/log/core"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var modeMap = map[string]LogModeEncoder{
	"SIMPLE": getSimpleEncoder,
	"FULL":   getFullEncoder,
}

// Init set the log options for debugging.
func NewLogger(mode, level, filename, sink string) (*zap.Logger, error) {
	sinkType, err := core.GetSinkType(sink)
	if err != nil {
		return nil, err
	}
	switch sinkType {
	case core.SinkFile:
		return newFileLogger(mode, level, filename)
	case core.SinkMulti:
		return newMultiLogger(mode, level, filename)
	default:
		return newConsoleLogger(mode, level)
	}
}

// newConsoleLogger set the console log level and mode for debugging.
func newConsoleLogger(mode, level string) (*zap.Logger, error) {
	core, err := createConsoleZapCore(mode, level)
	if err != nil {
		return nil, err
	}
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)), nil
}

// newFileLogger set the file log level and filename for debugging.
func newFileLogger(mode, level, filename string) (*zap.Logger, error) {
	core, err := createFileZapCore(mode, level, filename)
	if err != nil {
		return nil, err
	}
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)), nil
}

// newMultiLogger set the log mode, level, filename for debugging.
// The logger will print both to console and files.
func newMultiLogger(mode, level, filename string) (*zap.Logger, error) {
	core, err := createMultiZapCore(mode, level, filename)
	if err != nil {
		return nil, err
	}
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)), nil
}

func getEncoderAndLevel(mode, level string) (LogModeEncoder, zapcore.Level, error) {
	modeEncoder, ok := modeMap[strings.ToUpper(mode)]
	if !ok {
		return nil, zapcore.DebugLevel, fmt.Errorf("illegal log mode: %s", mode)
	}
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		return nil, zapcore.DebugLevel, fmt.Errorf("illegal log level: %s", level)
	}
	return modeEncoder, zapLevel, nil
}

func createConsoleZapCore(mode, level string) (zapcore.Core, error) {
	modeEncoder, zapLevel, err := getEncoderAndLevel(mode, level)
	if err != nil {
		return nil, err
	}
	// stdout level enabler
	stdoutLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapLevel && level <= zapcore.InfoLevel
	})

	// stderr level enabler
	stderrLevel := zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		return level >= zapLevel && level >= zapcore.ErrorLevel
	})

	// write syncers
	stdoutSyncer := zapcore.Lock(os.Stdout)
	stderrSyncer := zapcore.Lock(os.Stderr)

	// tee core
	core := zapcore.NewTee(
		zapcore.NewCore(
			modeEncoder(),
			stdoutSyncer,
			stdoutLevel,
		),
		zapcore.NewCore(
			modeEncoder(),
			stderrSyncer,
			stderrLevel,
		),
	)
	return core, nil
}

func createFileZapCore(mode, level, filename string) (zapcore.Core, error) {
	modeEncoder, zapLevel, err := getEncoderAndLevel(mode, level)
	if err != nil {
		return nil, err
	}
	ws, err := createFileWriter(filename)
	if err != nil {
		return nil, fmt.Errorf("create file logger failed: %s", err)
	}
	core := zapcore.NewCore(
		modeEncoder(),
		ws,
		zapLevel,
	)
	return core, nil
}

func createMultiZapCore(mode, level, filename string) (zapcore.Core, error) {
	consoleZapCore, err := createConsoleZapCore(mode, level)
	if err != nil {
		return nil, err
	}

	fileZapCore, err := createFileZapCore(mode, level, filename)
	if err != nil {
		return nil, err
	}

	// tee core
	core := zapcore.NewTee(
		consoleZapCore,
		fileZapCore,
	)
	return core, nil
}

func createFileWriter(filename string) (zapcore.WriteSyncer, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, fmt.Errorf("log filename can not be empty")
	}
	logger, err := createLumberjackLogger(filename)
	if err != nil {
		return nil, fmt.Errorf("create lumberjack logger failed: %s", err)
	}
	return zapcore.AddSync(logger), nil
}

func createLumberjackLogger(filename string) (*lumberjack.Logger, error) {
	// create output dir
	// dir := filepath.Dir(filename)
	// err := os.MkdirAll(dir, 0700)
	// if err != nil {
	// 	return nil, err
	// }
	return &lumberjack.Logger{
		Filename:   filename,
		MaxSize:    10, // megabytes
		MaxAge:     30, //days
		MaxBackups: 7,
		LocalTime:  true,
		// Compress:   true, // disabled by default
	}, nil
}

type LogModeEncoder func() zapcore.Encoder

func getSimpleEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.CallerKey = ""
	encoderConfig.FunctionKey = ""
	encoderConfig.EncodeTime = nil
	encoderConfig.EncodeLevel = nil
	encoderConfig.ConsoleSeparator = "|"
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getFullEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.FunctionKey = "func"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.ConsoleSeparator = "|"
	return zapcore.NewConsoleEncoder(encoderConfig)
}
