// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxycfg

import (
	"context"
	"errors"
	"fmt"
	"net"
	"reflect"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-hclog"
	"golang.org/x/time/rate"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
)

const (
	coalesceTimeout                    = 200 * time.Millisecond
	rootsWatchID                       = "roots"
	peeringTrustBundlesWatchID         = "peering-trust-bundles"
	leafWatchID                        = "leaf"
	peerTrustBundleIDPrefix            = "peer-trust-bundle:"
	intentionsWatchID                  = "intentions"
	serviceListWatchID                 = "service-list"
	peeringServiceListWatchID          = "peering-service-list:"
	federationStateListGatewaysWatchID = "federation-state-list-mesh-gateways"
	consulServerListWatchID            = "consul-server-list"
	datacentersWatchID                 = "datacenters"
	serviceResolversWatchID            = "service-resolvers"
	gatewayServicesWatchID             = "gateway-services"
	gatewayConfigWatchID               = "gateway-config"
	inlineCertificateConfigWatchID     = "inline-certificate-config"
	routeConfigWatchID                 = "route-config"
	externalServiceIDPrefix            = "external-service:"
	serviceLeafIDPrefix                = "service-leaf:"
	serviceConfigIDPrefix              = "service-config:"
	serviceResolverIDPrefix            = "service-resolver:"
	serviceIntentionsIDPrefix          = "service-intentions:"
	intentionUpstreamsID               = "intention-upstreams"
	jwtProviderID                      = "jwt-provider"
	peerServersWatchID                 = "peer-servers"
	peeredUpstreamsID                  = "peered-upstreams"
	intentionUpstreamsDestinationID    = "intention-upstreams-destination"
	upstreamPeerWatchIDPrefix          = "upstream-peer:"
	exportedServiceListWatchID         = "exported-service-list"
	meshConfigEntryID                  = "mesh"
	DestinationConfigEntryID           = "destination:"
	DestinationGatewayID               = "dest-gateway:"
	svcChecksWatchIDPrefix             = cachetype.ServiceHTTPChecksName + ":"
	preparedQueryIDPrefix              = string(structs.UpstreamDestTypePreparedQuery) + ":"
	defaultPreparedQueryPollInterval   = 30 * time.Second
)

type stateConfig struct {
	logger                hclog.Logger
	source                *structs.QuerySource
	dataSources           DataSources
	dnsConfig             DNSConfig
	serverSNIFn           ServerSNIFunc
	intentionDefaultAllow bool
}

// state holds all the state needed to maintain the config for a registered
// connect-proxy service. When a proxy registration is changed, the entire state
// is discarded and a new one created.
type state struct {
	source          ProxySource
	logger          hclog.Logger
	serviceInstance serviceInstance
	handler         kindHandler

	// cancel is set by Watch and called by Close to stop the goroutine started
	// in Watch.
	cancel func()

	// failedFlag is (atomically) set to 1 (by Close) when run exits because a data
	// source is in an irrecoverable state. It can be read with failed.
	failedFlag int32

	ch     chan UpdateEvent
	snapCh chan ConfigSnapshot
	reqCh  chan chan *ConfigSnapshot
	doneCh chan struct{}

	rateLimiter *rate.Limiter
}

func (s *state) stoppedRunning() bool {
	select {
	case <-s.doneCh:
		return true
	default:
		return false
	}
}

// failed returns whether run exited because a data source is in an
// irrecoverable state.
func (s *state) failed() bool {
	return atomic.LoadInt32(&s.failedFlag) == 1
}

type DNSConfig struct {
	Domain    string
	AltDomain string
}

type ServerSNIFunc func(dc, nodeName string) string

type serviceInstance struct {
	kind            structs.ServiceKind
	service         string
	proxyID         ProxyID
	address         string
	port            int
	meta            map[string]string
	taggedAddresses map[string]structs.ServiceAddress
	proxyCfg        structs.ConnectProxyConfig
	token           string
	locality        *structs.Locality
}

