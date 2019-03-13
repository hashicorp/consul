package dnstap

import "context"

type contextKey struct{}

var dnstapKey = contextKey{}

// ContextWithTapper returns a new `context.Context` that holds a reference to
// `t`'s Tapper.
func ContextWithTapper(ctx context.Context, t Tapper) context.Context {
	return context.WithValue(ctx, dnstapKey, t)
}

// TapperFromContext returns the `Tapper` previously associated with `ctx`, or
// `nil` if no such `Tapper` could be found.
func TapperFromContext(ctx context.Context) Tapper {
	val := ctx.Value(dnstapKey)
	if sp, ok := val.(Tapper); ok {
		return sp
	}
	return nil
}
