package log

import (
	"github.com/tableauio/tableau/internal/atom"
	"github.com/tableauio/tableau/options"
	"go.uber.org/zap"
)

func Log() *zap.SugaredLogger {
	return atom.Log
}

// Init set the log options for debugging.
func Init(opt *options.LogOption) error {
	sinkType, err := atom.GetSinkType(opt.Sink)
	if err != nil {
		return err
	}
	switch sinkType {
	case atom.SinkFile:
		return atom.InitFileLog(opt.Mode, opt.Level, opt.Filename)
	case atom.SinkMulti:
		return atom.InitMultiLog(opt.Mode, opt.Level, opt.Filename)
	default:
		return atom.InitConsoleLog(opt.Mode, opt.Level)
	}
}
