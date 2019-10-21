package proxycfg

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/copystructure"
	"github.com/mitchellh/mapstructure"
)

type CacheNotifier interface {
	Notify(ctx context.Context, t string, r cache.Request,
		correlationID string, ch chan<- cache.UpdateEvent) error
}

const (
	coalesceTimeout                    = 200 * time.Millisecond
	rootsWatchID                       = "roots"
	leafWatchID                        = "leaf"
	intentionsWatchID                  = "intentions"
	serviceListWatchID                 = "service-list"
	federationStateListGatewaysWatchID = "federation-state-list-mesh-gateways"
	consulServerListWatchID            = "consul-server-list"
	datacentersWatchID                 = "datacenters"
	serviceResolversWatchID            = "service-resolvers"
	svcChecksWatchIDPrefix             = cachetype.ServiceHTTPChecksName + ":"
	serviceIDPrefix                    = string(structs.UpstreamDestTypeService) + ":"
	preparedQueryIDPrefix              = string(structs.UpstreamDestTypePreparedQuery) + ":"
	defaultPreparedQueryPollInterval   = 30 * time.Second
)

// state holds all the state needed to maintain the config for a registered
// connect-proxy service. When a proxy registration is changed, the entire state
// is discarded and a new one created.
type state struct {
	// logger, source and cache are required to be set before calling Watch.
	logger      hclog.Logger
	source      *structs.QuerySource
	cache       CacheNotifier
	serverSNIFn ServerSNIFunc

	// ctx and cancel store the context created during initWatches call
	ctx    context.Context
	cancel func()

	kind            structs.ServiceKind
	service         string
	proxyID         structs.ServiceID
	address         string
	port            int
	meta            map[string]string
	taggedAddresses map[string]structs.ServiceAddress
	proxyCfg        structs.ConnectProxyConfig
	token           string

	ch     chan cache.UpdateEvent
	snapCh chan ConfigSnapshot
	reqCh  chan chan *ConfigSnapshot
}

type ServerSNIFunc func(dc, nodeName string) string

func copyProxyConfig(ns *structs.NodeService) (structs.ConnectProxyConfig, error) {
	if ns == nil {
		return structs.ConnectProxyConfig{}, nil
	}
	// Copy the config map
	proxyCfgRaw, err := copystructure.Copy(ns.Proxy)
	if err != nil {
		return structs.ConnectProxyConfig{}, err
	}
	proxyCfg, ok := proxyCfgRaw.(structs.ConnectProxyConfig)
	if !ok {
		return structs.ConnectProxyConfig{}, errors.New("failed to copy proxy config")
	}

	// we can safely modify these since we just copied them
	for idx, _ := range proxyCfg.Upstreams {
		us := &proxyCfg.Upstreams[idx]
		if us.DestinationType != structs.UpstreamDestTypePreparedQuery && us.DestinationNamespace == "" {
			// default the upstreams target namespace to the namespace of the proxy
			// doing this here prevents needing much more complex logic a bunch of other
			// places and makes tracking these upstreams simpler as we can dedup them
			// with the maps tracking upstream ids being watched.
			proxyCfg.Upstreams[idx].DestinationNamespace = ns.EnterpriseMeta.NamespaceOrDefault()
		}
	}

	return proxyCfg, nil
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

	proxyCfg, err := copyProxyConfig(ns)
	if err != nil {
		return nil, err
	}

	taggedAddresses := make(map[string]structs.ServiceAddress)
	for k, v := range ns.TaggedAddresses {
		taggedAddresses[k] = v
	}

	meta := make(map[string]string)
	for k, v := range ns.Meta {
		meta[k] = v
	}

	return &state{
		kind:            ns.Kind,
		service:         ns.Service,
		proxyID:         ns.CompoundServiceID(),
		address:         ns.Address,
		port:            ns.Port,
		meta:            meta,
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
		EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
	}, "mesh-gateway:"+dc+":"+upstreamID, s.ch)
}

