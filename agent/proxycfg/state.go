package proxycfg

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
)

type CacheNotifier interface {
	Notify(ctx context.Context, t string, r cache.Request,
		correlationID string, ch chan<- cache.UpdateEvent) error
}

const (
	coalesceTimeout                  = 200 * time.Millisecond
	rootsWatchID                     = "roots"
	leafWatchID                      = "leaf"
	intentionsWatchID                = "intentions"
	serviceListWatchID               = "service-list"
	datacentersWatchID               = "datacenters"
	serviceResolversWatchID          = "service-resolvers"
	serviceIDPrefix                  = string(structs.UpstreamDestTypeService) + ":"
	preparedQueryIDPrefix            = string(structs.UpstreamDestTypePreparedQuery) + ":"
	defaultPreparedQueryPollInterval = 30 * time.Second
)

// state holds all the state needed to maintain the config for a registered
// connect-proxy service. When a proxy registration is changed, the entire state
// is discarded and a new one created.
type state struct {
	// logger, source and cache are required to be set before calling Watch.
	logger *log.Logger
	source *structs.QuerySource
	cache  CacheNotifier

	// ctx and cancel store the context created during initWatches call
	ctx    context.Context
	cancel func()

	kind            structs.ServiceKind
	service         string
	proxyID         string
	address         string
	port            int
	taggedAddresses map[string]structs.ServiceAddress
	proxyCfg        structs.ConnectProxyConfig
	token           string

	ch     chan cache.UpdateEvent
	snapCh chan ConfigSnapshot
	reqCh  chan chan *ConfigSnapshot
}

// newState populates the state struct by copying relevant fields from the
// NodeService and Token. We copy so that we can use them in a separate
// goroutine later without reasoning about races with the NodeService passed
// (especially for embedded fields like maps and slices).
//
// The returned state needs its required dependencies to be set before Watch
// can be called.
func newState(ns *structs.NodeService, token string) (*state, error) {
	if ns.Kind != structs.ServiceKindConnectProxy && ns.Kind != structs.ServiceKindMeshGateway {
		return nil, errors.New("not a connect-proxy or mesh-gateway")
	}

	// Copy the config map
	proxyCfgRaw, err := copystructure.Copy(ns.Proxy)
	if err != nil {
		return nil, err
	}
	proxyCfg, ok := proxyCfgRaw.(structs.ConnectProxyConfig)
	if !ok {
		return nil, errors.New("failed to copy proxy config")
	}

	taggedAddresses := make(map[string]structs.ServiceAddress)
	for k, v := range ns.TaggedAddresses {
		taggedAddresses[k] = v
	}

	return &state{
		kind:            ns.Kind,
		service:         ns.Service,
		proxyID:         ns.ID,
		address:         ns.Address,
		port:            ns.Port,
		taggedAddresses: taggedAddresses,
		proxyCfg:        proxyCfg,
		token:           token,
		// 10 is fairly arbitrary here but allow for the 3 mandatory and a
		// reasonable number of upstream watches to all deliver their initial
		// messages in parallel without blocking the cache.Notify loops. It's not a
		// huge deal if we do for a short period so we don't need to be more
		// conservative to handle larger numbers of upstreams correctly but gives
		// some head room for normal operation to be non-blocking in most typical
		// cases.
		ch:     make(chan cache.UpdateEvent, 10),
		snapCh: make(chan ConfigSnapshot, 1),
		reqCh:  make(chan chan *ConfigSnapshot, 1),
	}, nil
}

// Watch initialized watches on all necessary cache data for the current proxy
// registration state and returns a chan to observe updates to the
// ConfigSnapshot that contains all necessary config state. The chan is closed
// when the state is Closed.
func (s *state) Watch() (<-chan ConfigSnapshot, error) {
	s.ctx, s.cancel = context.WithCancel(context.Background())

	err := s.initWatches()
	if err != nil {
		s.cancel()
		return nil, err
	}

	go s.run()

	return s.snapCh, nil
}

