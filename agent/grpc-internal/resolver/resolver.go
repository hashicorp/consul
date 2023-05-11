package resolver

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc/resolver"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/types"
)

// ServerResolverBuilder tracks the current server list and keeps any
// ServerResolvers updated when changes occur.
type ServerResolverBuilder struct {
	cfg Config

	// leaderResolver is used to track the address of the leader in the local DC.
	leaderResolver leaderResolver

	// servers is an index of Servers by area and Server.ID. The map contains server IDs
	// for all datacenters.
	servers map[types.AreaID]map[string]*metadata.Server

	// resolvers is an index of connections to the serverResolver which manages
	// addresses of servers for that connection.
	//
	// this is only applicable for non-leader conn types
	resolvers map[resolver.ClientConn]*serverResolver

	// lock for all stateful fields (excludes config which is immutable).
	lock sync.RWMutex
}

type Config struct {
	// Datacenter is the datacenter of this agent.
	Datacenter string

	// AgentType is either 'server' or 'client' and is required.
	AgentType string

	// Authority used to query the server. Defaults to "". Used to support
	// parallel testing because gRPC registers resolvers globally.
	Authority string
}

func NewServerResolverBuilder(cfg Config) *ServerResolverBuilder {
	if cfg.Datacenter == "" {
		panic("ServerResolverBuilder needs Config.Datacenter to be nonempty")
	}
	switch cfg.AgentType {
	case "server", "client":
	default:
		panic("ServerResolverBuilder needs Config.AgentType to be either server or client")
	}
	return &ServerResolverBuilder{
		cfg:       cfg,
		servers:   make(map[types.AreaID]map[string]*metadata.Server),
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

// ServerForGlobalAddr returns server metadata for a server with the specified globally unique address.
func (s *ServerResolverBuilder) ServerForGlobalAddr(globalAddr string) (*metadata.Server, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	for _, areaServers := range s.servers {
		for _, server := range areaServers {
			if DCPrefix(server.Datacenter, server.Addr.String()) == globalAddr {
				return server, nil
			}
		}
	}

	return nil, fmt.Errorf("failed to find Consul server for global address %q", globalAddr)
}

// Build returns a new serverResolver for the given ClientConn. The resolver
// will keep the ClientConn's state updated based on updates from Serf.
func (s *ServerResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, _ resolver.BuildOptions) (resolver.Resolver, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// If there's already a resolver for this connection, return it.
	// TODO(streaming): how would this happen since we already cache connections in ClientConnPool?
	if cc == s.leaderResolver.clientConn {
		return s.leaderResolver, nil
	}
	if resolver, ok := s.resolvers[cc]; ok {
		return resolver, nil
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
func (s *ServerResolverBuilder) AddServer(areaID types.AreaID, server *metadata.Server) {
	if s.shouldIgnoreServer(areaID, server) {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	areaServers, ok := s.servers[areaID]
	if !ok {
		areaServers = make(map[string]*metadata.Server)
		s.servers[areaID] = areaServers
	}

	areaServers[uniqueID(server)] = server

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

// DCPrefix prefixes the given string with a datacenter for use in
// disambiguation.
func DCPrefix(datacenter, suffix string) string {
	return datacenter + "-" + suffix
}

// RemoveServer updates the resolvers' states with the given server removed.
func (s *ServerResolverBuilder) RemoveServer(areaID types.AreaID, server *metadata.Server) {
	if s.shouldIgnoreServer(areaID, server) {
		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	areaServers, ok := s.servers[areaID]
	if !ok {
		return // already gone
	}

	delete(areaServers, uniqueID(server))
	if len(areaServers) == 0 {
		delete(s.servers, areaID)
	}

	addrs := s.getDCAddrs(server.Datacenter)
	for _, resolver := range s.resolvers {
		if resolver.datacenter == server.Datacenter {
			resolver.updateAddrs(addrs)
		}
	}
}

// shouldIgnoreServer is used to contextually decide if a particular kind of
// server should be accepted into a given area.
//
// On client agents it's pretty easy: clients only participate in the standard
// LAN, so we only accept servers from the LAN.
//
// On server agents it's a little less obvious. This resolver is ultimately
// used to have servers dial other servers. If a server is going to cross
// between datacenters (using traditional federation) then we want to use the
// WAN addresses for them, but if a server is going to dial a sibling server in
// the same datacenter we want it to use the LAN addresses always. To achieve
// that here we simply never allow WAN servers for our current datacenter to be
// added into the resolver, letting only the LAN instances through.
func (s *ServerResolverBuilder) shouldIgnoreServer(areaID types.AreaID, server *metadata.Server) bool {
	if s.cfg.AgentType == "client" && areaID != types.AreaLAN {
		return true
	}

	if s.cfg.AgentType == "server" &&
		server.Datacenter == s.cfg.Datacenter &&
		areaID != types.AreaLAN {
		return true
	}

	return false
}

// getDCAddrs returns a list of the server addresses for the given datacenter.
// This method requires that lock is held for reads.
func (s *ServerResolverBuilder) getDCAddrs(dc string) []resolver.Address {
	lanRequest := (s.cfg.Datacenter == dc)

	var (
		addrs         []resolver.Address
		keptServerIDs = make(map[string]struct{})
	)
	for areaID, areaServers := range s.servers {
		if (areaID == types.AreaLAN) != lanRequest {
			// LAN requests only look at LAN data. WAN requests only look at
			// WAN data.
			continue
		}
		for _, server := range areaServers {
			if server.Datacenter != dc {
				continue
			}

			// Servers may be part of multiple areas, so only include each one once.
			if _, ok := keptServerIDs[server.ID]; ok {
				continue
			}
			keptServerIDs[server.ID] = struct{}{}

			addrs = append(addrs, resolver.Address{
				// NOTE: the address persisted here is only dialable using our custom dialer
				Addr:       DCPrefix(server.Datacenter, server.Addr.String()),
				ServerName: server.Name,
			})
		}
	}
	return addrs
}

// UpdateLeaderAddr updates the leader address in the local DC's resolver.
func (s *ServerResolverBuilder) UpdateLeaderAddr(datacenter, addr string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.leaderResolver.globalAddr = DCPrefix(datacenter, addr)
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
	r.clientConn.UpdateState(resolver.State{Addresses: addrs})
	r.addrs = addrs
}

func (r *serverResolver) Close() {
	r.close()
}

// ResolveNow is not used
func (*serverResolver) ResolveNow(options resolver.ResolveNowOptions) {}

type leaderResolver struct {
	globalAddr string
	clientConn resolver.ClientConn
}

func (l leaderResolver) ResolveNow(resolver.ResolveNowOptions) {}

func (l leaderResolver) Close() {}

func (l leaderResolver) updateClientConn() {
	if l.globalAddr == "" || l.clientConn == nil {
		return
	}
	addrs := []resolver.Address{
		{
			// NOTE: the address persisted here is only dialable using our custom dialer
			Addr:       l.globalAddr,
			ServerName: "leader",
		},
	}
	l.clientConn.UpdateState(resolver.State{Addresses: addrs})
}

var _ resolver.Resolver = (*leaderResolver)(nil)
