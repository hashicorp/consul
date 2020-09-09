package consul

import (
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/router"
	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc/resolver"
)

var registerLock sync.Mutex

// registerResolverBuilder registers our custom grpc resolver with the given scheme.
func registerResolverBuilder(scheme, datacenter string, shutdownCh <-chan struct{}) *ServerResolverBuilder {
	registerLock.Lock()
	defer registerLock.Unlock()
	grpcResolverBuilder := NewServerResolverBuilder(scheme, datacenter, shutdownCh)
	resolver.Register(grpcResolverBuilder)
	return grpcResolverBuilder
}

// ServerResolverBuilder tracks the current server list and keeps any
// ServerResolvers updated when changes occur.
type ServerResolverBuilder struct {
	// Allow overriding the scheme to support parallel tests, since
	// the resolver builder is registered globally.
	scheme     string
	datacenter string
	servers    map[string]*metadata.Server
	resolvers  map[resolver.ClientConn]*ServerResolver
	shutdownCh <-chan struct{}
	lock       sync.Mutex
}

func NewServerResolverBuilder(scheme, datacenter string, shutdownCh <-chan struct{}) *ServerResolverBuilder {
	return &ServerResolverBuilder{
		scheme:     scheme,
		datacenter: datacenter,
		servers:    make(map[string]*metadata.Server),
		resolvers:  make(map[resolver.ClientConn]*ServerResolver),
	}
}

// periodicServerRebalance periodically reshuffles the order of server addresses
// within the resolvers to ensure the load is balanced across servers.
func (s *ServerResolverBuilder) periodicServerRebalance(serf *serf.Serf) {
	// Compute the rebalance timer based on the number of local servers and nodes.
	rebalanceDuration := router.ComputeRebalanceTimer(s.serversInDC(s.datacenter), serf.NumNodes())
	timer := time.NewTimer(rebalanceDuration)

	for {
		select {
		case <-timer.C:
			s.rebalanceResolvers()

			// Re-compute the wait duration.
			newTimerDuration := router.ComputeRebalanceTimer(s.serversInDC(s.datacenter), serf.NumNodes())
			timer.Reset(newTimerDuration)
		case <-s.shutdownCh:
			timer.Stop()
			return
		}
	}
}

// rebalanceResolvers shuffles the server list for resolvers in all datacenters.
func (s *ServerResolverBuilder) rebalanceResolvers() {
	s.lock.Lock()
	defer s.lock.Unlock()

	for _, resolver := range s.resolvers {
		// Shuffle the list of addresses using the last list given to the resolver.
		resolver.addrLock.Lock()
		addrs := resolver.lastAddrs
		rand.Shuffle(len(addrs), func(i, j int) {
			addrs[i], addrs[j] = addrs[j], addrs[i]
		})
		// Pass the shuffled list to the resolver.
		resolver.updateAddrsLocked(addrs)
		resolver.addrLock.Unlock()
	}
}

// serversInDC returns the number of servers in the given datacenter.
func (s *ServerResolverBuilder) serversInDC(dc string) int {
	s.lock.Lock()
	defer s.lock.Unlock()

	var serverCount int
	for _, server := range s.servers {
		if server.Datacenter == dc {
			serverCount++
		}
	}

	return serverCount
}

// Servers returns metadata for all currently known servers. This is used
// by grpc.ClientConn through our custom dialer.
func (s *ServerResolverBuilder) Servers() []*metadata.Server {
	s.lock.Lock()
	defer s.lock.Unlock()

	servers := make([]*metadata.Server, 0, len(s.servers))
	for _, server := range s.servers {
		servers = append(servers, server)
	}
	return servers
}

// Build returns a new ServerResolver for the given ClientConn. The resolver
// will keep the ClientConn's state updated based on updates from Serf.
func (s *ServerResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// If there's already a resolver for this datacenter, return it.
	datacenter := strings.TrimPrefix(target.Endpoint, "server.")
	if resolver, ok := s.resolvers[cc]; ok {
		return resolver, nil
	}

	// Make a new resolver for the dc and add it to the list of active ones.
	resolver := &ServerResolver{
		datacenter: datacenter,
		clientConn: cc,
	}
	resolver.updateAddrs(s.getDCAddrs(datacenter))
	resolver.closeCallback = func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		delete(s.resolvers, cc)
	}

	s.resolvers[cc] = resolver

	return resolver, nil
}

func (s *ServerResolverBuilder) Scheme() string { return s.scheme }

// AddServer updates the resolvers' states to include the new server's address.
func (s *ServerResolverBuilder) AddServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.servers[server.ID] = server

	addrs := s.getDCAddrs(server.Datacenter)
	for _, resolver := range s.resolvers {
		if resolver.datacenter == server.Datacenter {
			resolver.updateAddrs(addrs)
		}
	}
}

// RemoveServer updates the resolvers' states with the given server removed.
func (s *ServerResolverBuilder) RemoveServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.servers, server.ID)

	addrs := s.getDCAddrs(server.Datacenter)
	for _, resolver := range s.resolvers {
		if resolver.datacenter == server.Datacenter {
			resolver.updateAddrs(addrs)
		}
	}
}

// getDCAddrs returns a list of the server addresses for the given datacenter.
// This method assumes the lock is held.
func (s *ServerResolverBuilder) getDCAddrs(dc string) []resolver.Address {
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
	datacenter    string
	clientConn    resolver.ClientConn
	closeCallback func()

	lastAddrs []resolver.Address
	addrLock  sync.Mutex
}

// updateAddrs updates this ServerResolver's ClientConn to use the given set of
// addrs.
func (r *ServerResolver) updateAddrs(addrs []resolver.Address) {
	r.addrLock.Lock()
	defer r.addrLock.Unlock()
	r.updateAddrsLocked(addrs)
}

// updateAddrsLocked updates this ServerResolver's ClientConn to use the given
// set of addrs. addrLock must be held by calleer.
func (r *ServerResolver) updateAddrsLocked(addrs []resolver.Address) {
	// Only pass the first address initially, which will cause the
	// balancer to spin down the connection for its previous first address
	// if it is different. If we don't do this, it will keep using the old
	// first address as long as it is still in the list, making it impossible to
	// rebalance until that address is removed.
	var firstAddr []resolver.Address
	if len(addrs) > 0 {
		firstAddr = []resolver.Address{addrs[0]}
	}
	r.clientConn.UpdateState(resolver.State{Addresses: firstAddr})

	// Call UpdateState again with the entire list of addrs in case we need them
	// for failover.
	r.clientConn.UpdateState(resolver.State{Addresses: addrs})

	r.lastAddrs = addrs
}

func (s *ServerResolver) Close() {
	s.closeCallback()
}

// Unneeded since we only update the ClientConn when our server list changes.
func (*ServerResolver) ResolveNow(o resolver.ResolveNowOption) {}
