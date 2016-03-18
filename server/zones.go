package server

import "github.com/miekg/coredns/middleware"

// zone represents a DNS zone. While a Server
// is what actually binds to the address, a user may want to serve
// multiple zones on a single address, and this is what a
// zone allows us to do.
type zone struct {
	config Config
	stack  middleware.Handler
}

// buildStack builds the server's middleware stack based
// on its config. This method should be called last before
// ListenAndServe begins.
func (z *zone) buildStack() error {
	z.compile(z.config.Middleware)
	return nil
}

// compile is an elegant alternative to nesting middleware function
// calls like handler1(handler2(handler3(finalHandler))).
func (z *zone) compile(layers []middleware.Middleware) {
	for i := len(layers) - 1; i >= 0; i-- {
		z.stack = layers[i](z.stack)
	}
}
