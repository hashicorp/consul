// Package plugin provides some types and functions common among plugin.
package plugin

import (
	"context"
	"errors"
	"fmt"

	"github.com/miekg/dns"
	ot "github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
)

type (
	// Plugin is a middle layer which represents the traditional
	// idea of plugin: it chains one Handler to the next by being
	// passed the next Handler in the chain.
	Plugin func(Handler) Handler

	// Handler is like dns.Handler except ServeDNS may return an rcode
	// and/or error.
	//
	// If ServeDNS writes to the response body, it should return a status
	// code. CoreDNS assumes *no* reply has yet been written if the status
	// code is one of the following:
	//
	// * SERVFAIL (dns.RcodeServerFailure)
	//
	// * REFUSED (dns.RecodeRefused)
	//
	// * FORMERR (dns.RcodeFormatError)
	//
	// * NOTIMP (dns.RcodeNotImplemented)
	//
	// All other response codes signal other handlers above it that the
	// response message is already written, and that they should not write
	// to it also.
	//
	// If ServeDNS encounters an error, it should return the error value
	// so it can be logged by designated error-handling plugin.
	//
	// If writing a response after calling another ServeDNS method, the
	// returned rcode SHOULD be used when writing the response.
	//
	// If handling errors after calling another ServeDNS method, the
	// returned error value SHOULD be logged or handled accordingly.
	//
	// Otherwise, return values should be propagated down the plugin
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

// Error returns err with 'plugin/name: ' prefixed to it.
func Error(name string, err error) error { return fmt.Errorf("%s/%s: %s", "plugin", name, err) }

// NextOrFailure calls next.ServeDNS when next is not nil, otherwise it will return, a ServerFailure and a nil error.
func NextOrFailure(name string, next Handler, ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) { // nolint: golint
	if next != nil {
		if span := ot.SpanFromContext(ctx); span != nil {
			child := span.Tracer().StartSpan(next.Name(), ot.ChildOf(span.Context()))
			defer child.Finish()
			ctx = ot.ContextWithSpan(ctx, child)
		}
		return next.ServeDNS(ctx, w, r)
	}

	return dns.RcodeServerFailure, Error(name, errors.New("no next plugin found"))
}

// ClientWrite returns true if the response has been written to the client.
// Each plugin to adhere to this protocol.
func ClientWrite(rcode int) bool {
	switch rcode {
	case dns.RcodeServerFailure:
		fallthrough
	case dns.RcodeRefused:
		fallthrough
	case dns.RcodeFormatError:
		fallthrough
	case dns.RcodeNotImplemented:
		return false
	}
	return true
}

// Namespace is the namespace used for the metrics.
const Namespace = "coredns"

// TimeBuckets is based on Prometheus client_golang prometheus.DefBuckets
var TimeBuckets = prometheus.ExponentialBuckets(0.00025, 2, 16) // from 0.25ms to 8 seconds

// ErrOnce is returned when a plugin doesn't support multiple setups per server.
var ErrOnce = errors.New("this plugin can only be used once per Server Block")
