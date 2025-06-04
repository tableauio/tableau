package metasheet

import (
	"context"
	"fmt"
	"strings"
)

type ctxKey struct{}

// NewContext creates a new context with the given metasheet context.
// A valid metasheet name must start with '@', otherwise it will panic.
func NewContext(ctx context.Context, v *Metasheet) context.Context {
	if v.Name == "" {
		// use default metasheet name if not specified
		v.Name = DefaultMetasheetName
	}
	if !strings.HasPrefix(v.Name, "@") {
		panic(fmt.Sprintf("metasheet name must start with '@': %q", v.Name))
	}
	return context.WithValue(ctx, ctxKey{}, &Metasheet{
		Name: v.Name,
	})
}

// FromContext returns the metasheet context from the given context. If not
// found, it will return the default context.
func FromContext(ctx context.Context) *Metasheet {
	if v, ok := ctx.Value(ctxKey{}).(*Metasheet); ok {
		return v
	}
	return &Metasheet{Name: DefaultMetasheetName}
}
