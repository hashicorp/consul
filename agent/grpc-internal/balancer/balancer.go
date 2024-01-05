// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// package balancer implements a custom gRPC load balancer.
//
// Similarly to gRPC's built-in "pick_first" balancer, our balancer will pin the
// client to a single connection/server. However, it will switch servers as soon
// as an RPC error occurs (e.g. if the client has exhausted its rate limit on
// that server). It also provides a method that will be called periodically by
// the Consul router to randomize the connection priorities to rebalance load.
//
// Our balancer aims to keep exactly one TCP connection (to the current server)
// open at a time. This is different to gRPC's "round_robin" and "base" balancers
// which connect to *all* resolved addresses up-front so that you can quickly
// cycle between them - which we want to avoid because of the overhead on the
// servers. It's also slightly different to gRPC's "pick_first" balancer which
// will attempt to remain connected to the same server as long its address is
// returned by the resolver - we previously had to work around this behavior in
// order to shuffle the servers, which had some unfortunate side effects as
// documented in this issue: https://github.com/hashicorp/consul/issues/10603.
//
// If a server is in a perpetually bad state, the balancer's standard error
// handling will steer away from it but it will *not* be removed from the set
// and will remain in a TRANSIENT_FAILURE state to possibly be retried in the
// future. It is expected that Consul's router will remove servers from the
// resolver which have been network partitioned etc.
//
// Quick primer on how gRPC's different components work together:
//
//   - Targets (e.g. consul://.../server.dc1) represent endpoints/collections of
//     hosts. They're what you pass as the first argument to grpc.Dial.
//
//   - ClientConns represent logical connections to targets. Each ClientConn may
//     have many SubConns (and therefore TCP connections to different hosts).
//
//   - SubConns represent connections to a single host. They map 1:1 with TCP
//     connections (that's actually a bit of a lie, but true for our purposes).
//
//   - Resolvers are responsible for turning Targets into sets of addresses (e.g.
//     via DNS resolution) and updating the ClientConn when they change. They map
//     1:1 with ClientConns. gRPC creates them for a ClientConn using the builder
//     registered for the Target's scheme (i.e. the protocol part of the URL).
//
//   - Balancers are responsible for turning resolved addresses into SubConns and
//     a Picker. They're called whenever the Resolver updates the ClientConn's
//     state (e.g. with new addresses) or when the SubConns change state.
//
//     Like Resolvers, they also map 1:1 with ClientConns and are created using a
//     builder registered with a name that is specified in the "service config".
//
//   - Pickers are responsible for deciding which SubConn will be used for an RPC.
package balancer

import (
	"container/list"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	gbalancer "google.golang.org/grpc/balancer"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/status"
)

// NewBuilder constructs a new Builder. Calling Register will add the Builder
// to our global registry under the given "authority" such that it will be used
// when dialing targets in the form "consul-internal://<authority>/...", this
// allows us to add and remove balancers for different in-memory agents during
// tests.
func NewBuilder(authority string, logger hclog.Logger) *Builder {
	return &Builder{
		authority: authority,
		logger:    logger,
		byTarget:  make(map[string]*list.List),
		shuffler:  randomShuffler(),
	}
}

// Builder implements gRPC's balancer.Builder interface to construct balancers.
type Builder struct {
	authority string
	logger    hclog.Logger
	shuffler  shuffler

	mu       sync.Mutex
	byTarget map[string]*list.List
}

// Build is called by gRPC (e.g. on grpc.Dial) to construct a balancer for the
// given ClientConn.
func (b *Builder) Build(cc gbalancer.ClientConn, opts gbalancer.BuildOptions) gbalancer.Balancer {
	b.mu.Lock()
	defer b.mu.Unlock()

	targetURL := opts.Target.URL.String()

	logger := b.logger.With("target", targetURL)
	logger.Trace("creating balancer")

	bal := newBalancer(cc, opts.Target, logger)

	byTarget, ok := b.byTarget[targetURL]
	if !ok {
		byTarget = list.New()
		b.byTarget[targetURL] = byTarget
	}
	elem := byTarget.PushBack(bal)

	bal.closeFn = func() {
		logger.Trace("removing balancer")
		b.removeBalancer(targetURL, elem)
	}

	return bal
}

// removeBalancer is called when a Balancer is closed to remove it from our list.
func (b *Builder) removeBalancer(targetURL string, elem *list.Element) {
	b.mu.Lock()
	defer b.mu.Unlock()

	byTarget, ok := b.byTarget[targetURL]
	if !ok {
		return
	}
	byTarget.Remove(elem)

	if byTarget.Len() == 0 {
		delete(b.byTarget, targetURL)
	}
}

// Register the Builder in our global registry. Users should call Deregister
// when finished using the Builder to clean-up global state.
func (b *Builder) Register() {
	globalRegistry.register(b.authority, b)
}

// Deregister the Builder from our global registry to clean up state.
func (b *Builder) Deregister() {
	globalRegistry.deregister(b.authority)
}

