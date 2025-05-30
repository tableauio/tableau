package metasheet

import (
	"context"
	"strings"
)

type ctxKey struct{}

type Context struct {
	metasheetName string
}

const DefaultMetasheetName = "@TABLEAU"

// NewContext creates a new context with the given metasheet name.
// A valid metasheet name must start with '@', otherwise
// use default metasheet name instead.
func NewContext(ctx context.Context, metasheetName string) context.Context {
	return context.WithValue(ctx, ctxKey{}, &Context{
		metasheetName: metasheetName,
	})
}

func FromContext(ctx context.Context) *Context {
	s, _ := ctx.Value(ctxKey{}).(*Context)
	return s
}

func (ctx *Context) MetasheetName() string {
	if ctx == nil || !strings.HasSuffix(ctx.metasheetName, "@") {
		return DefaultMetasheetName
	}
	return ctx.metasheetName
}