func (s *state) watchConnectProxyService(ctx context.Context, correlationId string, service string, dc string, filter string, entMeta *structs.EnterpriseMeta) error {
	var finalMeta structs.EnterpriseMeta
	finalMeta.Merge(entMeta)

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
		Source:         *s.source,
		EnterpriseMeta: finalMeta,
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
		Datacenter:     s.source.Datacenter,
		Token:          s.token,
		Service:        s.proxyCfg.DestinationServiceName,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
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
					Namespace: s.proxyID.NamespaceOrDefault(),
					Name:      s.proxyCfg.DestinationServiceName,
				},
			},
		},
	}, intentionsWatchID, s.ch)
	if err != nil {
		return err
	}

	// Watch for service check updates
	err = s.cache.Notify(s.ctx, cachetype.ServiceHTTPChecksName, &cachetype.ServiceHTTPChecksRequest{
		ServiceID:      s.proxyCfg.DestinationServiceID,
		EnterpriseMeta: s.proxyID.EnterpriseMeta,
	}, svcChecksWatchIDPrefix+structs.ServiceIDString(s.proxyCfg.DestinationServiceID, &s.proxyID.EnterpriseMeta), s.ch)
	if err != nil {
		return err
	}

	// default the namespace to the namespace of this proxy service
	currentNamespace := s.proxyID.NamespaceOrDefault()

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
			s.logger.Warn("failed to parse upstream config",
				"upstream", u.Identifier(),
				"error", err,
			)
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
			if err != nil {
				return err
			}

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
	err = s.cache.Notify(s.ctx, cachetype.CatalogServiceListName, &structs.DCSpecificRequest{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		Source:         *s.source,
		EnterpriseMeta: *structs.WildcardEnterpriseMeta(),
	}, serviceListWatchID, s.ch)

	if err != nil {
		return err
	}

	if s.meta[structs.MetaWANFederationKey] == "1" {
		// TODO(wanfed): conveniently we can just use this attribute in one
		// place here to set the machinery in motion and leave the conditional
		// behavior out of the rest
		err = s.cache.Notify(s.ctx, cachetype.FederationStateListMeshGatewaysName, &structs.DCSpecificRequest{
			Datacenter:   s.source.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.token},
			Source:       *s.source,
		}, federationStateListGatewaysWatchID, s.ch)
		if err != nil {
			return err
		}

		err = s.cache.Notify(s.ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
			Datacenter:   s.source.Datacenter,
			QueryOptions: structs.QueryOptions{Token: s.token},
			ServiceName:  structs.ConsulServiceName,
		}, consulServerListWatchID, s.ch)
		if err != nil {
			return err
		}
	}

	// Eventually we will have to watch connect enable instances for each service as well as the
	// destination services themselves but those notifications will be setup later. However we
	// cannot setup those watches until we know what the services are. from the service list
	// watch above

	err = s.cache.Notify(s.ctx, cachetype.CatalogDatacentersName, &structs.DatacentersRequest{
		QueryOptions: structs.QueryOptions{Token: s.token, MaxAge: 30 * time.Second},
	}, datacentersWatchID, s.ch)
	if err != nil {
		return err
	}

	// Once we start getting notified about the datacenters we will setup watches on the
	// gateways within those other datacenters. We cannot do that here because we don't
	// know what they are yet.

	// Watch service-resolvers so we can setup service subset clusters
	err = s.cache.Notify(s.ctx, cachetype.ConfigEntriesName, &structs.ConfigEntryQuery{
		Datacenter:     s.source.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: s.token},
		Kind:           structs.ServiceResolver,
		EnterpriseMeta: *structs.WildcardEnterpriseMeta(),
	}, serviceResolversWatchID, s.ch)

	if err != nil {
		s.logger.Named(logging.MeshGateway).
			Error("failed to register watch for service-resolver config entries", "error", err)
		return err
	}

	return err
}

