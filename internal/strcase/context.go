package strcase

import (
	"context"
)

type ctxKey struct{}

// NewContext creates a new context with the given Strcase.
func NewContext(ctx context.Context, v *Strcase) context.Context {
	return context.WithValue(ctx, ctxKey{}, v)
}

// FromContext returns the Strcase from the given context. If not found, it will
// return the default Strcase.
func FromContext(ctx context.Context) *Strcase {
	if v, ok := ctx.Value(ctxKey{}).(*Strcase); ok {
		return v
	}
	return &Strcase{}
}