// Close discards the state and stops any long-running watches.
func (s *state) Close() error {
	if s.cancel != nil {
		s.cancel()
	}
	return nil
}

// initWatches sets up the watches needed for the particular service
func (s *state) initWatches() error {
	switch s.kind {
	case structs.ServiceKindConnectProxy:
		return s.initWatchesConnectProxy()
	case structs.ServiceKindMeshGateway:
		return s.initWatchesMeshGateway()
	default:
		return fmt.Errorf("Unsupported service kind")
	}
}

func (s *state) watchMeshGateway(ctx context.Context, dc string, upstreamID string) error {
	return s.cache.Notify(ctx, cachetype.InternalServiceDumpName, &structs.ServiceDumpRequest{
		Datacenter:     dc,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		ServiceKind:    structs.ServiceKindMeshGateway,
		UseServiceKind: true,
		Source:         *s.source,
	}, "mesh-gateway:"+dc+":"+upstreamID, s.ch)
}

func (s *state) watchConnectProxyService(ctx context.Context, correlationId string, service string, dc string, filter string) error {
	return s.cache.Notify(ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
		Datacenter: dc,
		QueryOptions: structs.QueryOptions{
			Token:  s.token,
			Filter: filter,
		},
		ServiceName: service,
		Connect:     true,
		// Note that Identifier doesn't type-prefix for service any more as it's
		// the default and makes metrics and other things much cleaner. It's
		// simpler for us if we have the type to make things unambiguous.
		Source: *s.source,
	}, correlationId, s.ch)
}