func copyProxyConfig(ns *structs.NodeService) (structs.ConnectProxyConfig, error) {
	if ns == nil {
		return structs.ConnectProxyConfig{}, nil
	}

	proxyCfg := *(&ns.Proxy).DeepCopy()

	// we can safely modify these since we just copied them
	for idx := range proxyCfg.Upstreams {
		us := &proxyCfg.Upstreams[idx]
		if us.DestinationType != structs.UpstreamDestTypePreparedQuery {
			// default the upstreams target namespace and partition to those of the proxy
			// doing this here prevents needing much more complex logic a bunch of other
			// places and makes tracking these upstreams simpler as we can dedup them
			// with the maps tracking upstream ids being watched.
			if us.DestinationPartition == "" {
				proxyCfg.Upstreams[idx].DestinationPartition = ns.EnterpriseMeta.PartitionOrDefault()
			}
			if us.DestinationNamespace == "" {
				proxyCfg.Upstreams[idx].DestinationNamespace = ns.EnterpriseMeta.NamespaceOrDefault()
			}

			// If PeerName is not empty, the DestinationPartition refers
			// to the local Partition in which the Peer exists and the
			// DestinationNamespace refers to the Namespace residing in
			// the remote peer
			if us.DestinationPeer == "" {
				proxyCfg.Upstreams[idx].DestinationPeer = ns.PeerName
			}
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
func newState(id ProxyID, ns *structs.NodeService, source ProxySource, token string, config stateConfig, rateLimiter *rate.Limiter) (*state, error) {
	// 10 is fairly arbitrary here but allow for the 3 mandatory and a
	// reasonable number of upstream watches to all deliver their initial
	// messages in parallel without blocking the cache.Notify loops. It's not a
	// huge deal if we do for a short period so we don't need to be more
	// conservative to handle larger numbers of upstreams correctly but gives
	// some head room for normal operation to be non-blocking in most typical
	// cases.
	ch := make(chan UpdateEvent, 10)

	s, err := newServiceInstanceFromNodeService(id, ns, token)
	if err != nil {
		return nil, err
	}

	handler, err := newKindHandler(config, s, ch)
	if err != nil {
		return nil, err
	}

	return &state{
		source:          source,
		logger:          config.logger.With("proxy", s.proxyID, "kind", s.kind),
		serviceInstance: s,
		handler:         handler,
		ch:              ch,
		snapCh:          make(chan ConfigSnapshot, 1),
		reqCh:           make(chan chan *ConfigSnapshot, 1),
		doneCh:          make(chan struct{}),
		rateLimiter:     rateLimiter,
	}, nil
}

func newKindHandler(config stateConfig, s serviceInstance, ch chan UpdateEvent) (kindHandler, error) {
	var handler kindHandler
	h := handlerState{stateConfig: config, serviceInstance: s, ch: ch}

	switch s.kind {
	case structs.ServiceKindConnectProxy:
		handler = &handlerConnectProxy{handlerState: h}
	case structs.ServiceKindTerminatingGateway:
		h.stateConfig.logger = config.logger.Named(logging.TerminatingGateway)
		handler = &handlerTerminatingGateway{handlerState: h}
	case structs.ServiceKindMeshGateway:
		h.stateConfig.logger = config.logger.Named(logging.MeshGateway)
		handler = &handlerMeshGateway{handlerState: h}
	case structs.ServiceKindIngressGateway:
		handler = &handlerIngressGateway{handlerState: h}
	case structs.ServiceKindAPIGateway:
		handler = &handlerAPIGateway{handlerState: h}
	default:
		return nil, errors.New("not a connect-proxy, terminating-gateway, mesh-gateway, or ingress-gateway")
	}

	return handler, nil
}

func newServiceInstanceFromNodeService(id ProxyID, ns *structs.NodeService, token string) (serviceInstance, error) {
	proxyCfg, err := copyProxyConfig(ns)
	if err != nil {
		return serviceInstance{}, err
	}

	taggedAddresses := make(map[string]structs.ServiceAddress)
	for k, v := range ns.TaggedAddresses {
		taggedAddresses[k] = v
	}

	meta := make(map[string]string)
	for k, v := range ns.Meta {
		meta[k] = v
	}

	return serviceInstance{
		kind:            ns.Kind,
		service:         ns.Service,
		locality:        ns.Locality,
		proxyID:         id,
		address:         ns.Address,
		port:            ns.Port,
		meta:            meta,
		taggedAddresses: taggedAddresses,
		proxyCfg:        proxyCfg,
		token:           token,
	}, nil
}

type kindHandler interface {
	initialize(ctx context.Context) (ConfigSnapshot, error)
	handleUpdate(ctx context.Context, u UpdateEvent, snap *ConfigSnapshot) error
}

// Watch initialized watches on all necessary cache data for the current proxy
// registration state and returns a chan to observe updates to the
// ConfigSnapshot that contains all necessary config state. The chan is closed
// when the state is Closed.
func (s *state) Watch() (<-chan ConfigSnapshot, error) {
	var ctx context.Context
	ctx, s.cancel = context.WithCancel(context.Background())

	snap, err := s.handler.initialize(ctx)
	if err != nil {
		s.cancel()
		return nil, err
	}

	go s.run(ctx, &snap)

	return s.snapCh, nil
}

// Close discards the state and stops any long-running watches.
func (s *state) Close(failed bool) error {
	if s.stoppedRunning() {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if failed {
		atomic.StoreInt32(&s.failedFlag, 1)
	}
	return nil
}

type handlerState struct {
	stateConfig     // TODO: un-embed
	serviceInstance // TODO: un-embed
	ch              chan UpdateEvent
}

func newConfigSnapshotFromServiceInstance(s serviceInstance, config stateConfig) ConfigSnapshot {
	// TODO: use serviceInstance type in ConfigSnapshot
	return ConfigSnapshot{
		Kind:                  s.kind,
		Service:               s.service,
		ServiceLocality:       s.locality,
		ProxyID:               s.proxyID,
		Address:               s.address,
		Port:                  s.port,
		ServiceMeta:           s.meta,
		TaggedAddresses:       s.taggedAddresses,
		Proxy:                 s.proxyCfg,
		Datacenter:            config.source.Datacenter,
		Locality:              GatewayKey{Datacenter: config.source.Datacenter, Partition: s.proxyID.PartitionOrDefault()},
		ServerSNIFn:           config.serverSNIFn,
		IntentionDefaultAllow: config.intentionDefaultAllow,
	}
}

func (s *state) run(ctx context.Context, snap *ConfigSnapshot) {
	// Add a recover here so than any panics do not make their way up
	// into the server / agent.
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("unexpected panic while running proxycfg",
				"node", s.serviceInstance.proxyID.NodeName,
				"service", s.serviceInstance.proxyID.ServiceID,
				"message", r,
				"stacktrace", string(debug.Stack()))
		}
	}()
	s.unsafeRun(ctx, snap)
}

func (s *state) unsafeRun(ctx context.Context, snap *ConfigSnapshot) {
	// Closing the done channel signals that this entire state is no longer
	// going to be updated.
	defer close(s.doneCh)
	// Close the channel we return from Watch when we stop so consumers can stop
	// watching and clean up their goroutines. It's important we do this here and
	// not in Close since this routine sends on this chan and so might panic if it
	// gets closed from another goroutine.
	defer close(s.snapCh)

	// This turns out to be really fiddly/painful by just using time.Timer.C
	// directly in the code below since you can't detect when a timer is stopped
	// vs waiting in order to know to reset it. So just use a chan to send
	// ourselves messages.
	sendCh := make(chan struct{})
	var coalesceTimer *time.Timer

	scheduleUpdate := func() {
		// Wait for MAX(<rate limiter delay>, coalesceTimeout)
		delay := s.rateLimiter.Reserve().Delay()
		if delay < coalesceTimeout {
			delay = coalesceTimeout
		}
		coalesceTimer = time.AfterFunc(delay, func() {
			// This runs in another goroutine so we can't just do the send
			// directly here as access to snap is racy. Instead, signal the main
			// loop above.
			select {
			case sendCh <- struct{}{}:
			case <-ctx.Done():
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			return
		case u := <-s.ch:
			s.logger.Trace("Data source returned; handling snapshot update", "correlationID", u.CorrelationID)

			if IsTerminalError(u.Err) {
				s.logger.Error("Data source in an irrecoverable state; exiting", "error", u.Err, "correlationID", u.CorrelationID)
				s.Close(true)
				return
			}

			if err := s.handler.handleUpdate(ctx, u, snap); err != nil {
				s.logger.Error("Failed to handle update from watch",
					"id", u.CorrelationID, "error", err,
				)
				continue
			}

		case <-sendCh:
			// Allow the next change to trigger a send
			coalesceTimer = nil
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			snapCopy := snap.Clone()

			select {
			// Try to send
			case s.snapCh <- *snapCopy:
				s.logger.Trace("Delivered new snapshot to proxy config watchers")

				// Skip rest of loop - there is nothing to send since nothing changed on
				// this iteration
				continue

			// Avoid blocking if a snapshot is already buffered in snapCh as this can result in a deadlock.
			// See PR #9689 for more details.
			default:
				s.logger.Trace("Failed to deliver new snapshot to proxy config watchers")

				// Reset the timer to retry later. This is to ensure we attempt to redeliver the updated snapshot shortly.
				scheduleUpdate()

				// Do not reset coalesceTimer since we just queued a timer-based refresh
				continue
			}

		case replyCh := <-s.reqCh:
			s.logger.Trace("A proxy config snapshot was requested")

			if !snap.Valid() {
				// Not valid yet just respond with nil and move on to next task.
				replyCh <- nil

				s.logger.Trace("The proxy's config snapshot is not valid yet")
				continue
			}
			// Make a deep copy of snap so we don't mutate any of the embedded structs
			// etc on future updates.
			replyCh <- snap.Clone()

			// Skip rest of loop - there is nothing to send since nothing changed on
			// this iteration
			continue
		}

		// Check if snap is complete enough to be a valid config to deliver to a
		// proxy yet.
		if snap.Valid() {
			if coalesceTimer == nil {
				// Don't send it right away, set a short timer that will wait for updates
				// from any of the other cache values and deliver them all together.
				scheduleUpdate()
			}
		}
	}
}

// CurrentSnapshot synchronously returns the current ConfigSnapshot if there is
// one ready. If we don't have one yet because not all necessary parts have been
// returned (i.e. both roots and leaf cert), nil is returned.
func (s *state) CurrentSnapshot() *ConfigSnapshot {
	// Make a chan for the response to be sent on
	ch := make(chan *ConfigSnapshot, 1)

	select {
	case <-s.doneCh:
		return nil
	case s.reqCh <- ch:
	}

	// Wait for the response
	select {
	case <-s.doneCh:
		return nil
	case resp := <-ch:
		return resp
	}
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

	i := s.serviceInstance
	return ns.Kind != i.kind ||
		i.address != ns.Address ||
		i.port != ns.Port ||
		!reflect.DeepEqual(i.proxyCfg, proxyCfg) ||
		i.token != token
}

// hostnameEndpoints returns all CheckServiceNodes that have hostnames instead of IPs as the address.
// Envoy cannot resolve hostnames provided through EDS, so we exclusively use CDS for these clusters.
// If there is a mix of hostnames and addresses we exclusively use the hostnames, since clusters cannot discover
// services with both EDS and DNS.
func hostnameEndpoints(logger hclog.Logger, localKey GatewayKey, nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	var (
		hasIP       bool
		hasHostname bool
		resp        structs.CheckServiceNodes
	)

	for _, n := range nodes {
		_, addr, _ := n.BestAddress(!localKey.Matches(n.Node.Datacenter, n.Node.PartitionOrDefault()))
		if net.ParseIP(addr) != nil {
			hasIP = true
			continue
		}
		hasHostname = true
		resp = append(resp, n)
	}

	if hasHostname && hasIP {
		dc := nodes[0].Node.Datacenter
		sn := nodes[0].Service.CompoundServiceName()

		logger.Warn("service contains instances with mix of hostnames and IP addresses; only hostnames will be passed to Envoy",
			"dc", dc, "service", sn.String())
	}
	return resp
}

type gatewayWatchOpts struct {
	internalServiceDump InternalServiceDump
	notifyCh            chan UpdateEvent
	source              structs.QuerySource
	token               string
	key                 GatewayKey
	upstreamID          UpstreamID
}

func watchMeshGateway(ctx context.Context, opts gatewayWatchOpts) error {
	var correlationId string
	if opts.upstreamID.Name == "" {
		correlationId = fmt.Sprintf("mesh-gateway:%s", opts.key.String())
	} else {
		correlationId = fmt.Sprintf("mesh-gateway:%s:%s", opts.key.String(), opts.upstreamID.String())
	}

	return opts.internalServiceDump.Notify(ctx, &structs.ServiceDumpRequest{
		Datacenter:     opts.key.Datacenter,
		QueryOptions:   structs.QueryOptions{Token: opts.token},
		ServiceKind:    structs.ServiceKindMeshGateway,
		UseServiceKind: true,
		Source:         opts.source,
		EnterpriseMeta: *structs.DefaultEnterpriseMetaInPartition(opts.key.Partition),
	}, correlationId, opts.notifyCh)
}