// Rebalance randomizes the priority order of servers for the given target to
// rebalance load.
func (b *Builder) Rebalance(target resolver.Target) {
	b.mu.Lock()
	defer b.mu.Unlock()

	byTarget, ok := b.byTarget[target.URL.String()]
	if !ok {
		return
	}

	for item := byTarget.Front(); item != nil; item = item.Next() {
		item.Value.(*balancer).shuffleServerOrder(b.shuffler)
	}
}

func newBalancer(conn gbalancer.ClientConn, target resolver.Target, logger hclog.Logger) *balancer {
	return &balancer{
		conn:    conn,
		target:  target,
		logger:  logger,
		servers: resolver.NewAddressMap(),
	}
}

type balancer struct {
	conn    gbalancer.ClientConn
	target  resolver.Target
	logger  hclog.Logger
	closeFn func()

	mu            sync.Mutex
	subConn       gbalancer.SubConn
	connState     connectivity.State
	connError     error
	currentServer *serverInfo
	servers       *resolver.AddressMap
}

type serverInfo struct {
	addr       resolver.Address
	index      int       // determines the order in which servers will be attempted.
	lastFailed time.Time // used to steer away from servers that recently returned errors.
}

// String returns a log-friendly representation of the server.
func (si *serverInfo) String() string {
	if si == nil {
		return "<none>"
	}
	return si.addr.Addr
}

// Close is called by gRPC when the Balancer is no longer needed (e.g. when the
// ClientConn is closed by the application).
func (b *balancer) Close() { b.closeFn() }

// ResolverError is called by gRPC when the resolver reports an error. It puts
// the connection into a TRANSIENT_FAILURE state.
func (b *balancer) ResolverError(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Trace("resolver error", "error", err)
	b.handleErrorLocked(err)
}

// UpdateClientConnState is called by gRPC when the ClientConn changes state,
// such as when the resolver produces new addresses.
func (b *balancer) UpdateClientConnState(state gbalancer.ClientConnState) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	newAddrs := resolver.NewAddressMap()

	// Add any new addresses.
	for _, addr := range state.ResolverState.Addresses {
		newAddrs.Set(addr, struct{}{})

		if _, have := b.servers.Get(addr); !have {
			b.logger.Trace("adding server address", "address", addr.Addr)

			b.servers.Set(addr, &serverInfo{
				addr:  addr,
				index: b.servers.Len(),
			})
		}
	}

	// Delete any addresses that have been removed.
	for _, addr := range b.servers.Keys() {
		if _, have := newAddrs.Get(addr); !have {
			b.logger.Trace("removing server address", "address", addr.Addr)
			b.servers.Delete(addr)
		}
	}

	if b.servers.Len() == 0 {
		b.switchServerLocked(nil)
		b.handleErrorLocked(errors.New("resolver produced no addresses"))
		return gbalancer.ErrBadResolverState
	}

	b.maybeSwitchServerLocked()
	return nil
}

// UpdateSubConnState is called by gRPC when a SubConn changes state, such as
// when transitioning from CONNECTING to READY.
func (b *balancer) UpdateSubConnState(sc gbalancer.SubConn, state gbalancer.SubConnState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if sc != b.subConn {
		return
	}

	b.logger.Trace("sub-connection state changed", "server", b.currentServer, "state", state.ConnectivityState)
	b.connState = state.ConnectivityState
	b.connError = state.ConnectionError

	// Note: it's not clear whether this can actually happen or not. It would mean
	// the sub-conn was shut down by something other than us calling RemoveSubConn.
	if state.ConnectivityState == connectivity.Shutdown {
		b.switchServerLocked(nil)
		return
	}

	b.updatePickerLocked()
}

// handleErrorLocked puts the ClientConn into a TRANSIENT_FAILURE state and
// causes the picker to return the given error on Pick.
//
// Note: b.mu must be held when calling this method.
func (b *balancer) handleErrorLocked(err error) {
	b.connState = connectivity.TransientFailure
	b.connError = fmt.Errorf("resolver error: %w", err)
	b.updatePickerLocked()
}

// maybeSwitchServerLocked switches server if the one we're currently connected
// to is no longer our preference (e.g. based on error state).
//
// Note: b.mu must be held when calling this method.
func (b *balancer) maybeSwitchServerLocked() {
	if ideal := b.idealServerLocked(); ideal != b.currentServer {
		b.switchServerLocked(ideal)
	}
}

// idealServerLocked determines which server we should currently be connected to
// when taking the error state and rebalance-shuffling into consideration.
//
// Returns nil if there isn't a suitable server.
//
// Note: b.mu must be held when calling this method.
func (b *balancer) idealServerLocked() *serverInfo {
	candidates := make([]*serverInfo, b.servers.Len())
	for idx, v := range b.servers.Values() {
		candidates[idx] = v.(*serverInfo)
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(a, b int) bool {
		ca, cb := candidates[a], candidates[b]

		return ca.lastFailed.Before(cb.lastFailed) ||
			(ca.lastFailed.Equal(cb.lastFailed) && ca.index < cb.index)
	})
	return candidates[0]
}

