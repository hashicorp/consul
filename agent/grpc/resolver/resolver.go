package resolver

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/resolver"

	"github.com/hashicorp/consul/agent/metadata"
)

// ServerResolverBuilder tracks the current server list and keeps any
// ServerResolvers updated when changes occur.
type ServerResolverBuilder struct {
	cfg Config
	// leaderResolver is used to track the address of the leader in the local DC.
	leaderResolver leaderResolver
	// servers is an index of Servers by Server.ID. The map contains server IDs
	// for all datacenters.
	servers map[string]*metadata.Server
	// resolvers is an index of connections to the serverResolver which manages
	// addresses of servers for that connection.
	resolvers map[resolver.ClientConn]*serverResolver
	// lock for all stateful fields (excludes config which is immutable).
	lock sync.RWMutex
}

type Config struct {
	// Authority used to query the server. Defaults to "". Used to support
	// parallel testing because gRPC registers resolvers globally.
	Authority string
}

func NewServerResolverBuilder(cfg Config) *ServerResolverBuilder {
	return &ServerResolverBuilder{
		cfg:       cfg,
		servers:   make(map[string]*metadata.Server),
		resolvers: make(map[resolver.ClientConn]*serverResolver),
	}
}

// NewRebalancer returns a function which shuffles the server list for resolvers
// in all datacenters.
func (s *ServerResolverBuilder) NewRebalancer(dc string) func() {
	shuffler := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func() {
		s.lock.RLock()
		defer s.lock.RUnlock()

		for _, resolver := range s.resolvers {
			if resolver.datacenter != dc {
				continue
			}
			// Shuffle the list of addresses using the last list given to the resolver.
			resolver.addrLock.Lock()
			addrs := resolver.addrs
			shuffler.Shuffle(len(addrs), func(i, j int) {
				addrs[i], addrs[j] = addrs[j], addrs[i]
			})
			// Pass the shuffled list to the resolver.
			resolver.updateAddrsLocked(addrs)
			resolver.addrLock.Unlock()
		}
	}
}

// ServerForAddr returns server metadata for a server with the specified address.
func (s *ServerResolverBuilder) ServerForAddr(addr string) (*metadata.Server, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, server := range s.servers {
		if server.Addr.String() == addr {
			return server, nil
		}
	}
	return nil, fmt.Errorf("failed to find Consul server for address %q", addr)
}

// Build returns a new serverResolver for the given ClientConn. The resolver
// will keep the ClientConn's state updated based on updates from Serf.
func (s *ServerResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOption) (resolver.Resolver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// If there's already a resolver for this connection, return it.
	// TODO(streaming): how would this happen since we already cache connections in ClientConnPool?
	if resolver, ok := s.resolvers[cc]; ok {
		return resolver, nil
	}
	if cc == s.leaderResolver.clientConn {
		return s.leaderResolver, nil
	}

	serverType, datacenter, err := parseEndpoint(target.Endpoint)
	if err != nil {
		return nil, err
	}
	if serverType == "leader" {
		// TODO: is this safe? can we ever have multiple CC for the leader? Seems
		// like we can only have one given the caching in ClientConnPool.Dial
		s.leaderResolver.clientConn = cc
		s.leaderResolver.updateClientConn()
		return s.leaderResolver, nil
	}

	// Make a new resolver for the dc and add it to the list of active ones.
	resolver := &serverResolver{
		datacenter: datacenter,
		clientConn: cc,
		close: func() {
			s.lock.Lock()
			defer s.lock.Unlock()
			delete(s.resolvers, cc)
		},
	}
	resolver.updateAddrs(s.getDCAddrs(datacenter))

	s.resolvers[cc] = resolver
	return resolver, nil
}

// parseEndpoint parses a string, expecting a format of "serverType.datacenter"
func parseEndpoint(target string) (string, string, error) {
	parts := strings.SplitN(target, ".", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("unexpected endpoint address: %v", target)
	}

	return parts[0], parts[1], nil
}

func (s *ServerResolverBuilder) Authority() string {
	return s.cfg.Authority
}

// AddServer updates the resolvers' states to include the new server's address.
func (s *ServerResolverBuilder) AddServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.servers[uniqueID(server)] = server

	addrs := s.getDCAddrs(server.Datacenter)
	for _, resolver := range s.resolvers {
		if resolver.datacenter == server.Datacenter {
			resolver.updateAddrs(addrs)
		}
	}
}

// uniqueID returns a unique identifier for the server which includes the
// Datacenter and the ID.
//
// In practice it is expected that the server.ID is already a globally unique
// UUID. This function is an extra safeguard in case that ever changes.
func uniqueID(server *metadata.Server) string {
	return server.Datacenter + "-" + server.ID
}

// RemoveServer updates the resolvers' states with the given server removed.
func (s *ServerResolverBuilder) RemoveServer(server *metadata.Server) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.servers, uniqueID(server))

	addrs := s.getDCAddrs(server.Datacenter)
	for _, resolver := range s.resolvers {
		if resolver.datacenter == server.Datacenter {
			resolver.updateAddrs(addrs)
		}
	}
}

// getDCAddrs returns a list of the server addresses for the given datacenter.
// This method requires that lock is held for reads.
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

// UpdateLeaderAddr updates the leader address in the local DC's resolver.
func (s *ServerResolverBuilder) UpdateLeaderAddr(leaderAddr string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.leaderResolver.addr = leaderAddr
	s.leaderResolver.updateClientConn()
}

// serverResolver is a grpc Resolver that will keep a grpc.ClientConn up to date
// on the list of server addresses to use.
type serverResolver struct {
	// datacenter that can be reached by the clientConn. Used by ServerResolverBuilder
	// to filter resolvers for those in a specific datacenter.
	datacenter string

	// clientConn that this resolver is providing addresses for.
	clientConn resolver.ClientConn

	// close is used by ServerResolverBuilder to remove this resolver from the
	// index of resolvers. It is called by grpc when the connection is closed.
	close func()

	// addrs stores the list of addresses passed to updateAddrs, so that they
	// can be rebalanced periodically by ServerResolverBuilder.
	addrs    []resolver.Address
	addrLock sync.Mutex
}

var _ resolver.Resolver = (*serverResolver)(nil)

// updateAddrs updates this serverResolver's ClientConn to use the given set of
// addrs.
func (r *serverResolver) updateAddrs(addrs []resolver.Address) {
	r.addrLock.Lock()
	defer r.addrLock.Unlock()
	r.updateAddrsLocked(addrs)
}

// updateAddrsLocked updates this serverResolver's ClientConn to use the given
// set of addrs. addrLock must be held by caller.
func (r *serverResolver) updateAddrsLocked(addrs []resolver.Address) {
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

	r.addrs = addrs
}

func (r *serverResolver) Close() {
	r.close()
}

// ResolveNow is not used
func (*serverResolver) ResolveNow(resolver.ResolveNowOption) {}

type leaderResolver struct {
	addr       string
	clientConn resolver.ClientConn
}

func (l leaderResolver) ResolveNow(resolver.ResolveNowOption) {}

func (l leaderResolver) Close() {}

func (l leaderResolver) updateClientConn() {
	if l.addr == "" || l.clientConn == nil {
		return
	}
	addrs := []resolver.Address{
		{
			Addr:       l.addr,
			Type:       resolver.Backend,
			ServerName: "leader",
		},
	}
	l.clientConn.UpdateState(resolver.State{Addresses: addrs})
}

var _ resolver.Resolver = (*leaderResolver)(nil)