func (s *state) initialConfigSnapshot() ConfigSnapshot {
	snap := ConfigSnapshot{
		Kind:            s.kind,
		Service:         s.service,
		ProxyID:         s.proxyID,
		Address:         s.address,
		Port:            s.port,
		ServiceMeta:     s.meta,
		TaggedAddresses: s.taggedAddresses,
		Proxy:           s.proxyCfg,
		Datacenter:      s.source.Datacenter,
		ServerSNIFn:     s.serverSNIFn,
	}

	switch s.kind {
	case structs.ServiceKindConnectProxy:
		snap.ConnectProxy.DiscoveryChain = make(map[string]*structs.CompiledDiscoveryChain)
		snap.ConnectProxy.WatchedUpstreams = make(map[string]map[string]context.CancelFunc)
		snap.ConnectProxy.WatchedUpstreamEndpoints = make(map[string]map[string]structs.CheckServiceNodes)
		snap.ConnectProxy.WatchedGateways = make(map[string]map[string]context.CancelFunc)
		snap.ConnectProxy.WatchedGatewayEndpoints = make(map[string]map[string]structs.CheckServiceNodes)
		snap.ConnectProxy.WatchedServiceChecks = make(map[structs.ServiceID][]structs.CheckType)

		snap.ConnectProxy.PreparedQueryEndpoints = make(map[string]structs.CheckServiceNodes) // TODO(rb): deprecated
	case structs.ServiceKindMeshGateway:
		snap.MeshGateway.WatchedServices = make(map[structs.ServiceID]context.CancelFunc)
		snap.MeshGateway.WatchedDatacenters = make(map[string]context.CancelFunc)
		snap.MeshGateway.ServiceGroups = make(map[structs.ServiceID]structs.CheckServiceNodes)
		snap.MeshGateway.GatewayGroups = make(map[string]structs.CheckServiceNodes)
		snap.MeshGateway.ServiceResolvers = make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry)
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
				s.logger.Error("watch error",
					"id", u.CorrelationID,
					"error", err,
				)
				continue
			}

		case <-sendCh:
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy, err := snap.Clone()
			if err != nil {
				s.logger.Error("Failed to copy config snapshot for proxy",
					"proxy", s.proxyID,
					"error", err,
				)
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
				s.logger.Error("Failed to copy config snapshot for proxy",
					"proxy", s.proxyID,
					"error", err,
				)
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
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	switch {
	case u.CorrelationID == rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots

	case u.CorrelationID == leafWatchID:
		leaf, ok := u.Result.(*structs.IssuedCert)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.ConnectProxy.Leaf = leaf

	case u.CorrelationID == intentionsWatchID:
		// Not in snapshot currently, no op

	case strings.HasPrefix(u.CorrelationID, "discovery-chain:"):
		resp, ok := u.Result.(*structs.DiscoveryChainResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		svc := strings.TrimPrefix(u.CorrelationID, "discovery-chain:")
		snap.ConnectProxy.DiscoveryChain[svc] = resp.Chain

		if err := s.resetWatchesFromChain(svc, resp.Chain, snap); err != nil {
			return err
		}

	case strings.HasPrefix(u.CorrelationID, "upstream-target:"):
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
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
			return fmt.Errorf("invalid type for response: %T", u.Result)
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

	case strings.HasPrefix(u.CorrelationID, "upstream:"+preparedQueryIDPrefix):
		resp, ok := u.Result.(*structs.PreparedQueryExecuteResponse)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		pq := strings.TrimPrefix(u.CorrelationID, "upstream:")
		snap.ConnectProxy.PreparedQueryEndpoints[pq] = resp.Nodes

	case strings.HasPrefix(u.CorrelationID, svcChecksWatchIDPrefix):
		resp, ok := u.Result.([]structs.CheckType)
		if !ok {
			return fmt.Errorf("invalid type for service checks response: %T, want: []structs.CheckType", u.Result)
		}
		svcID := structs.ServiceIDFromString(strings.TrimPrefix(u.CorrelationID, svcChecksWatchIDPrefix))
		snap.ConnectProxy.WatchedServiceChecks[svcID] = resp

	default:
		return fmt.Errorf("unknown correlation ID: %s", u.CorrelationID)
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
	s.logger.Trace("resetting watches for discovery chain", "id", id)
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
		s.logger.Trace("stopping watch of target",
			"upstream", id,
			"chain", chain.ServiceName,
			"target", targetID,
		)
		delete(snap.ConnectProxy.WatchedUpstreams[id], targetID)
		delete(snap.ConnectProxy.WatchedUpstreamEndpoints[id], targetID)
		cancelFn()
	}

	needGateways := make(map[string]struct{})
	for _, target := range chain.Targets {
		s.logger.Trace("initializing watch of target",
			"upstream", id,
			"chain", chain.ServiceName,
			"target", target.ID,
			"mesh-gateway-mode", target.MeshGateway.Mode,
		)

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
			target.GetEnterpriseMetadata(),
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

		s.logger.Trace("initializing watch of mesh gateway in datacenter",
			"upstream", id,
			"chain", chain.ServiceName,
			"datacenter", dc,
		)

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
		s.logger.Trace("stopping watch of mesh gateway in datacenter",
			"upstream", id,
			"chain", chain.ServiceName,
			"datacenter", dc,
		)
		delete(snap.ConnectProxy.WatchedGateways[id], dc)
		delete(snap.ConnectProxy.WatchedGatewayEndpoints[id], dc)
		cancelFn()
	}

	return nil
}

func (s *state) handleUpdateMeshGateway(u cache.UpdateEvent, snap *ConfigSnapshot) error {
	if u.Err != nil {
		return fmt.Errorf("error filling agent cache: %v", u.Err)
	}

	meshLogger := s.logger.Named(logging.MeshGateway)

	switch u.CorrelationID {
	case rootsWatchID:
		roots, ok := u.Result.(*structs.IndexedCARoots)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.Roots = roots
	case federationStateListGatewaysWatchID:
		dcIndexedNodes, ok := u.Result.(*structs.DatacenterIndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}
		snap.MeshGateway.FedStateGateways = dcIndexedNodes.DatacenterNodes
	case serviceListWatchID:
		services, ok := u.Result.(*structs.IndexedServiceList)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		svcMap := make(map[structs.ServiceID]struct{})
		for _, svc := range services.Services {
			sid := svc.ToServiceID()
			if _, ok := snap.MeshGateway.WatchedServices[sid]; !ok {
				ctx, cancel := context.WithCancel(s.ctx)
				err := s.cache.Notify(ctx, cachetype.HealthServicesName, &structs.ServiceSpecificRequest{
					Datacenter:     s.source.Datacenter,
					QueryOptions:   structs.QueryOptions{Token: s.token},
					ServiceName:    svc.Name,
					Connect:        true,
					EnterpriseMeta: sid.EnterpriseMeta,
				}, fmt.Sprintf("connect-service:%s", sid.String()), s.ch)

				if err != nil {
					meshLogger.Error("failed to register watch for connect-service",
						"service", sid.String(),
						"error", err,
					)
					cancel()
					return err
				}
				snap.MeshGateway.WatchedServices[sid] = cancel
				svcMap[sid] = struct{}{}
			}
		}

		for sid, cancelFn := range snap.MeshGateway.WatchedServices {
			if _, ok := svcMap[sid]; !ok {
				delete(snap.MeshGateway.WatchedServices, sid)
				cancelFn()
			}
		}

		snap.MeshGateway.WatchedServicesSet = true
	case datacentersWatchID:
		datacentersRaw, ok := u.Result.(*[]string)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
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
					EnterpriseMeta: *structs.DefaultEnterpriseMeta(),
				}, fmt.Sprintf("mesh-gateway:%s", dc), s.ch)

				if err != nil {
					meshLogger.Error("failed to register watch for mesh-gateway",
						"datacenter", dc,
						"error", err,
					)
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
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		resolvers := make(map[structs.ServiceID]*structs.ServiceResolverConfigEntry)
		for _, entry := range configEntries.Entries {
			if resolver, ok := entry.(*structs.ServiceResolverConfigEntry); ok {
				resolvers[structs.NewServiceID(resolver.Name, &resolver.EnterpriseMeta)] = resolver
			}
		}
		snap.MeshGateway.ServiceResolvers = resolvers

	case consulServerListWatchID:
		resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
		if !ok {
			return fmt.Errorf("invalid type for response: %T", u.Result)
		}

		// Do some initial sanity checks to avoid doing something dumb.
		for _, csn := range resp.Nodes {
			if csn.Service.Service != structs.ConsulServiceName {
				return fmt.Errorf("expected service name %q but got %q",
					structs.ConsulServiceName, csn.Service.Service)
			}
			if csn.Node.Datacenter != snap.Datacenter {
				return fmt.Errorf("expected datacenter %q but got %q",
					snap.Datacenter, csn.Node.Datacenter)
			}
		}

		snap.MeshGateway.ConsulServers = resp.Nodes

	default:
		switch {
		case strings.HasPrefix(u.CorrelationID, "connect-service:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
			}

			sid := structs.ServiceIDFromString(strings.TrimPrefix(u.CorrelationID, "connect-service:"))

			if len(resp.Nodes) > 0 {
				snap.MeshGateway.ServiceGroups[sid] = resp.Nodes
			} else if _, ok := snap.MeshGateway.ServiceGroups[sid]; ok {
				delete(snap.MeshGateway.ServiceGroups, sid)
			}
		case strings.HasPrefix(u.CorrelationID, "mesh-gateway:"):
			resp, ok := u.Result.(*structs.IndexedCheckServiceNodes)
			if !ok {
				return fmt.Errorf("invalid type for response: %T", u.Result)
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

	proxyCfg, err := copyProxyConfig(ns)
	if err != nil {
		s.logger.Warn("Failed to parse proxy config and will treat the new service as unchanged")
	}

	return ns.Kind != s.kind ||
		s.proxyID != ns.CompoundServiceID() ||
		s.address != ns.Address ||
		s.port != ns.Port ||
		!reflect.DeepEqual(s.proxyCfg, proxyCfg) ||
		s.token != token
}
