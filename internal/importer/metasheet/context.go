package metasheet

import (
	"context"
	"fmt"
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
	if metasheetName == "" {
		metasheetName = DefaultMetasheetName
	}
	if !strings.HasPrefix(metasheetName, "@") {
		panic(fmt.Sprintf("metasheet name must start with '@': %q", metasheetName))
	}
	return context.WithValue(ctx, ctxKey{}, &Context{
		metasheetName: metasheetName,
	})
}

func FromContext(ctx context.Context) *Context {
	if s, ok := ctx.Value(ctxKey{}).(*Context); ok {
		return s
	}
	return &Context{metasheetName: DefaultMetasheetName}
}

func (ctx *Context) MetasheetName() string {
	return ctx.metasheetName
}
