package consul

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/ipaddr"
	"github.com/hashicorp/consul/lib/stringslice"
	"github.com/hashicorp/consul/logging"
)

// GatewayLocator assists in selecting an appropriate mesh gateway when wan
// federation via mesh gateways is enabled.
//
// This is exclusively used by the consul server itself when it needs to tunnel
// RPC or gossip through a mesh gateway to reach its ultimate destination.
//
// During secondary datacenter bootstrapping there is a phase where it is
// impossible for mesh gateways in the secondary datacenter to register
// themselves into the catalog to be discovered by the servers, so the servers
// maintain references for the mesh gateways in the primary in addition to its
// own local mesh gateways.
//
// After initial datacenter federation the primary mesh gateways are only used
// in extreme fallback situations (basically re-bootstrapping).
//
// For all other operations a consul server will ALWAYS contact a local mesh
// gateway to ultimately forward the request through a remote mesh gateway to
// reach its destination.
type GatewayLocator struct {
	logger            hclog.Logger
	srv               serverDelegate
	datacenter        string // THIS dc
	primaryDatacenter string

	// these ONLY contain ones that have the wanfed:1 meta
	gatewaysLock      sync.Mutex
	primaryGateways   []string // WAN addrs
	localGateways     []string // LAN addrs
	populatedGateways bool

	// primaryMeshGatewayDiscoveredAddresses is the current fallback addresses
	// for the mesh gateways in the primary datacenter.
	primaryMeshGatewayDiscoveredAddresses     []string
	primaryMeshGatewayDiscoveredAddressesLock sync.Mutex

	// This will be closed the FIRST time we get some gateways populated
	primaryGatewaysReadyCh   chan struct{}
	primaryGatewaysReadyOnce sync.Once

	// these are a collection of measurements that factor into deciding if we
	// should directly dial the primary's mesh gateways or if we should try to
	// route through our local gateways (if they are up).
	lastReplLock         sync.Mutex
	lastReplSuccess      time.Time
	lastReplFailure      time.Time
	lastReplSuccesses    uint64
	lastReplFailures     uint64
	useReplicationSignal bool // this should be set to true on the leader
}

// SetLastFederationStateReplicationError is used to indicate if the federation
// state replication loop has succeeded (nil) or failed during the last
// execution.
//
// Rather than introduce a completely new mechanism to periodically probe that
// our chosen mesh-gateway configuration can reach the primary's servers (like
// a ping or status RPC) we cheat and use the federation state replicator
// goroutine's success or failure as a proxy.
func (g *GatewayLocator) SetLastFederationStateReplicationError(err error, fromReplication bool) {
	if g == nil {
		return
	}

	g.lastReplLock.Lock()
	defer g.lastReplLock.Unlock()

	oldChoice := g.dialPrimaryThroughLocalGateway()
	if err == nil {
		g.lastReplSuccess = time.Now().UTC()
		g.lastReplSuccesses++
		g.lastReplFailures = 0
		if fromReplication {
			// If we get info from replication, assume replication is operating.
			g.useReplicationSignal = true
		}
	} else {
		g.lastReplFailure = time.Now().UTC()
		g.lastReplFailures++
		g.lastReplSuccesses = 0
	}
	newChoice := g.dialPrimaryThroughLocalGateway()
	if oldChoice != newChoice {
		g.logPrimaryDialingMessage(newChoice)
	}
}

func (g *GatewayLocator) SetUseReplicationSignal(newValue bool) {
	if g == nil {
		return
	}

	g.lastReplLock.Lock()
	g.useReplicationSignal = newValue
	g.lastReplLock.Unlock()
}

func (g *GatewayLocator) logPrimaryDialingMessage(useLocal bool) {
	if g.datacenter == g.primaryDatacenter {
		// These messages are useless when the server is in the primary
		// datacenter.
		return
	}
	if useLocal {
		g.logger.Info("will dial the primary datacenter using our local mesh gateways if possible")
	} else {
		g.logger.Info("will dial the primary datacenter through its mesh gateways")
	}
}

// DialPrimaryThroughLocalGateway determines if we should dial the primary's
// mesh gateways directly or use our local mesh gateways (if they are up).
//
// Generally the system has three states:
//
// 1. Servers dial primary MGWs using fallback addresses from the agent config.
// 2. Servers dial primary MGWs using replicated federation state data.
// 3. Servers dial primary MGWs indirectly through local MGWs.
//
// After initial bootstrapping most communication should go through (3). If the
// local mesh gateways are not coming up for chicken/egg problems (mostly the
// kind that arise from secondary datacenter bootstrapping) then (2) is useful
// to solve the chicken/egg problem and get back to (3). In the worst case
// where we completely lost communication with the primary AND all of their old
// mesh gateway addresses are changed then we need to go all the way back to
// square one and re-bootstrap via (1).
//
// Since both (1) and (2) are meant to be temporary we simplify things and make
// the system only consider two overall configurations: (1+2, with the
// addresses being unioned) or (3).
//
// This method returns true if in state (3) and false if in state (1+2).
func (g *GatewayLocator) DialPrimaryThroughLocalGateway() bool {
	if g.datacenter == g.primaryDatacenter {
		return false // not important
	}
	g.lastReplLock.Lock()
	defer g.lastReplLock.Unlock()
	return g.dialPrimaryThroughLocalGateway()
}

