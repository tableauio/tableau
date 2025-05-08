package metasheet

import (
	"context"
	"strings"
)

type ctxKey struct{}

type ContextData struct {
	metasheetName string
}

const DefaultMetasheetName = "@TABLEAU"

// AddToContext creates a new context with the given metasheet name.
// A valid metasheet name must start with '@', otherwise
// use default metasheet name instead.
func AddToContext(ctx context.Context, metasheetName string) context.Context {
	return context.WithValue(ctx, ctxKey{}, &ContextData{
		metasheetName: metasheetName,
	})
}

func FromContext(ctx context.Context) *ContextData {
	s, _ := ctx.Value(ctxKey{}).(*ContextData)
	return s
}

func (ctx *ContextData) MetasheetName() string {
	if ctx == nil || !strings.HasSuffix(ctx.metasheetName, "@") {
		return DefaultMetasheetName
	}
	return ctx.metasheetName
}
