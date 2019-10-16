package consul

import (
	"sync"

	"github.com/hashicorp/consul/agent/metadata"
	"google.golang.org/grpc/resolver"
)

const grpcResolverScheme = "consul"

var resolverBuilder *ServerResolverBuilder

func init() {
	resolverBuilder = NewServerResolverBuilder()
	resolver.Register(resolverBuilder)
}

// ServerResolverBuilder tracks the current server list and keeps any
// ServerResolvers updated when changes occur.
type ServerResolverBuilder struct {
	servers   map[string]*metadata.Server
	resolvers map[*ServerResolver]struct{}
	lock      sync.Mutex
}

func NewServerResolverBuilder() *ServerResolverBuilder {
	return &ServerResolverBuilder{
		servers:   make(map[string]*metadata.Server),
		resolvers: make(map[*ServerResolver]struct{}),
	}
}

func (s *ServerResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Make a new resolver and add it to the list of active ones.
	resolver := &ServerResolver{
		cc: cc,
	}
	resolver.updateAddrs(s.getAddrs())
	resolver.closeCallback = func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		delete(s.resolvers, resolver)
	}

	s.resolvers[resolver] = struct{}{}

	return resolver, nil
}

func (s *ServerResolverBuilder) Scheme() string { return grpcResolverScheme }

func (s *ServerResolverBuilder) AddServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.servers[server.ID] = server
	addrs := s.getAddrs()
	for resolver, _ := range s.resolvers {
		resolver.updateAddrs(addrs)
	}
}

func (s *ServerResolverBuilder) RemoveServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.servers, server.ID)
	addrs := s.getAddrs()
	for resolver, _ := range s.resolvers {
		resolver.updateAddrs(addrs)
	}
}

// getAddrs returns a list of the current servers' addresses. This method assumes
// the lock is held.
func (s *ServerResolverBuilder) getAddrs() []resolver.Address {
	var addrs []resolver.Address
	for _, server := range s.servers {
		addrs = append(addrs, resolver.Address{
			Addr:       server.Addr.String(),
			Type:       resolver.Backend,
			ServerName: server.Name,
		})
	}
	return addrs
}

type ServerResolver struct {
	cc            resolver.ClientConn
	closeCallback func()
}

func (r *ServerResolver) updateAddrs(addrs []resolver.Address) {
	r.cc.NewAddress(addrs)
}

func (s *ServerResolver) Close() {
	s.closeCallback()
}

func (*ServerResolver) ResolveNow(o resolver.ResolveNowOption) {}