const localFederationStateReplicatorFailuresBeforeDialingDirectly = 3

func (g *GatewayLocator) dialPrimaryThroughLocalGateway() bool {
	if !g.useReplicationSignal {
		// Followers should blindly assume these gateways work. The leader will
		// try to bypass them and correct the replicated federation state info
		// that the followers will eventually pick up on.
		return true
	}
	if g.lastReplSuccess.IsZero() && g.lastReplFailure.IsZero() {
		return false // no data yet
	}

	if g.lastReplSuccess.After(g.lastReplFailure) {
		return true // we have viable data
	}

	if g.lastReplFailures < localFederationStateReplicatorFailuresBeforeDialingDirectly {
		return true // maybe it's just a little broken
	}

	return false
}

// PrimaryMeshGatewayAddressesReadyCh returns a channel that will be closed
// when federation state replication ships back at least one primary mesh
// gateway (not via fallback config).
func (g *GatewayLocator) PrimaryMeshGatewayAddressesReadyCh() <-chan struct{} {
	return g.primaryGatewaysReadyCh
}

// PickGateway returns the address for a gateway suitable for reaching the
// provided datacenter.
func (g *GatewayLocator) PickGateway(dc string) string {
	item := g.pickGateway(dc == g.primaryDatacenter)
	g.logger.Trace("picking gateway for transit", "gateway", item, "source_datacenter", g.datacenter, "dest_datacenter", dc)
	return item
}

func (g *GatewayLocator) pickGateway(primary bool) string {
	addrs := g.listGateways(primary)
	return getRandomItem(addrs)
}

func (g *GatewayLocator) listGateways(primary bool) []string {
	g.gatewaysLock.Lock()
	defer g.gatewaysLock.Unlock()

	if !g.populatedGateways {
		return nil // don't even do anything yet
	}

	var addrs []string
	if primary {
		if g.datacenter == g.primaryDatacenter {
			addrs = g.primaryGateways
		} else if g.DialPrimaryThroughLocalGateway() && len(g.localGateways) > 0 {
			addrs = g.localGateways
		} else {
			// Note calling StringSliceMergeSorted only works because both
			// inputs are pre-sorted. If for some reason one of the lists has
			// *duplicates* (which shouldn't happen) it's not great but it
			// won't break anything other than biasing our eventual random
			// choice a little bit.
			addrs = stringslice.MergeSorted(g.primaryGateways, g.PrimaryGatewayFallbackAddresses())
		}
	} else {
		addrs = g.localGateways
	}

	return addrs
}

// RefreshPrimaryGatewayFallbackAddresses is used to update the list of current
// fallback addresses for locating mesh gateways in the primary datacenter.
func (g *GatewayLocator) RefreshPrimaryGatewayFallbackAddresses(addrs []string) {
	sort.Strings(addrs)

	g.primaryMeshGatewayDiscoveredAddressesLock.Lock()
	defer g.primaryMeshGatewayDiscoveredAddressesLock.Unlock()

	if !stringslice.Equal(addrs, g.primaryMeshGatewayDiscoveredAddresses) {
		g.primaryMeshGatewayDiscoveredAddresses = addrs
		g.logger.Info("updated fallback list of primary mesh gateways", "mesh_gateways", addrs)
	}
}

// PrimaryGatewayFallbackAddresses returns the current set of discovered
// fallback addresses for the mesh gateways in the primary datacenter.
func (g *GatewayLocator) PrimaryGatewayFallbackAddresses() []string {
	g.primaryMeshGatewayDiscoveredAddressesLock.Lock()
	defer g.primaryMeshGatewayDiscoveredAddressesLock.Unlock()

	out := make([]string, len(g.primaryMeshGatewayDiscoveredAddresses))
	copy(out, g.primaryMeshGatewayDiscoveredAddresses)
	return out
}

func getRandomItem(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		idx := int(rand.Int31n(int32(len(items))))
		return items[idx]
	}
}

type serverDelegate interface {
	blockingQuery(queryOpts blockingQueryOptions, queryMeta blockingQueryResponseMeta, fn queryFn) error
	IsLeader() bool
	LeaderLastContact() time.Time
	setDatacenterSupportsFederationStates()
}

func NewGatewayLocator(
	logger hclog.Logger,
	srv serverDelegate,
	datacenter string,
	primaryDatacenter string,
) *GatewayLocator {
	g := &GatewayLocator{
		logger:                 logger.Named(logging.GatewayLocator),
		srv:                    srv,
		datacenter:             datacenter,
		primaryDatacenter:      primaryDatacenter,
		primaryGatewaysReadyCh: make(chan struct{}),
	}
	g.logPrimaryDialingMessage(g.DialPrimaryThroughLocalGateway())
	// initialize
	g.SetLastFederationStateReplicationError(nil, false)
	return g
}

