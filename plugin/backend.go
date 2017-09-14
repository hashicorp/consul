package plugin

import (
	"github.com/coredns/coredns/plugin/etcd/msg"
	"github.com/coredns/coredns/request"

	"github.com/miekg/dns"
)

// ServiceBackend defines a (dynamic) backend that returns a slice of service definitions.
type ServiceBackend interface {
	// Services communicates with the backend to retrieve the service definition. Exact indicates
	// on exact much are that we are allowed to recurs.
	Services(state request.Request, exact bool, opt Options) ([]msg.Service, error)

	// Reverse communicates with the backend to retrieve service definition based on a IP address
	// instead of a name. I.e. a reverse DNS lookup.
	Reverse(state request.Request, exact bool, opt Options) ([]msg.Service, error)

	// Lookup is used to find records else where.
	Lookup(state request.Request, name string, typ uint16) (*dns.Msg, error)

	// Returns _all_ services that matches a certain name.
	// Note: it does not implement a specific service.
	Records(state request.Request, exact bool) ([]msg.Service, error)

	// IsNameError return true if err indicated a record not found condition
	IsNameError(err error) bool
}

// Options are extra options that can be specified for a lookup.
type Options struct{}
