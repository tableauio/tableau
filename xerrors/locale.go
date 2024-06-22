package xerrors

import (
	"github.com/tableauio/tableau/internal/localizer"
)

func renderSummary(module string, data map[string]any) string {
	return localizer.Default.RenderMessage(module, data)
}

func renderEcode(ecode string, data any) error {
	detail := localizer.Default.RenderEcode(ecode, data)
	return ErrorKV(detail.Text,
		keyErrCode, detail.Ecode,
		keyErrDesc, detail.Desc,
		keyHelp, detail.Help)
}
