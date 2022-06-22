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
	if opt.Filename == "" {
		return atom.InitConsoleLog(opt.Mode, opt.Level)
	} else {
		return atom.InitMultiLog(opt.Mode, opt.Level, opt.Filename)
	}
}
