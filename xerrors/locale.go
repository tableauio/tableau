package xerrors

import (
	"github.com/tableauio/tableau/internal/localizer"
)

func renderSummary(module string, data map[string]interface{}) string {
	return localizer.Default.RenderKV(module, data)
}

func renderEcode(ecode string, data interface{}) error {
	detail := localizer.Default.RenderEcode(ecode, data)
	return ErrorKV(detail.Text,
		keyErrCode, detail.Ecode,
		keyErrDesc, detail.Desc,
		keyHelp, detail.Help)
}
