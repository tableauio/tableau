package log

import (
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var zaplogger *zap.Logger

// SkipUntilTrueCaller is the skip level which prints out the actual caller instead of slf4go or slf4go-zap wrappers
const SkipUntilTrueCaller = 1

func init() {
	err := InitConsoleLog("FULL", "DEBUG")
	if err != nil {
		panic(err)
	}
}

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

type SinkType int

const (
	SinkConsole SinkType = iota // default
	SinkFile
	SinkMulti
)

var sinkMap = map[string]SinkType{
	"":        SinkConsole,
	"CONSOLE": SinkConsole,
	"FILE":    SinkFile,
	"MULTI":   SinkMulti,
}

func GetSinkType(sink string) (SinkType, error) {
	sinkType, ok := sinkMap[strings.ToUpper(sink)]
	if !ok {
		return SinkConsole, fmt.Errorf("illegal sink: %s", sink)
	}
	return sinkType, nil
}

func updateLogger(logger *zap.Logger) {
	zaplogger = logger
}

// InitConsoleLog set the console log level and mode for debugging.
func InitConsoleLog(mode, level string) error {
	modeEncoder, zapLevel, err := getEncoderAndLevel(mode, level)
	if err != nil {
		return err
	}
	ws := createConsoleWriter()
	core := zapcore.NewCore(
		modeEncoder(),
		ws,
		zapLevel,
	)
	updateLogger(zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)))
	return nil
}

// InitFileLog set the file log level and filename for debugging.
func InitFileLog(mode, level, filename string) error {
	modeEncoder, zapLevel, err := getEncoderAndLevel(mode, level)
	if err != nil {
		return err
	}
	ws, err := createFileWriter(filename)
	if err != nil {
		return fmt.Errorf("create file logger failed: %s", err)
	}
	core := zapcore.NewCore(
		modeEncoder(),
		ws,
		zapLevel,
	)
	updateLogger(zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)))
	return nil
}

// InitMultiLog set the log mode, level, filename for debugging.
// The logger will print both to console and files.
func InitMultiLog(mode, level, filename string) error {
	modeEncoder, zapLevel, err := getEncoderAndLevel(mode, level)
	if err != nil {
		return err
	}
	consoleSyncer := createConsoleWriter()
	fileSyncer, err := createFileWriter(filename)
	if err != nil {
		return fmt.Errorf("create file logger failed: %s", err)
	}
	core := zapcore.NewCore(
		modeEncoder(),
		zapcore.NewMultiWriteSyncer(
			consoleSyncer,
			fileSyncer,
		),
		zapLevel,
	)
	updateLogger(zap.New(core, zap.AddCaller(), zap.AddCallerSkip(SkipUntilTrueCaller)))
	return nil
}

func getEncoderAndLevel(mode, level string) (LogModeEncoder, zapcore.Level, error) {
	modeEncoder, ok := modeMap[strings.ToUpper(mode)]
	if !ok {
		return nil, zapcore.DebugLevel, fmt.Errorf("illegal log mode: %s", mode)
	}
	zapLevel, ok := levelMap[strings.ToUpper(level)]
	if !ok {
		return nil, zapcore.DebugLevel, fmt.Errorf("illegal log level: %s", level)
	}
	return modeEncoder, zapLevel, nil
}

func NewSugar(name string) *zap.SugaredLogger {
	return zaplogger.Named(name).Sugar()
}

func createConsoleWriter() zapcore.WriteSyncer {
	return zapcore.AddSync(os.Stdout)
}

func createFileWriter(filename string) (zapcore.WriteSyncer, error) {
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
