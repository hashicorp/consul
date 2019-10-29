package consul

import (
	"strings"
	"sync"

	"github.com/hashicorp/consul/agent/metadata"
	"google.golang.org/grpc/resolver"
)

// registerResolverBuilder registers our custom grpc resolver with the given scheme.
func registerResolverBuilder(scheme string) *ServerResolverBuilder {
	grpcResolverBuilder := NewServerResolverBuilder(scheme)
	resolver.Register(grpcResolverBuilder)
	return grpcResolverBuilder
}

// ServerResolverBuilder tracks the current server list and keeps any
// ServerResolvers updated when changes occur.
type ServerResolverBuilder struct {
	// Allow overriding the scheme to support parallel tests, since
	// the resolver builder is registered globally.
	scheme    string
	servers   map[string]*metadata.Server
	resolvers map[string]*ServerResolver
	lock      sync.Mutex
}

func NewServerResolverBuilder(scheme string) *ServerResolverBuilder {
	return &ServerResolverBuilder{
		scheme:    scheme,
		servers:   make(map[string]*metadata.Server),
		resolvers: make(map[string]*ServerResolver),
	}
}

// Build returns a new ServerResolver for the given ClientConn. The resolver
// will keep the ClientConn's state updated based on updates from Serf.
func (s *ServerResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// If there's already a resolver for this datacenter, return it.
	datacenter := strings.TrimPrefix(target.Endpoint, "server.")
	if resolver, ok := s.resolvers[datacenter]; ok {
		return resolver, nil
	}

	// Make a new resolver for the dc and add it to the list of active ones.
	resolver := &ServerResolver{
		clientConn: cc,
	}
	resolver.updateAddrs(s.getAddrs(datacenter))
	resolver.closeCallback = func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		delete(s.resolvers, datacenter)
	}

	s.resolvers[datacenter] = resolver

	return resolver, nil
}

func (s *ServerResolverBuilder) Scheme() string { return s.scheme }

// AddServer updates the resolvers' states to include the new server's address.
func (s *ServerResolverBuilder) AddServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.servers[server.ID] = server
	if resolver, ok := s.resolvers[server.Datacenter]; ok {
		addrs := s.getAddrs(server.Datacenter)
		resolver.updateAddrs(addrs)
	}
}

// RemoveServer updates the resolvers' states with the given server removed.
func (s *ServerResolverBuilder) RemoveServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.servers, server.ID)
	if resolver, ok := s.resolvers[server.Datacenter]; ok {
		addrs := s.getAddrs(server.Datacenter)
		resolver.updateAddrs(addrs)
	}
}

// getAddrs returns a list of the current servers' addresses. This method assumes
// the lock is held.
func (s *ServerResolverBuilder) getAddrs(dc string) []resolver.Address {
	var addrs []resolver.Address
	for _, server := range s.servers {
		if server.Datacenter != dc {
			continue
		}

		addrs = append(addrs, resolver.Address{
			Addr:       server.Addr.String(),
			Type:       resolver.Backend,
			ServerName: server.Name,
		})
	}
	return addrs
}

// ServerResolver is a grpc Resolver that will keep a grpc.ClientConn up to date
// on the list of server addresses to use.
type ServerResolver struct {
	clientConn    resolver.ClientConn
	closeCallback func()
}

// updateAddrs updates this ServerResolver's ClientConn to use the given set of addrs.
func (r *ServerResolver) updateAddrs(addrs []resolver.Address) {
	r.clientConn.NewAddress(addrs)
}

func (s *ServerResolver) Close() {
	s.closeCallback()
}

// Unneeded since we only update the ClientConn when our server list changes.
func (*ServerResolver) ResolveNow(o resolver.ResolveNowOption) {}
