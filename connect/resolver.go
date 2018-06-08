package connect

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

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

// StaticResolver is a statically defined resolver. This can be used to Dial a
// known Connect endpoint without performing service discovery.
type StaticResolver struct {
	// Addr is the network address (including port) of the instance. It must be
	// the connect-enabled mTLS listener and may be a proxy in front of the actual
	// target service process. It is a string in any valid form for passing
	// directly to net.Dial("tcp", addr).
	Addr string

	// CertURL is the identity we expect the server to present in it's TLS
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

	// trustDomain stores the cluster's trust domain it's populated once on first
	// Resolve call and blocks all resolutions.
	trustDomain   string
	trustDomainMu sync.Mutex
}

// Resolve performs service discovery against the local Consul agent and returns
// the address and expected identity of a suitable service instance.
func (cr *ConsulResolver) Resolve(ctx context.Context) (string, connect.CertURI, error) {
	// Fetch trust domain if we've not done that yet
	err := cr.ensureTrustDomain()
	if err != nil {
		return "", nil, err
	}

	switch cr.Type {
	case ConsulResolverTypeService:
		return cr.resolveService(ctx)
	case ConsulResolverTypePreparedQuery:
		return cr.resolveQuery(ctx)
	default:
		return "", nil, fmt.Errorf("unknown resolver type")
	}
}

func (cr *ConsulResolver) ensureTrustDomain() error {
	cr.trustDomainMu.Lock()
	defer cr.trustDomainMu.Unlock()

	if cr.trustDomain != "" {
		return nil
	}

	roots, _, err := cr.Client.Agent().ConnectCARoots(nil)
	if err != nil {
		return fmt.Errorf("failed fetching cluster trust domain: %s", err)
	}

	if roots.TrustDomain == "" {
		return fmt.Errorf("cluster trust domain empty, connect not bootstrapped")
	}

	cr.trustDomain = roots.TrustDomain
	return nil
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

	return cr.resolveServiceEntry(svcs[idx])
}

func (cr *ConsulResolver) resolveQuery(ctx context.Context) (string, connect.CertURI, error) {
	resp, _, err := cr.Client.PreparedQuery().Execute(cr.Name, cr.queryOptions(ctx))
	if err != nil {
		return "", nil, err
	}

	svcs := resp.Nodes
	if len(svcs) < 1 {
		return "", nil, fmt.Errorf("no healthy instances found")
	}

	// Services are not shuffled by HTTP API, pick one at (pseudo) random.
	idx := 0
	if len(svcs) > 1 {
		idx = rand.Intn(len(svcs))
	}

	return cr.resolveServiceEntry(&svcs[idx])
}

func (cr *ConsulResolver) resolveServiceEntry(entry *api.ServiceEntry) (string, connect.CertURI, error) {
	addr := entry.Service.Address
	if addr == "" {
		addr = entry.Node.Address
	}
	port := entry.Service.Port

	service := entry.Service.ProxyDestination
	if entry.Service.Connect != nil && entry.Service.Connect.Native {
		service = entry.Service.Service
	}
	if service == "" {
		// Shouldn't happen but to protect against bugs in agent API returning bad
		// service response...
		return "", nil, fmt.Errorf("not a valid connect service")
	}

	// Generate the expected CertURI
	certURI := &connect.SpiffeIDService{
		Host:       cr.trustDomain,
		Namespace:  "default",
		Datacenter: entry.Node.Datacenter,
		Service:    service,
	}

	return fmt.Sprintf("%s:%d", addr, port), certURI, nil
}

func (cr *ConsulResolver) queryOptions(ctx context.Context) *api.QueryOptions {
	q := &api.QueryOptions{
		// We may make this configurable one day but we may also implement our own
		// caching which is even more stale so...
		AllowStale: true,
		Datacenter: cr.Datacenter,

		// For prepared queries
		Connect: true,
	}
	return q.WithContext(ctx)
}
