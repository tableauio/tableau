package xerrors

import "github.com/tableauio/tableau/internal/localizer"

// WrapEcodeWithCallerSkip adds code defaults through the existing metadata
// wrapper without replacing individual messages. Forwarding APIs pass one to
// capture their caller; existing codes, metadata, causes, and stacks survive.
func WrapEcodeWithCallerSkip(skip int, err error, code *ecode) error {
	if err == nil {
		return nil
	}
	detail := localizer.Default.RenderEcode(code.code, nil)
	return WrapKVWithCallerSkip(skip+1, err,
		KeyErrCode, code.code,
		KeyErrDesc, detail.Desc,
		KeyModule, ModuleDefault)
}
