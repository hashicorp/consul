// Package middleware provides some types and functions common among middleware.
package middleware

import (
	"fmt"

	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

type (
	// Middleware is the middle layer which represents the traditional
	// idea of middleware: it chains one Handler to the next by being
	// passed the next Handler in the chain.
	Middleware func(Handler) Handler

	// Handler is like dns.Handler except ServeDNS may return an rcode
	// and/or error.
	//
	// If ServeDNS writes to the response body, it should return a status
	// code. If the status code is not one of the following:
	//
	// * SERVFAIL (dns.RcodeServerFailure)
	//
	// * REFUSED (dns.RecodeRefused)
	//
	// * FORMERR (dns.RcodeFormatError)
	//
	// * NOTIMP (dns.RcodeNotImplemented)
	//
	// CoreDNS assumes *no* reply has yet been written. All other response
	// codes signal other handlers above it that the response message is
	// already written, and that they should not write to it also.
	//
	// If ServeDNS encounters an error, it should return the error value
	// so it can be logged by designated error-handling middleware.
	//
	// If writing a response after calling another ServeDNS method, the
	// returned rcode SHOULD be used when writing the response.
	//
	// If handling errors after calling another ServeDNS method, the
	// returned error value SHOULD be logged or handled accordingly.
	//
	// Otherwise, return values should be propagated down the middleware
	// chain by returning them unchanged.
	Handler interface {
		ServeDNS(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)
		Name() string
	}

	// HandlerFunc is a convenience type like dns.HandlerFunc, except
	// ServeDNS returns an rcode and an error. See Handler
	// documentation for more information.
	HandlerFunc func(context.Context, dns.ResponseWriter, *dns.Msg) (int, error)
)

// ServeDNS implements the Handler interface.
func (f HandlerFunc) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	return f(ctx, w, r)
}

// Name implements the Handler interface.
func (f HandlerFunc) Name() string { return "handlerfunc" }

// Error returns err with 'middleware/name: ' prefixed to it.
func Error(name string, err error) error { return fmt.Errorf("%s/%s: %s", "middleware", name, err) }

// Namespace is the namespace used for the metrics.
const Namespace = "coredns"
