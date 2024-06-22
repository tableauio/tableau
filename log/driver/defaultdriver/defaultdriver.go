package defaultdriver

import (
	"fmt"
	"path"
	"runtime"
	"time"

	"github.com/tableauio/tableau/log/core"
	"github.com/tableauio/tableau/log/driver"
)

type DefaultDriver struct {
	CallerSkip int
}

func init() {
	driver.RegisteDriver(&DefaultDriver{
		CallerSkip: 2,
	})
}

func (d *DefaultDriver) Name() string {
	return "default"
}

func (d *DefaultDriver) Print(r *core.Record) {
	if r.Level < d.GetLevel(d.Name()) {
		return
	}

	msg := getMessage(*r.Format, r.Args)
	callInfo := getCallerInfo(d.CallerSkip)
	text := fmt.Sprintf("%s\t%s\t%s\t%s:%d", ISO8601TimeEncoder(time.Now()), r.Level.CapitalString(), callInfo.FuncName, callInfo.File, callInfo.Line)
	// if len(context) != 0 {
	// 	text += fmt.Sprintf(" %+v", context)
	// }
	text += "\t\t" + msg
	fmt.Println(text)
}

func (d *DefaultDriver) GetLevel(logger string) core.Level {
	return core.DebugLevel
}

// getMessage format with Sprint, Sprintf, or neither.
func getMessage(format string, fmtArgs []any) string {
	if len(fmtArgs) == 0 {
		return format
	}

	if format != "" {
		return fmt.Sprintf(format, fmtArgs...)
	}

	if len(fmtArgs) == 1 {
		if str, ok := fmtArgs[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(fmtArgs...)
}

type CallerInfo struct {
	FullFuncName string
	FuncName     string
	File         string
	Line         int
}

func getCallerInfo(callerSkip int) *CallerInfo {
	pc, file, line, ok := runtime.Caller(4 + callerSkip) // backtrace two frames
	if !ok {
		return &CallerInfo{
			FullFuncName: "unknown",
			FuncName:     "unknown",
			File:         "unknown",
		}
	}
	fullFuncName := runtime.FuncForPC(pc).Name()
	return &CallerInfo{
		FullFuncName: fullFuncName,
		FuncName:     path.Base(fullFuncName),
		File:         file,
		Line:         line,
	}
}

// ISO8601TimeEncoder serializes a time.Time to an ISO8601-formatted string
// with millisecond precision.
func ISO8601TimeEncoder(t time.Time) string {
	return t.Format("2006-01-02T15:04:05.000Z0700")
}

// RFC3339TimeEncoder serializes a time.Time to an RFC3339-formatted string.
func RFC3339TimeEncoder(t time.Time) string {
	return t.Format(time.RFC3339)
}

// RFC3339NanoTimeEncoder serializes a time.Time to an RFC3339-formatted string
// with nanosecond precision.
func RFC3339NanoTimeEncoder(t time.Time) string {
	return t.Format(time.RFC3339Nano)
}
