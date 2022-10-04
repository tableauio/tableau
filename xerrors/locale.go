package xerrors

import (
	"github.com/tableauio/tableau/internal/localizer"
)

func renderSummary(module string, data interface{}) string {
	return localizer.Default.RenderSummary(module, data)
}

func renderEcode(ecode string, data interface{}) error {
	detail := localizer.Default.RenderEcode(ecode, data)
	return ErrorKV(detail.Text,
		keyErrCode, detail.Ecode,
		keyErrDesc, detail.Desc,
		keyHelp, detail.Help)
}