var errGatewayLocalStateNotInitialized = errors.New("local state not initialized")

func (g *GatewayLocator) Run(ctx context.Context) {
	var lastFetchIndex uint64
	retryLoopBackoff(ctx, func() error {
		idx, err := g.runOnce(lastFetchIndex)
		if errors.Is(err, errGatewayLocalStateNotInitialized) {
			// don't do exponential backoff for something that's not broken
			return nil
		} else if err != nil {
			return err
		}

		lastFetchIndex = idx

		return nil
	}, func(err error) {
		g.logger.Error("error tracking primary and local mesh gateways", "error", err)
	})
}

func (g *GatewayLocator) runOnce(lastFetchIndex uint64) (uint64, error) {
	if err := g.checkLocalStateIsReady(); err != nil {
		return 0, err
	}

	// NOTE: we can't do RPC here because we won't have a token so we'll just
	// mostly assume that our FSM is caught up enough to answer locally.  If
	// this has drifted it's no different than a cache that drifts or an
	// inconsistent read.
	queryOpts := &structs.QueryOptions{
		MinQueryIndex:     lastFetchIndex,
		RequireConsistent: false,
	}

	var (
		results   []*structs.FederationState
		queryMeta structs.QueryMeta
	)
	err := g.srv.blockingQuery(
		queryOpts,
		&queryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			// Get the existing stored version of this config that has replicated down.
			// We could phone home to get this but that would incur extra WAN traffic
			// when we already have enough information locally to figure it out
			// (assuming that our replicator is still functioning).
			idx, all, err := state.FederationStateList(ws)
			if err != nil {
				return err
			}

			queryMeta.Index = idx
			results = all

			return nil
		})
	if err != nil {
		return 0, err
	}

	g.updateFromState(results)

	return queryMeta.Index, nil
}

// checkLocalStateIsReady is inlined a bit from (*Server).ForwardRPC(). We need to
// wait until our own state machine is safe to read from.
func (g *GatewayLocator) checkLocalStateIsReady() error {
	// Check if we can allow a stale read, ensure our local DB is initialized
	if !g.srv.LeaderLastContact().IsZero() {
		return nil // the raft leader talked to us
	}

	if g.srv.IsLeader() {
		return nil // we are the leader
	}

	return errGatewayLocalStateNotInitialized
}

func (g *GatewayLocator) updateFromState(results []*structs.FederationState) {
	if len(results) > 0 {
		g.srv.setDatacenterSupportsFederationStates()
	}

	var (
		local   structs.CheckServiceNodes
		primary structs.CheckServiceNodes
	)
	for _, config := range results {
		retained := retainGateways(config.MeshGateways)
		if config.Datacenter == g.datacenter {
			local = retained
		}
		// NOT else-if because conditionals are not mutually exclusive
		if config.Datacenter == g.primaryDatacenter {
			primary = retained
		}
	}

	primaryAddrs := renderGatewayAddrs(primary, true)
	localAddrs := renderGatewayAddrs(local, false)

	g.gatewaysLock.Lock()
	defer g.gatewaysLock.Unlock()

	g.populatedGateways = true

	changed := false
	primaryReady := false
	if !stringslice.Equal(g.primaryGateways, primaryAddrs) {
		g.primaryGateways = primaryAddrs
		primaryReady = len(g.primaryGateways) > 0
		changed = true
	}
	if !stringslice.Equal(g.localGateways, localAddrs) {
		g.localGateways = localAddrs
		changed = true
	}

	if changed {
		g.logger.Info(
			"new cached locations of mesh gateways",
			"primary", primaryAddrs,
			"local", localAddrs,
		)
	}

	if primaryReady {
		g.primaryGatewaysReadyOnce.Do(func() {
			close(g.primaryGatewaysReadyCh)
		})
	}
}

func retainGateways(full structs.CheckServiceNodes) structs.CheckServiceNodes {
	out := make([]structs.CheckServiceNode, 0, len(full))
	for _, csn := range full {
		if csn.Service.Meta[structs.MetaWANFederationKey] != "1" {
			continue
		}

		// only keep healthy ones
		ok := true
		for _, chk := range csn.Checks {
			if chk.Status == api.HealthCritical {
				ok = false
			}
		}

		if ok {
			out = append(out, csn)
		}
	}
	return out
}

func renderGatewayAddrs(gateways structs.CheckServiceNodes, wan bool) []string {
	out := make([]string, 0, len(gateways))
	for _, csn := range gateways {
		_, addr, port := csn.BestAddress(wan)
		completeAddr := ipaddr.FormatAddressPort(addr, port)
		out = append(out, completeAddr)
	}
	sort.Strings(out)
	return out
}