// switchServerLocked switches to the given server, creating a new connection
// and tearing down the previous connection.
//
// It's expected for either/neither/both of b.currentServer and newServer to be nil.
//
// Note: b.mu must be held when calling this method.
func (b *balancer) switchServerLocked(newServer *serverInfo) {
	b.logger.Debug("switching server", "from", b.currentServer, "to", newServer)

	prevConn := b.subConn
	b.currentServer = newServer

	if newServer == nil {
		b.subConn = nil
	} else {
		var err error
		b.subConn, err = b.conn.NewSubConn([]resolver.Address{newServer.addr}, gbalancer.NewSubConnOptions{})
		if err == nil {
			b.subConn.Connect()
			b.connState = connectivity.Connecting
		} else {
			b.logger.Trace("failed to create sub-connection", "addr", newServer.addr, "error", err)
			b.handleErrorLocked(fmt.Errorf("failed to create sub-connection: %w", err))
			return
		}
	}

	b.updatePickerLocked()

	if prevConn != nil {
		b.conn.RemoveSubConn(prevConn)
	}
}

// updatePickerLocked updates the ClientConn's Picker based on the balancer's
// current state.
//
// Note: b.mu must be held when calling this method.
func (b *balancer) updatePickerLocked() {
	var p gbalancer.Picker
	switch b.connState {
	case connectivity.Connecting:
		p = errPicker{err: gbalancer.ErrNoSubConnAvailable}
	case connectivity.TransientFailure:
		p = errPicker{err: b.connError}
	case connectivity.Idle:
		p = idlePicker{conn: b.subConn}
	case connectivity.Ready:
		srv := b.currentServer

		p = readyPicker{
			conn: b.subConn,
			errFn: func(err error) {
				b.witnessError(srv, err)
			},
		}
	default:
		// Note: shutdown state is handled in UpdateSubConnState.
		b.logger.Trace("connection in unexpected state", "state", b.connState)
	}

	b.conn.UpdateState(gbalancer.State{
		ConnectivityState: b.connState,
		Picker:            p,
	})
}

// witnessError marks the given server as having failed and triggers a switch
// if required.
func (b *balancer) witnessError(server *serverInfo, err error) {
	// The following status codes represent errors that probably won't be solved
	// by switching servers, so we shouldn't bother disrupting in-flight streams.
	switch status.Code(err) {
	case codes.Canceled,
		codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated:
		return
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Trace("witnessed RPC error", "server", server, "error", err)
	server.lastFailed = time.Now()
	b.maybeSwitchServerLocked()
}

// shuffleServerOrder re-prioritizes the servers using the given shuffler, it
// also unsets the lastFailed timestamp (to prevent us *never* connecting to a
// server that previously failed).
func (b *balancer) shuffleServerOrder(shuffler shuffler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.logger.Trace("shuffling server order")

	addrs := b.servers.Keys()
	shuffler(addrs)

	for idx, addr := range addrs {
		v, ok := b.servers.Get(addr)
		if !ok {
			continue
		}

		srv := v.(*serverInfo)
		srv.index = idx
		srv.lastFailed = time.Time{}
	}
	b.maybeSwitchServerLocked()
}

// errPicker returns the given error on Pick.
type errPicker struct{ err error }

func (p errPicker) Pick(gbalancer.PickInfo) (gbalancer.PickResult, error) {
	return gbalancer.PickResult{}, p.err
}

// idlePicker attempts to re-establish the given (idle) connection on Pick.
type idlePicker struct{ conn gbalancer.SubConn }

func (p idlePicker) Pick(gbalancer.PickInfo) (gbalancer.PickResult, error) {
	p.conn.Connect()
	return gbalancer.PickResult{}, gbalancer.ErrNoSubConnAvailable
}

// readyPicker returns the given connection on Pick. errFn will be called if
// the RPC fails (i.e. to switch to another server).
type readyPicker struct {
	conn  gbalancer.SubConn
	errFn func(error)
}

func (p readyPicker) Pick(info gbalancer.PickInfo) (gbalancer.PickResult, error) {
	return gbalancer.PickResult{
		SubConn: p.conn,
		Done: func(done gbalancer.DoneInfo) {
			if err := done.Err; err != nil {
				p.errFn(err)
			}
		},
	}, nil
}

// shuffler is used to change the priority order of servers, to rebalance load.
type shuffler func([]resolver.Address)

// randomShuffler randomizes the priority order.
func randomShuffler() shuffler {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))

	return func(addrs []resolver.Address) {
		rand.Shuffle(len(addrs), func(a, b int) {
			addrs[a], addrs[b] = addrs[b], addrs[a]
		})
	}
}
