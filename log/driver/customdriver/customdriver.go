// Package customdriver adapts a user-provided [Logger] to the
// driver.Driver interface, so that tableau's log output can be routed into
// an external logging system.
package customdriver

import (
	"fmt"
	"strings"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
)

var _ driver.Driver = (*CustomDriver)(nil)

// Logger is a minimal Printf-style logging interface, satisfied directly by
// most loggers (e.g. *zap.SugaredLogger) or via a thin adapter (e.g. slog).
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	DPanicf(format string, args ...any)
	Panicf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// CustomDriver forwards core.Record entries to a user-provided Logger.
type CustomDriver struct {
	logger Logger
}

// New creates a CustomDriver that forwards log records to logger.
func New(logger Logger) *CustomDriver {
	return &CustomDriver{logger: logger}
}

func (d *CustomDriver) Name() string {
	return "custom"
}

// GetLevel always returns core.DebugLevel; filtering is delegated to logger.
func (d *CustomDriver) GetLevel(logger string) core.Level {
	return core.DebugLevel
}

// Print normalizes r (from a Xxx/Xxxf/Xxxw call) into one message and
// forwards it via the corresponding Xxxf method.
func (d *CustomDriver) Print(r *core.Record) {
	msg := formatRecord(r)
	switch r.Level {
	case core.DebugLevel:
		d.logger.Debugf("%s", msg)
	case core.InfoLevel:
		d.logger.Infof("%s", msg)
	case core.WarnLevel:
		d.logger.Warnf("%s", msg)
	case core.ErrorLevel:
		d.logger.Errorf("%s", msg)
	case core.DPanicLevel:
		d.logger.DPanicf("%s", msg)
	case core.PanicLevel:
		d.logger.Panicf("%s", msg)
	case core.FatalLevel:
		d.logger.Fatalf("%s", msg)
	}
}

// formatRecord renders r's format/args and any key-value pairs into one message.
func formatRecord(r *core.Record) string {
	var msg string
	if r.Format != nil && *r.Format != "" {
		msg = fmt.Sprintf(*r.Format, r.Args...)
	} else if len(r.Args) > 0 {
		msg = fmt.Sprint(r.Args...)
	}
	if len(r.KVs) > 0 {
		if msg != "" {
			msg += " "
		}
		msg += formatKVs(r.KVs)
	}
	return msg
}

// formatKVs renders kvs as "key1=value1 key2=value2".
func formatKVs(kvs []any) string {
	var b strings.Builder
	for i := 0; i < len(kvs); i += 2 {
		if i > 0 {
			b.WriteByte(' ')
		}
		if i+1 < len(kvs) {
			fmt.Fprintf(&b, "%v=%v", kvs[i], kvs[i+1])
		} else {
			fmt.Fprintf(&b, "%v=(MISSING)", kvs[i])
		}
	}
	return b.String()
}
