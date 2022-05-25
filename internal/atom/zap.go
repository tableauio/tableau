package atom

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var levelMap = map[string]zapcore.Level{
	"DEBUG": zapcore.DebugLevel,
	"INFO":  zapcore.InfoLevel,
	"WARN":  zapcore.WarnLevel,
	"ERROR": zapcore.ErrorLevel,
	"FATAL": zapcore.FatalLevel,
}

var modeMap = map[string]LogModeEncoder{
	"SIMPLE": getSimpleEncoder,
	"FULL":   getFullEncoder,
}

var Log *zap.SugaredLogger
var zaplogger *zap.Logger

// func GetZapLogger() *zap.Logger {
// 	return zaplogger
// }

func init() {
	err := InitConsoleLog("DEBUG", "FULL")
	if err != nil {
		panic(err)
	}
}

func InitConsoleLog(level, mode string) error {
	zapLevel, ok := levelMap[strings.ToUpper(level)]
	if !ok {
		return fmt.Errorf("illegal log level: %s", level)
	}
	modeEncoder, ok := modeMap[strings.ToUpper(mode)]
	if !ok {
		return fmt.Errorf("illegal log mode: %s", mode)
	}
	ws := createConsoleWriter()
	core := zapcore.NewCore(
		modeEncoder(),
		ws,
		zapLevel,
	)
	zaplogger := zap.New(core, zap.AddCaller())
	Log = zaplogger.Sugar()
	return nil
}

func InitFileLog(level string, dir string, filename string) error {
	zapLevel, ok := levelMap[strings.ToUpper(level)]
	if !ok {
		return fmt.Errorf("illegal log level: %s", level)
	}
	ws, err := createFileWriter(dir, filename)
	if err != nil {
		return fmt.Errorf("create file logger failed: %s", err)
	}
	core := zapcore.NewCore(
		getFullEncoder(),
		ws,
		zapLevel,
	)
	zaplogger := zap.New(core, zap.AddCaller())
	// zap.ReplaceGlobals(zaplogger)
	Log = zaplogger.Sugar()

	return nil
}

func NewSugar(name string) *zap.SugaredLogger {
	return zaplogger.Named(name).Sugar()
}

func createConsoleWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}

func createFileWriter(dir string, filename string) (zapcore.WriteSyncer, error) {
	logger, err := createLumberjackLogger(dir, filename)
	if err != nil {
		return nil, fmt.Errorf("create lumberjack logger failed: %s", err)
	}
	return zapcore.AddSync(logger), nil
}

func createLumberjackLogger(dir string, filename string) (*lumberjack.Logger, error) {
	// create output dir
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		return nil, err
	}
	return &lumberjack.Logger{
		Filename:   filepath.Join(dir, filename),
		MaxSize:    100, // megabytes
		MaxAge:     7,   //days
		MaxBackups: 3,
		LocalTime:  true,
		Compress:   true, // disabled by default
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
