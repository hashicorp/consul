package connect

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/api"
)

// Resolver is the interface implemented by a service discovery mechanism to get
// the address and identity of an instance to connect to via Connect as a
// client.
type Resolver interface {
	// Resolve returns a single service instance to connect to. Implementations
	// may attempt to ensure the instance returned is currently available. It is
	// expected that a client will re-dial on a connection failure so making an
	// effort to return a different service instance each time where available
	// increases reliability. The context passed can be used to impose timeouts
	// which may or may not be respected by implementations that make network
	// calls to resolve the service. The addr returned is a string in any valid
	// form for passing directly to `net.Dial("tcp", addr)`. The certURI
	// represents the identity of the service instance. It will be matched against
	// the TLS certificate URI SAN presented by the server and the connection
	// rejected if they don't match.
	Resolve(ctx context.Context) (addr string, certURI connect.CertURI, err error)
}

// StaticResolver is a statically defined resolver. This can be used to connect
// to an known-Connect endpoint without performing service discovery.
type StaticResolver struct {
	// Addr is the network address (including port) of the instance. It must be
	// the connect-enabled mTLS server and may be a proxy in front of the actual
	// target service process. It is a string in any valid form for passing
	// directly to `net.Dial("tcp", addr)`.
	Addr string

	// CertURL is the _identity_ we expect the server to present in it's TLS
	// certificate. It must be an exact URI string match or the connection will be
	// rejected.
	CertURI connect.CertURI
}

// Resolve implements Resolver by returning the static values.
func (sr *StaticResolver) Resolve(ctx context.Context) (string, connect.CertURI, error) {
	return sr.Addr, sr.CertURI, nil
}

const (
	// ConsulResolverTypeService indicates resolving healthy service nodes.
	ConsulResolverTypeService int = iota

	// ConsulResolverTypePreparedQuery indicates resolving via prepared query.
	ConsulResolverTypePreparedQuery
)

// ConsulResolver queries Consul for a service instance.
type ConsulResolver struct {
	// Client is the Consul API client to use. Must be non-nil or Resolve will
	// panic.
	Client *api.Client

	// Namespace of the query target.
	Namespace string

	// Name of the query target.
	Name string

	// Type of the query target. Should be one of the defined ConsulResolverType*
	// constants. Currently defaults to ConsulResolverTypeService.
	Type int

	// Datacenter to resolve in, empty indicates agent's local DC.
	Datacenter string
}

// Resolve performs service discovery against the local Consul agent and returns
// the address and expected identity of a suitable service instance.
func (cr *ConsulResolver) Resolve(ctx context.Context) (string, connect.CertURI, error) {
	switch cr.Type {
	case ConsulResolverTypeService:
		return cr.resolveService(ctx)
	case ConsulResolverTypePreparedQuery:
		// TODO(banks): we need to figure out what API changes are needed for
		// prepared queries to become connect-aware. How do we signal that we want
		// connect-enabled endpoints vs the direct ones for the responses?
		return "", nil, fmt.Errorf("not implemented")
	default:
		return "", nil, fmt.Errorf("unknown resolver type")
	}
}

func (cr *ConsulResolver) resolveService(ctx context.Context) (string, connect.CertURI, error) {
	health := cr.Client.Health()

	svcs, _, err := health.Connect(cr.Name, "", true, cr.queryOptions(ctx))
	if err != nil {
		return "", nil, err
	}

	if len(svcs) < 1 {
		return "", nil, fmt.Errorf("no healthy instances found")
	}

	// Services are not shuffled by HTTP API, pick one at (pseudo) random.
	idx := 0
	if len(svcs) > 1 {
		idx = rand.Intn(len(svcs))
	}

	addr := svcs[idx].Service.Address
	if addr == "" {
		addr = svcs[idx].Node.Address
	}
	port := svcs[idx].Service.Port

	// Generate the expected CertURI

	// TODO(banks): when we've figured out the CA story around generating and
	// propagating these trust domains we need to actually fetch the trust domain
	// somehow. We also need to implement namespaces. Use of test function here is
	// temporary pending the work on trust domains.
	certURI := &connect.SpiffeIDService{
		Host:       "11111111-2222-3333-4444-555555555555.consul",
		Namespace:  "default",
		Datacenter: svcs[idx].Node.Datacenter,
		Service:    svcs[idx].Service.ProxyDestination,
	}

	return fmt.Sprintf("%s:%d", addr, port), certURI, nil
}

func (cr *ConsulResolver) queryOptions(ctx context.Context) *api.QueryOptions {
	q := &api.QueryOptions{
		// We may make this configurable one day but we may also implement our own
		// caching which is even more stale so...
		AllowStale: true,
		Datacenter: cr.Datacenter,
	}
	return q.WithContext(ctx)
}
