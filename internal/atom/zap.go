package atom

import (
	"errors"
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var levelMap = map[string]zapcore.Level{
	"debug": zapcore.DebugLevel,
	"info":  zapcore.InfoLevel,
	"warn":  zapcore.WarnLevel,
	"error": zapcore.ErrorLevel,
}

var Log *zap.SugaredLogger

func init() {
	err := InitZap("debug")
	if err != nil {
		panic(err)
	}
}

func InitZap(level string) error {
	zapLevel, ok := levelMap[level]
	if !ok {
		log.Fatalf("illegal log level: %s", level)
		return errors.New("illegal log level")
	}
	writeSyncer := zapcore.AddSync(os.Stdout)
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapLevel)

	zaplogger := zap.New(core, zap.AddCaller())
	Log = zaplogger.Sugar()

	// Logger.Infow("sugar log test1",
	// 	"url", "http://example.com",
	// 	"attempt", 3,
	// 	"backoff", time.Second,
	// )

	// Logger.Infof("sugar log test2: %s", "http://example.com")

	return nil
}

func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	// encoderConfig.FunctionKey = "func"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.ConsoleSeparator = "|"
	return zapcore.NewConsoleEncoder(encoderConfig)
}