// initWatchesConnectProxy sets up the watches needed based on current proxy registration
// state.
func (s *state) initWatchesConnectProxy() error {
	// Watch for root changes
	err := s.cache.Notify(s.ctx, cachetype.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch the leaf cert
	err = s.cache.Notify(s.ctx, cachetype.ConnectCALeafName, &cachetype.ConnectCALeafRequest{
		Datacenter: s.source.Datacenter,
		Token:      s.token,
		Service:    s.proxyCfg.DestinationServiceName,
	}, leafWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch for intention updates
	err = s.cache.Notify(s.ctx, cachetype.IntentionMatchName, &structs.IntentionQueryRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Match: &structs.IntentionQueryMatch{
			Type: structs.IntentionMatchDestination,
			Entries: []structs.IntentionMatchEntry{
				{
					Namespace: structs.IntentionDefaultNamespace,
					Name:      s.proxyCfg.DestinationServiceName,
				},
			},
		},
	}, intentionsWatchID, s.ch)
	if err != nil {
		return err
	}

	// TODO(namespaces): pull this from something like s.source.Namespace?
	currentNamespace := "default"

	// Watch for updates to service endpoints for all upstreams
	for _, u := range s.proxyCfg.Upstreams {
		dc := s.source.Datacenter
		if u.Datacenter != "" {
			// TODO(rb): if we ASK for a specific datacenter, do we still use the chain?
			dc = u.Datacenter
		}

		ns := currentNamespace
		if u.DestinationNamespace != "" {
			ns = u.DestinationNamespace
		}

		cfg, err := parseReducedUpstreamConfig(u.Config)
		if err != nil {
			// Don't hard fail on a config typo, just warn. We'll fall back on
			// the plain discovery chain if there is an error so it's safe to
			// continue.
			s.logger.Printf("[WARN] envoy: failed to parse Upstream[%s].Config: %s",
				u.Identifier(), err)
		}

		switch u.DestinationType {
		case structs.UpstreamDestTypePreparedQuery:
			err = s.cache.Notify(s.ctx, cachetype.PreparedQueryName, &structs.PreparedQueryExecuteRequest{
				Datacenter:    dc,
				QueryOptions:  structs.QueryOptions{Token: s.token, MaxAge: defaultPreparedQueryPollInterval},
				QueryIDOrName: u.DestinationName,
				Connect:       true,
				Source:        *s.source,
			}, "upstream:"+u.Identifier(), s.ch)

		case structs.UpstreamDestTypeService:
			fallthrough

		case "": // Treat unset as the default Service type
			err = s.cache.Notify(s.ctx, cachetype.CompiledDiscoveryChainName, &structs.DiscoveryChainRequest{
				Datacenter:             s.source.Datacenter,
				QueryOptions:           structs.QueryOptions{Token: s.token},
				Name:                   u.DestinationName,
				EvaluateInDatacenter:   dc,
				EvaluateInNamespace:    ns,
				OverrideMeshGateway:    s.proxyCfg.MeshGateway.OverlayWith(u.MeshGateway),
				OverrideProtocol:       cfg.Protocol,
				OverrideConnectTimeout: cfg.ConnectTimeout(),
			}, "discovery-chain:"+u.Identifier(), s.ch)
			if err != nil {
				return err
			}

		default:
			return fmt.Errorf("unknown upstream type: %q", u.DestinationType)
		}
	}
	return nil
}

// reducedProxyConfig represents the basic opaque config values that are now
// managed with the discovery chain but for backwards compatibility reasons
// should still affect how the proxy is configured.
//
// The full-blown config is agent/xds.UpstreamConfig
type reducedUpstreamConfig struct {
	Protocol         string `mapstructure:"protocol"`
	ConnectTimeoutMs int    `mapstructure:"connect_timeout_ms"`
}

func (c *reducedUpstreamConfig) ConnectTimeout() time.Duration {
	return time.Duration(c.ConnectTimeoutMs) * time.Millisecond
}

func parseReducedUpstreamConfig(m map[string]interface{}) (reducedUpstreamConfig, error) {
	var cfg reducedUpstreamConfig
	err := mapstructure.WeakDecode(m, &cfg)
	return cfg, err
}

// initWatchesMeshGateway sets up the watches needed based on the current mesh gateway registration
func (s *state) initWatchesMeshGateway() error {
	// Watch for root changes
	err := s.cache.Notify(s.ctx, cachetype.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, rootsWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch for all services
	err = s.cache.Notify(s.ctx, cachetype.CatalogListServicesName, &structs.DCSpecificRequest{
		Datacenter:   s.source.Datacenter,
		QueryOptions: structs.QueryOptions{Token: s.token},
		Source:       *s.source,
	}, serviceListWatchID, s.ch)

	if err != nil {
		return err
	}

	// Eventually we will have to watch connect enable instances for each service as well as the
	// destination services themselves but those notifications will be setup later. However we
	// cannot setup those watches until we know what the services are. from the service list
	// watch above

	err = s.cache.Notify(s.ctx, cachetype.CatalogDatacentersName, &structs.DatacentersRequest{
		QueryOptions: structs.QueryOptions{Token: s.token, MaxAge: 30 * time.Second},
	}, datacentersWatchID, s.ch)

	// Once we start getting notified about the datacenters we will setup watches on the
	// gateways within those other datacenters. We cannot do that here because we don't
	// know what they are yet.

	return err
}

func (s *state) initialConfigSnapshot() ConfigSnapshot {
	snap := ConfigSnapshot{
		Kind:            s.kind,
		Service:         s.service,
		ProxyID:         s.proxyID,
		Address:         s.address,
		Port:            s.port,
		TaggedAddresses: s.taggedAddresses,
		Proxy:           s.proxyCfg,
		Datacenter:      s.source.Datacenter,
	}

	switch s.kind {
	case structs.ServiceKindConnectProxy:
		snap.ConnectProxy.DiscoveryChain = make(map[string]*structs.CompiledDiscoveryChain)
		snap.ConnectProxy.WatchedUpstreams = make(map[string]map[string]context.CancelFunc)
		snap.ConnectProxy.WatchedUpstreamEndpoints = make(map[string]map[string]structs.CheckServiceNodes)
		snap.ConnectProxy.WatchedGateways = make(map[string]map[string]context.CancelFunc)
		snap.ConnectProxy.WatchedGatewayEndpoints = make(map[string]map[string]structs.CheckServiceNodes)

		snap.ConnectProxy.UpstreamEndpoints = make(map[string]structs.CheckServiceNodes) // TODO(rb): deprecated
	case structs.ServiceKindMeshGateway:
		snap.MeshGateway.WatchedServices = make(map[string]context.CancelFunc)
		snap.MeshGateway.WatchedDatacenters = make(map[string]context.CancelFunc)
		snap.MeshGateway.ServiceGroups = make(map[string]structs.CheckServiceNodes)
		snap.MeshGateway.GatewayGroups = make(map[string]structs.CheckServiceNodes)
		// there is no need to initialize the map of service resolvers as we
		// fully rebuild it every time we get updates
	}

	return snap
}

func (s *state) run() {
	// Close the channel we return from Watch when we stop so consumers can stop
	// watching and clean up their goroutines. It's important we do this here and
	// not in Close since this routine sends on this chan and so might panic if it
	// gets closed from another goroutine.
	defer close(s.snapCh)

	snap := s.initialConfigSnapshot()

	// This turns out to be really fiddly/painful by just using time.Timer.C
	// directly in the code below since you can't detect when a timer is stopped
	// vs waiting in order to know to reset it. So just use a chan to send
	// ourselves messages.
	sendCh := make(chan struct{})
	var coalesceTimer *time.Timer

	for {
		select {
		case <-s.ctx.Done():
			return
		case u := <-s.ch:
			if err := s.handleUpdate(u, &snap); err != nil {
				s.logger.Printf("[ERR] %s watch error: %s", u.CorrelationID, err)
				continue
			}

		case <-sendCh:
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy, err := snap.Clone()
			if err != nil {
				s.logger.Printf("[ERR] Failed to copy config snapshot for proxy %s",
					s.proxyID)
				continue
			}
			s.snapCh <- *snapCopy
			// Allow the next change to trigger a send
			coalesceTimer = nil

			// Skip rest of loop - there is nothing to send since nothing changed on
			// this iteration
			continue

		case replyCh := <-s.reqCh:
			if !snap.Valid() {
				// Not valid yet just respond with nil and move on to next task.
				replyCh <- nil
				continue
			}
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy, err := snap.Clone()
			if err != nil {
				s.logger.Printf("[ERR] Failed to copy config snapshot for proxy %s",
					s.proxyID)
				continue
			}
			replyCh <- snapCopy

			// Skip rest of loop - there is nothing to send since nothing changed on
			// this iteration
			continue
		}

		// Check if snap is complete enough to be a valid config to deliver to a
		// proxy yet.
		if snap.Valid() {
			// Don't send it right away, set a short timer that will wait for updates
			// from any of the other cache values and deliver them all together.
			if coalesceTimer == nil {
				coalesceTimer = time.AfterFunc(coalesceTimeout, func() {
					// This runs in another goroutine so we can't just do the send
					// directly here as access to snap is racy. Instead, signal the main
					// loop above.
					sendCh <- struct{}{}
				})
			}
		}
	}
}

func (s *state) handleUpdate(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	switch s.kind {
	case structs.ServiceKindConnectProxy:
		return s.handleUpdateConnectProxy(u, snap)
	case structs.ServiceKindMeshGateway:
		return s.handleUpdateMeshGateway(u, snap)
	default:
		return fmt.Errorf("Unsupported service kind")
	}
}

func (s *state) handleUpdateConnectProxy(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots response: %T", u.Result)
		}
		snap.Roots = roots

	case u.CorrelationID == leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for leaf response: %T", u.Result)
		}
		snap.ConnectProxy.Leaf = leaf

	case u.CorrelationID == intentionsWatchID:
		// Not in snapshot currently, no op

	case strings.HasPrefix(u.CorrelationID, "discovery-chain:"):
		resp, ok := u.Result.(*structs.DiscoveryChainResponse)
		if !ok {
			return fmt.Errorf("invalid type for service response: %T", u.Result)
		}
		svc := strings.TrimPrefix(u.CorrelationID, "discovery-chain:")
		snap.ConnectProxy.DiscoveryChain[svc] = resp.Chain

		if err := s.resetWatchesFromChain(svc, resp.Chain, snap); err != nil {
			return err
		}

	case strings.HasPrefix(u.CorrelationID, "upstream-target:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for service response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "upstream-target:")
		targetID, svc, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}

		m, ok := snap.ConnectProxy.WatchedUpstreamEndpoints[svc]
		if !ok {
			m = make(map[string]structs.CheckServiceNodes)
			snap.ConnectProxy.WatchedUpstreamEndpoints[svc] = m
		}
		snap.ConnectProxy.WatchedUpstreamEndpoints[svc][targetID] = resp.Nodes

	case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for service response: %T", u.Result)
		}
		correlationID := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")
		dc, svc, ok := removeColonPrefix(correlationID)
		if !ok {
			return fmt.Errorf("invalid correlation id %q", u.CorrelationID)
		}
		m, ok := snap.ConnectProxy.WatchedGatewayEndpoints[svc]
		if !ok {
			m = make(map[string]structs.CheckServiceNodes)
			snap.ConnectProxy.WatchedGatewayEndpoints[svc] = m
		}
		snap.ConnectProxy.WatchedGatewayEndpoints[svc][dc] = resp.Nodes

	case strings.HasPrefix(u.CorrelationID, "upstream:"+serviceIDPrefix):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for service response: %T", u.Result)
		}
		svc := strings.TrimPrefix(u.CorrelationID, "upstream:"+serviceIDPrefix)
		snap.ConnectProxy.UpstreamEndpoints[svc] = resp.Nodes

	case strings.HasPrefix(u.CorrelationID, "upstream:"+preparedQueryIDPrefix):
		resp, ok := u.Result.(*structs.PreparedQueryExecuteResponse)
		if !ok {
			return fmt.Errorf("invalid type for prepared query response: %T", u.Result)
		}
		pq := strings.TrimPrefix(u.CorrelationID, "upstream:")
		snap.ConnectProxy.UpstreamEndpoints[pq] = resp.Nodes

	default:
		return errors.New("unknown correlation ID")
	}
	return nil
}

