package connect

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
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

// ConsulResolverFromAddrFunc returns a function for constructing ConsulResolver
// from a consul DNS formatted hostname (e.g. foo.service.consul or
// foo.query.consul).
//
// Note, the returned ConsulResolver resolves the query via regular agent HTTP
// discovery API. DNS is not needed or used for discovery, only the hostname
// format re-used for consistency.
func ConsulResolverFromAddrFunc(client *api.Client) func(addr string) (Resolver, error) {
	// Capture client dependency
	return func(addr string) (Resolver, error) {
		// Http clients might provide hostname and port
		host := strings.ToLower(stripPort(addr))

		// For now we force use of `.consul` TLD regardless of the configured domain
		// on the cluster. That's because we don't know that domain here and it
		// would be really complicated to discover it inline here. We do however
		// need to be able to distingush a hostname with the optional datacenter
		// segment which we can't do unambiguously if we allow arbitrary trailing
		// domains.
		domain := ".consul"
		if !strings.HasSuffix(host, domain) {
			return nil, fmt.Errorf("invalid Consul DNS domain: note Connect SDK " +
				"currently requires use of .consul domain even if cluster is " +
				"configured with a different domain.")
		}

		// Remove the domain suffix
		host = host[0 : len(host)-len(domain)]

		parts := strings.Split(host, ".")
		numParts := len(parts)

		r := &ConsulResolver{
			Client:    client,
			Namespace: "default",
		}

		// Note that 3 segments may be a valid DNS name like
		// <tag>.<service>.service.consul but not one we support, it might also be
		// <service>.service.<datacenter>.consul which we do want to support so we
		// have to figure out if the last segment is supported keyword and if not
		// check if the supported keyword is further up...

		// To simplify logic for now, we must match one of the following (not domain
		// is stripped):
		//  <name>.[service|query]
		//  <name>.[service|query].<dc>
		if numParts < 2 || numParts > 3 || !supportedTypeLabel(parts[1]) {
			return nil, fmt.Errorf("unsupported Consul DNS domain: must be either " +
				"<name>.service[.<datacenter>].consul or " +
				"<name>.query[.<datacenter>].consul")
		}

		if numParts == 3 {
			// Must be datacenter case
			r.Datacenter = parts[2]
		}

		// By know we must have a supported query type which means at least 2
		// elements with first 2 being name, and type respectively.
		r.Name = parts[0]
		switch parts[1] {
		case "service":
			r.Type = ConsulResolverTypeService
		case "query":
			r.Type = ConsulResolverTypePreparedQuery
		default:
			// This should never happen (tm) unless the supportedTypeLabel
			// implementation is changed and this switch isn't.
			return nil, fmt.Errorf("invalid discovery type")
		}

		return r, nil
	}
}

func supportedTypeLabel(label string) bool {
	return label == "service" || label == "query"
}

// stripPort copied from net/url/url.go
func stripPort(hostport string) string {
	colon := strings.IndexByte(hostport, ':')
	if colon == -1 {
		return hostport
	}
	if i := strings.IndexByte(hostport, ']'); i != -1 {
		return strings.TrimPrefix(hostport[:i], "[")
	}
	return hostport[:colon]
}