func removeColonPrefix(s string) (string, string, bool) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return "", "", false
	}
	return s[0:idx], s[idx+1:], true
}

func (s *state) resetWatchesFromChain(
	id string,
	chain *structs.CompiledDiscoveryChain,
	snap *ConfigSnapshot,
) error {
	if chain == nil {
		return fmt.Errorf("not possible to arrive here with no discovery chain")
	}

	// Initialize relevant sub maps.
	if _, ok := snap.ConnectProxy.WatchedUpstreams[id]; !ok {
		snap.ConnectProxy.WatchedUpstreams[id] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.ConnectProxy.WatchedUpstreamEndpoints[id]; !ok {
		snap.ConnectProxy.WatchedUpstreamEndpoints[id] = make(map[string]structs.CheckServiceNodes)
	}
	if _, ok := snap.ConnectProxy.WatchedGateways[id]; !ok {
		snap.ConnectProxy.WatchedGateways[id] = make(map[string]context.CancelFunc)
	}
	if _, ok := snap.ConnectProxy.WatchedGatewayEndpoints[id]; !ok {
		snap.ConnectProxy.WatchedGatewayEndpoints[id] = make(map[string]structs.CheckServiceNodes)
	}

	// We could invalidate this selectively based on a hash of the relevant
	// resolver information, but for now just reset anything about this
	// upstream when the chain changes in any way.
	//
	// TODO(rb): content hash based add/remove
	for targetID, cancelFn := range snap.ConnectProxy.WatchedUpstreams[id] {
		s.logger.Printf("[TRACE] proxycfg: upstream=%q:chain=%q: stopping watch of target %s", id, chain.ServiceName, targetID)
		delete(snap.ConnectProxy.WatchedUpstreams[id], targetID)
		delete(snap.ConnectProxy.WatchedUpstreamEndpoints[id], targetID)
		cancelFn()
	}

	needGateways := make(map[string]struct{})
	for _, target := range chain.Targets {
		s.logger.Printf("[TRACE] proxycfg: upstream=%q:chain=%q: initializing watch of target %s", id, chain.ServiceName, target.ID)

		// We'll get endpoints from the gateway query, but the health still has
		// to come from the backing service query.
		switch target.MeshGateway.Mode {
		case structs.MeshGatewayModeRemote:
			needGateways[target.Datacenter] = struct{}{}
		case structs.MeshGatewayModeLocal:
			needGateways[s.source.Datacenter] = struct{}{}
		}

		ctx, cancel := context.WithCancel(s.ctx)
		err := s.watchConnectProxyService(
			ctx,
			"upstream-target:"+target.ID+":"+id,
			target.Service,
			target.Datacenter,
			target.Subset.Filter,
		)
		if err != nil {
			cancel()
			return err
		}

		snap.ConnectProxy.WatchedUpstreams[id][target.ID] = cancel
	}

	for dc, _ := range needGateways {
		if _, ok := snap.ConnectProxy.WatchedGateways[id][dc]; ok {
			continue
		}

		s.logger.Printf("[TRACE] proxycfg: upstream=%q:chain=%q: initializing watch of mesh gateway in dc %s", id, chain.ServiceName, dc)

		ctx, cancel := context.WithCancel(s.ctx)
		err := s.watchMeshGateway(ctx, dc, id)
		if err != nil {
			cancel()
			return err
		}

		snap.ConnectProxy.WatchedGateways[id][dc] = cancel
	}

	for dc, cancelFn := range snap.ConnectProxy.WatchedGateways[id] {
		if _, ok := needGateways[dc]; ok {
			continue
		}
		s.logger.Printf("[TRACE] proxycfg: upstream=%q:chain=%q: stopping watch of mesh gateway in dc %s", id, chain.ServiceName, dc)
		delete(snap.ConnectProxy.WatchedGateways[id], dc)
		delete(snap.ConnectProxy.WatchedGatewayEndpoints[id], dc)
		cancelFn()
	}

	return nil
}

func (s *state) handleUpdateMeshGateway(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	switch u.CorrelationID {
	case rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for roots response: %T", u.Result)
		}
		snap.Roots = roots
	case serviceListWatchID:
		services, ok := u.Result.(*structs.IndexedServices)
		if !ok {
			return fmt.Errorf("invalid type for services response: %T", u.Result)
		}

		for svcName := range services.Services {
			if _, ok := snap.MeshGateway.WatchedServices[svcName]; !ok {
				ctx, cancel := context.WithCancel(s.ctx)
				err := s.cache.Notify(ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
					Datacenter:   s.source.Datacenter,
					QueryOptions: structs.QueryOptions{Token: s.token},
					ServiceName:  svcName,
					Connect:      true,
				}, fmt.Sprintf("connect-service:%s", svcName), s.ch)

				if err != nil {
					s.logger.Printf("[ERR] mesh-gateway: failed to register watch for connect-service:%s", svcName)
					cancel()
					return err
				}

				err = s.cache.Notify(ctx, cachetype.ConfigEntriesName, &structs.ConfigEntryQuery{
					Datacenter:   s.source.Datacenter,
					QueryOptions: structs.QueryOptions{Token: s.token},
					Kind:         structs.ServiceResolver,
				}, serviceResolversWatchID, s.ch)

				if err != nil {
					s.logger.Printf("[ERR] mesh-gateway: failed to register watch for service-resolver config entries")
					cancel()
					return err
				}
				snap.MeshGateway.WatchedServices[svcName] = cancel
			}
		}

		for svcName, cancelFn := range snap.MeshGateway.WatchedServices {
			if _, ok := services.Services[svcName]; !ok {
				delete(snap.MeshGateway.WatchedServices, svcName)
				cancelFn()
			}
		}
	case datacentersWatchID:
		datacentersRaw, ok := u.Result.(*[]string)
		if !ok {
			return fmt.Errorf("invalid type for datacenters response: %T", u.Result)
		}
		if datacentersRaw == nil {
			return fmt.Errorf("invalid response with a nil datacenter list")
		}

		datacenters := *datacentersRaw

		for _, dc := range datacenters {
			if dc == s.source.Datacenter {
				continue
			}

			if _, ok := snap.MeshGateway.WatchedDatacenters[dc]; !ok {
				ctx, cancel := context.WithCancel(s.ctx)
				err := s.cache.Notify(ctx, cachetype.InternalServiceDumpName, &structs.ServiceDumpRequest{
					Datacenter:     dc,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					ServiceKind:    structs.ServiceKindMeshGateway,
					UseServiceKind: true,
					Source:         *s.source,
				}, fmt.Sprintf("mesh-gateway:%s", dc), s.ch)

				if err != nil {
					s.logger.Printf("[ERR] mesh-gateway: failed to register watch for mesh-gateway:%s", dc)
					cancel()
					return err
				}

				snap.MeshGateway.WatchedDatacenters[dc] = cancel
			}
		}

		for dc, cancelFn := range snap.MeshGateway.WatchedDatacenters {
			found := false
			for _, dcCurrent := range datacenters {
				if dcCurrent == dc {
					found = true
					break
				}
			}

			if !found {
				delete(snap.MeshGateway.WatchedDatacenters, dc)
				cancelFn()
			}
		}
	case serviceResolversWatchID:
		configEntries, ok := u.Result.(*structs.IndexedConfigEntries)
		if !ok {
			return fmt.Errorf("invalid type for services response: %T", u.Result)
		}

		resolvers := make(map[string]*structs.ServiceResolverConfigEntry)
		for _, entry := range configEntries.Entries {
			if resolver, ok := entry.(*structs.ServiceResolverConfigEntry); ok {
				resolvers[resolver.Name] = resolver
			}
		}
		snap.MeshGateway.ServiceResolvers = resolvers
	default:
		switch {
		case strings.HasPrefix(u.CorrelationID, "connect-service:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for service response: %T", u.Result)
			}

			svc := strings.TrimPrefix(u.CorrelationID, "connect-service:")

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.ServiceGroups[svc] = resp.Nodes
			} else if _, ok := snap.MeshGateway.ServiceGroups[svc]; ok {
				delete(snap.MeshGateway.ServiceGroups, svc)
			}
		case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for service response: %T", u.Result)
			}

			dc := strings.TrimPrefix(u.CorrelationID, "mesh-gateway:")

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.GatewayGroups[dc] = resp.Nodes
			} else if _, ok := snap.MeshGateway.GatewayGroups[dc]; ok {
				delete(snap.MeshGateway.GatewayGroups, dc)
			}
		default:
			// do nothing for now
		}
	}

	return nil
}

// CurrentSnapshot synchronously returns the current ConfigSnapshot if there is
// one ready. If we don't have one yet because not all necessary parts have been
// returned (i.e. both roots and leaf cert), nil is returned.
func (s *state) CurrentSnapshot() *ConfigSnapshot {
	// Make a chan for the response to be sent on
	ch := make(chan *ConfigSnapshot, 1)
	s.reqCh <- ch
	// Wait for the response
	return <-ch
}

// Changed returns whether or not the passed NodeService has had any of the
// fields we care about for config state watching changed or a different token.
func (s *state) Changed(ns *structs.NodeService, token string) bool {
	if ns == nil {
		return true
	}
	return ns.Kind != s.kind ||
		s.proxyID != ns.ID ||
		s.address != ns.Address ||
		s.port != ns.Port ||
		!reflect.DeepEqual(s.proxyCfg, ns.Proxy) ||
		s.token != token
}
