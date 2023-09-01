// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"github.com/hashicorp/consul/agent/proxycfg-sources/catalog"
	"github.com/hashicorp/consul/agent/xds/response"
	"github.com/hashicorp/consul/internal/mesh/proxy-snapshot"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/grpc-external/limiter"
	"github.com/hashicorp/consul/envoyextensions/xdscommon"

	envoy_cluster_v3 "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	envoy_core_v3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	envoy_endpoint_v3 "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	envoy_listener_v3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	envoy_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/config/rbac/v3"
	envoy_route_v3 "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	envoy_http_router_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/http/router/v3"
	envoy_http_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/http_connection_manager/v3"
	envoy_network_rbac_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/rbac/v3"
	envoy_tcp_proxy_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/filters/network/tcp_proxy/v3"
	envoy_tls_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	envoy_upstreams_v3 "github.com/envoyproxy/go-control-plane/envoy/extensions/upstreams/http/v3"
	envoy_discovery_v3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	envoy_type_v3 "github.com/envoyproxy/go-control-plane/envoy/type/v3"

	"github.com/armon/go-metrics"
	"github.com/mitchellh/copystructure"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
)

// NOTE: this file is a collection of test helper functions for testing xDS
// protocols.

func newTestSnapshot(
	t *testing.T,
	prevSnap *proxycfg.ConfigSnapshot,
	dbServiceProtocol string,
	nsFn func(ns *structs.NodeService),
	additionalEntries ...structs.ConfigEntry,
) *proxycfg.ConfigSnapshot {
	snap := proxycfg.TestConfigSnapshotDiscoveryChain(t, "default", false, nsFn, nil, additionalEntries...)
	snap.ConnectProxy.PreparedQueryEndpoints = map[proxycfg.UpstreamID]structs.CheckServiceNodes{
		UID("prepared_query:geo-cache"): proxycfg.TestPreparedQueryNodes(t, "geo-cache"),
	}
	if prevSnap != nil {
		snap.Roots = prevSnap.Roots
		snap.ConnectProxy.Leaf = prevSnap.ConnectProxy.Leaf
	}
	if dbServiceProtocol != "" {
		// Simulate ServiceManager injection of protocol
		snap.Proxy.Upstreams[0].Config["protocol"] = dbServiceProtocol
		snap.ConnectProxy.ConfigSnapshotUpstreams.UpstreamConfig = proxycfg.UpstreamsToMap(snap.Proxy.Upstreams)
	}
	return snap
}

// testManager is a mock of proxycfg.Manager that's simpler to control for
// testing. It also implements ConnectAuthz to allow control over authorization.
type testManager struct {
	sync.Mutex
	stateChans map[structs.ServiceID]chan proxysnapshot.ProxySnapshot
	drainChans map[structs.ServiceID]chan struct{}
	cancels    chan structs.ServiceID
}

func newTestManager(t *testing.T) *testManager {
	return &testManager{
		stateChans: map[structs.ServiceID]chan proxysnapshot.ProxySnapshot{},
		drainChans: map[structs.ServiceID]chan struct{}{},
		cancels:    make(chan structs.ServiceID, 10),
	}
}

// RegisterProxy simulates a proxy registration
func (m *testManager) RegisterProxy(t *testing.T, proxyID structs.ServiceID) {
	m.Lock()
	defer m.Unlock()
	m.stateChans[proxyID] = make(chan proxysnapshot.ProxySnapshot, 1)
	m.drainChans[proxyID] = make(chan struct{})
}

// Deliver simulates a proxy registration
func (m *testManager) DeliverConfig(t *testing.T, proxyID structs.ServiceID, cfg proxysnapshot.ProxySnapshot) {
	t.Helper()
	m.Lock()
	defer m.Unlock()
	select {
	case m.stateChans[proxyID] <- cfg:
	case <-time.After(10 * time.Millisecond):
		t.Fatalf("took too long to deliver config")
	}
}

// DrainStreams drains any open streams for the given proxyID. If there aren't
// any open streams, it'll create a marker so that future attempts to watch the
// given proxyID will return limiter.ErrCapacityReached.
func (m *testManager) DrainStreams(proxyID structs.ServiceID) {
	m.Lock()
	defer m.Unlock()

	ch, ok := m.drainChans[proxyID]
	if !ok {
		ch = make(chan struct{})
		m.drainChans[proxyID] = ch
	}
	close(ch)
}

// Watch implements ConfigManager
func (m *testManager) Watch(id *pbresource.ID, _ string, _ string) (<-chan proxysnapshot.ProxySnapshot,
	limiter.SessionTerminatedChan, proxysnapshot.CancelFunc, error) {
	// Create service ID
	proxyID := structs.NewServiceID(id.Name, catalog.GetEnterpriseMetaFromResourceID(id))
	m.Lock()
	defer m.Unlock()

	// If the drain chan has already been closed, return limiter.ErrCapacityReached.
	drainCh := m.drainChans[proxyID]
	select {
	case <-drainCh:
		return nil, nil, nil, limiter.ErrCapacityReached
	default:
	}

	// ch might be nil but then it will just block forever
	return m.stateChans[proxyID], drainCh, func() {
		m.cancels <- proxyID
	}, nil
}

// AssertWatchCancelled checks that the most recent call to a Watch cancel func
// was from the specified proxyID and that one is made in a short time. This
// probably won't work if you are running multiple Watches in parallel on
// multiple proxyIDS due to timing/ordering issues but I don't think we need to
// do that.
func (m *testManager) AssertWatchCancelled(t *testing.T, proxyID structs.ServiceID) {
	t.Helper()
	select {
	case got := <-m.cancels:
		require.Equal(t, proxyID, got)
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("timed out waiting for Watch cancel for %s", proxyID)
	}
}

type testServerScenario struct {
	server *Server
	mgr    *testManager
	envoy  *TestEnvoy
	sink   *metrics.InmemSink
	errCh  <-chan error
}

func newTestServerDeltaScenario(
	t *testing.T,
	resolveTokenSecret ACLResolverFunc,
	proxyID string,
	token string,
	authCheckFrequency time.Duration,
) *testServerScenario {
	mgr := newTestManager(t)
	envoy := NewTestEnvoy(t, proxyID, token)

	sink := metrics.NewInmemSink(1*time.Minute, 1*time.Minute)
	cfg := metrics.DefaultConfig("consul.xds.test")
	cfg.EnableHostname = false
	cfg.EnableRuntimeMetrics = false
	metrics.NewGlobal(cfg, sink)

	t.Cleanup(func() {
		envoy.Close()
		sink := &metrics.BlackholeSink{}
		metrics.NewGlobal(cfg, sink)
	})

	s := NewServer(
		"node-123",
		testutil.Logger(t),
		mgr,
		resolveTokenSecret,
		nil, /*cfgFetcher ConfigFetcher*/
	)
	if authCheckFrequency > 0 {
		s.AuthCheckFrequency = authCheckFrequency
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.DeltaAggregatedResources(envoy.deltaStream)
	}()

	return &testServerScenario{
		server: s,
		mgr:    mgr,
		envoy:  envoy,
		sink:   sink,
		errCh:  errCh,
	}
}

func protoToSortedJSON(t *testing.T, pb proto.Message) string {
	dup, err := copystructure.Copy(pb)
	require.NoError(t, err)
	pb = dup.(proto.Message)

	switch x := pb.(type) {
	case *envoy_discovery_v3.DeltaDiscoveryResponse:
		sort.Slice(x.Resources, func(i, j int) bool {
			return x.Resources[i].Name < x.Resources[j].Name
		})
		sort.Strings(x.RemovedResources)
	}

	return protoToJSON(t, pb)
}

func xdsNewEndpoint(ip string, port int) *envoy_endpoint_v3.LbEndpoint {
	return &envoy_endpoint_v3.LbEndpoint{
		HostIdentifier: &envoy_endpoint_v3.LbEndpoint_Endpoint{
			Endpoint: &envoy_endpoint_v3.Endpoint{
				Address: response.MakeAddress(ip, port),
			},
		},
	}
}

func xdsNewEndpointWithHealth(ip string, port int, health envoy_core_v3.HealthStatus, weight int) *envoy_endpoint_v3.LbEndpoint {
	ep := xdsNewEndpoint(ip, port)
	ep.HealthStatus = health
	ep.LoadBalancingWeight = response.MakeUint32Value(weight)
	return ep
}

func xdsNewADSConfig() *envoy_core_v3.ConfigSource {
	return &envoy_core_v3.ConfigSource{
		ResourceApiVersion: envoy_core_v3.ApiVersion_V3,
		ConfigSourceSpecifier: &envoy_core_v3.ConfigSource_Ads{
			Ads: &envoy_core_v3.AggregatedConfigSource{},
		},
	}
}

func xdsNewPublicTransportSocket(
	t *testing.T,
	snap *proxycfg.ConfigSnapshot,
) *envoy_core_v3.TransportSocket {
	return xdsNewTransportSocket(t, snap, true, true, "")
}

func xdsNewUpstreamTransportSocket(
	t *testing.T,
	snap *proxycfg.ConfigSnapshot,
	sni string,
	spiffeID ...string,
) *envoy_core_v3.TransportSocket {
	return xdsNewTransportSocket(t, snap, false, false, sni, spiffeID...)
}

func xdsNewTransportSocket(
	t *testing.T,
	snap *proxycfg.ConfigSnapshot,
	downstream bool,
	requireClientCert bool,
	sni string,
	spiffeID ...string,
) *envoy_core_v3.TransportSocket {
	// Assume just one root for now, can get fancier later if needed.
	caPEM := snap.Roots.Roots[0].RootCert

	commonTLSContext := &envoy_tls_v3.CommonTlsContext{
		TlsParams: &envoy_tls_v3.TlsParameters{},
		TlsCertificates: []*envoy_tls_v3.TlsCertificate{{
			CertificateChain: xdsNewInlineString(snap.Leaf().CertPEM),
			PrivateKey:       xdsNewInlineString(snap.Leaf().PrivateKeyPEM),
		}},
		ValidationContextType: &envoy_tls_v3.CommonTlsContext_ValidationContext{
			ValidationContext: &envoy_tls_v3.CertificateValidationContext{
				TrustedCa: xdsNewInlineString(caPEM),
			},
		},
	}
	if len(spiffeID) > 0 {
		require.NoError(t, injectSANMatcher(commonTLSContext, spiffeID...))
	}

	var tlsContext proto.Message
	if downstream {
		var requireClientCertPB *wrapperspb.BoolValue
		if requireClientCert {
			requireClientCertPB = response.MakeBoolValue(true)
		}

		tlsContext = &envoy_tls_v3.DownstreamTlsContext{
			CommonTlsContext:         commonTLSContext,
			RequireClientCertificate: requireClientCertPB,
		}
	} else {
		tlsContext = &envoy_tls_v3.UpstreamTlsContext{
			CommonTlsContext: commonTLSContext,
			Sni:              sni,
		}
	}

	any, err := anypb.New(tlsContext)
	require.NoError(t, err)

	return &envoy_core_v3.TransportSocket{
		Name: "tls",
		ConfigType: &envoy_core_v3.TransportSocket_TypedConfig{
			TypedConfig: any,
		},
	}
}

func xdsNewInlineString(s string) *envoy_core_v3.DataSource {
	return &envoy_core_v3.DataSource{
		Specifier: &envoy_core_v3.DataSource_InlineString{
			InlineString: s,
		},
	}
}

func xdsNewFilter(t *testing.T, name string, cfg proto.Message) *envoy_listener_v3.Filter {
	f, err := makeFilter(name, cfg)
	require.NoError(t, err)
	return f
}

func xdsNewListenerFilter(t *testing.T, name string, cfg proto.Message) *envoy_listener_v3.ListenerFilter {
	f, err := makeEnvoyListenerFilter(name, cfg)
	require.NoError(t, err)
	return f
}

func xdsNewHttpFilter(t *testing.T, name string, cfg proto.Message) *envoy_http_v3.HttpFilter {
	f, err := makeEnvoyHTTPFilter(name, cfg)
	require.NoError(t, err)
	return f
}

func mustHashResource(t *testing.T, res proto.Message) string {
	v, err := hashResource(res)
	require.NoError(t, err)
	return v
}

func makeTestResources(t *testing.T, resources ...interface{}) []*envoy_discovery_v3.Resource {
	var ret []*envoy_discovery_v3.Resource
	for _, res := range resources {
		ret = append(ret, makeTestResource(t, res))
	}
	return ret
}

func makeTestResource(t *testing.T, raw interface{}) *envoy_discovery_v3.Resource {
	switch res := raw.(type) {
	case string:
		return &envoy_discovery_v3.Resource{
			Name: res,
		}
	case proto.Message:

		any, err := anypb.New(res)
		require.NoError(t, err)

		return &envoy_discovery_v3.Resource{
			Name:     xdscommon.GetResourceName(res),
			Version:  mustHashResource(t, res),
			Resource: any,
		}
	default:
		t.Fatalf("unexpected type: %T", res)
		return nil // not possible
	}
}

func makeTestCluster(t *testing.T, snap *proxycfg.ConfigSnapshot, fixtureName string) *envoy_cluster_v3.Cluster {
	var (
		dbSNI = "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul"
		dbURI = connect.SpiffeIDService{
			Host:       "11111111-2222-3333-4444-555555555555.consul",
			Namespace:  "default",
			Datacenter: "dc1",
			Service:    "db",
		}.URI().String()

		geocacheSNI  = "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul"
		geocacheURIs = []string{
			connect.SpiffeIDService{
				Host:       "11111111-2222-3333-4444-555555555555.consul",
				Namespace:  "default",
				Datacenter: "dc1",
				Service:    "geo-cache-target",
			}.URI().String(),
			connect.SpiffeIDService{
				Host:       "11111111-2222-3333-4444-555555555555.consul",
				Namespace:  "default",
				Datacenter: "dc2",
				Service:    "geo-cache-target",
			}.URI().String(),
		}
	)

	switch fixtureName {
	case "tcp:local_app":
		return &envoy_cluster_v3.Cluster{
			Name: "local_app",
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_STATIC,
			},
			ConnectTimeout: durationpb.New(5 * time.Second),
			LoadAssignment: &envoy_endpoint_v3.ClusterLoadAssignment{
				ClusterName: "local_app",
				Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						xdsNewEndpoint("127.0.0.1", 8080),
					},
				}},
			},
		}
	case "tcp:db":
		return &envoy_cluster_v3.Cluster{
			Name: dbSNI,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: xdsNewADSConfig(),
			},
			CircuitBreakers:  &envoy_cluster_v3.CircuitBreakers{},
			OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			AltStatName:      dbSNI,
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{Value: 0},
			},
			ConnectTimeout:  durationpb.New(5 * time.Second),
			TransportSocket: xdsNewUpstreamTransportSocket(t, snap, dbSNI, dbURI),
		}
	case "tcp:db:timeout":
		return &envoy_cluster_v3.Cluster{
			Name: dbSNI,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: xdsNewADSConfig(),
			},
			CircuitBreakers:  &envoy_cluster_v3.CircuitBreakers{},
			OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			AltStatName:      dbSNI,
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{Value: 0},
			},
			ConnectTimeout:  durationpb.New(1337 * time.Second),
			TransportSocket: xdsNewUpstreamTransportSocket(t, snap, dbSNI, dbURI),
		}
	case "http2:db":
		c := &envoy_cluster_v3.Cluster{
			Name: dbSNI,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: xdsNewADSConfig(),
			},
			CircuitBreakers:  &envoy_cluster_v3.CircuitBreakers{},
			OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			AltStatName:      dbSNI,
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{Value: 0},
			},
			ConnectTimeout:  durationpb.New(5 * time.Second),
			TransportSocket: xdsNewUpstreamTransportSocket(t, snap, dbSNI, dbURI),
		}
		typedExtensionProtocolOptions := &envoy_upstreams_v3.HttpProtocolOptions{
			UpstreamProtocolOptions: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig_{
				ExplicitHttpConfig: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig{
					ProtocolConfig: &envoy_upstreams_v3.HttpProtocolOptions_ExplicitHttpConfig_Http2ProtocolOptions{
						Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
					},
				},
			},
		}
		typedExtensionProtocolOptionsEncoded, err := anypb.New(typedExtensionProtocolOptions)
		require.NoError(t, err)
		c.TypedExtensionProtocolOptions = map[string]*anypb.Any{
			"envoy.extensions.upstreams.http.v3.HttpProtocolOptions": typedExtensionProtocolOptionsEncoded,
		}
		return c
	case "http:db":
		return &envoy_cluster_v3.Cluster{
			Name: dbSNI,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: xdsNewADSConfig(),
			},
			CircuitBreakers:  &envoy_cluster_v3.CircuitBreakers{},
			OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			AltStatName:      dbSNI,
			CommonLbConfig: &envoy_cluster_v3.Cluster_CommonLbConfig{
				HealthyPanicThreshold: &envoy_type_v3.Percent{Value: 0},
			},
			ConnectTimeout:  durationpb.New(5 * time.Second),
			TransportSocket: xdsNewUpstreamTransportSocket(t, snap, dbSNI, dbURI),
			// HttpProtocolOptions: &envoy_core_v3.Http1ProtocolOptions{},
		}
	case "tcp:geo-cache":
		return &envoy_cluster_v3.Cluster{
			Name: geocacheSNI,
			ClusterDiscoveryType: &envoy_cluster_v3.Cluster_Type{
				Type: envoy_cluster_v3.Cluster_EDS,
			},
			EdsClusterConfig: &envoy_cluster_v3.Cluster_EdsClusterConfig{
				EdsConfig: xdsNewADSConfig(),
			},
			CircuitBreakers:  &envoy_cluster_v3.CircuitBreakers{},
			OutlierDetection: &envoy_cluster_v3.OutlierDetection{},
			ConnectTimeout:   durationpb.New(5 * time.Second),
			TransportSocket:  xdsNewUpstreamTransportSocket(t, snap, geocacheSNI, geocacheURIs...),
		}
	default:
		t.Fatalf("unexpected fixture name: %s", fixtureName)
		return nil
	}
}

func makeTestEndpoints(t *testing.T, _ *proxycfg.ConfigSnapshot, fixtureName string) *envoy_endpoint_v3.ClusterLoadAssignment {
	switch fixtureName {
	case "tcp:db":
		return &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						xdsNewEndpointWithHealth("10.10.1.1", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
						xdsNewEndpointWithHealth("10.10.1.2", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
					},
				},
			},
		}
	case "tcp:db[0]":
		return &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						xdsNewEndpointWithHealth("10.10.1.1", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
					},
				},
			},
		}
	case "http2:db", "http:db":
		return &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						xdsNewEndpointWithHealth("10.10.1.1", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
						xdsNewEndpointWithHealth("10.10.1.2", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
					},
				},
			},
		}
	case "tcp:geo-cache":
		return &envoy_endpoint_v3.ClusterLoadAssignment{
			ClusterName: "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
			Endpoints: []*envoy_endpoint_v3.LocalityLbEndpoints{
				{
					LbEndpoints: []*envoy_endpoint_v3.LbEndpoint{
						xdsNewEndpointWithHealth("10.10.1.1", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
						xdsNewEndpointWithHealth("10.20.1.2", 8080, envoy_core_v3.HealthStatus_HEALTHY, 1),
					},
				},
			},
		}
	default:
		t.Fatalf("unexpected fixture name: %s", fixtureName)
		return nil
	}
}

func makeTestListener(t *testing.T, snap *proxycfg.ConfigSnapshot, fixtureName string) *envoy_listener_v3.Listener {
	switch fixtureName {
	case "tcp:bad_public_listener":
		return &envoy_listener_v3.Listener{
			// Envoy can't bind to port 1
			Name:             "public_listener:0.0.0.0:1",
			Address:          response.MakeAddress("0.0.0.0", 1),
			TrafficDirection: envoy_core_v3.TrafficDirection_INBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					TransportSocket: xdsNewPublicTransportSocket(t, snap),
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.rbac", &envoy_network_rbac_v3.RBAC{
							Rules:      &envoy_rbac_v3.RBAC{},
							StatPrefix: "connect_authz",
						}),
						xdsNewFilter(t, "envoy.filters.network.tcp_proxy", &envoy_tcp_proxy_v3.TcpProxy{
							ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{
								Cluster: "local_app",
							},
							StatPrefix: "public_listener",
						}),
					},
				},
			},
		}
	case "tcp:public_listener":
		return &envoy_listener_v3.Listener{
			Name:             "public_listener:0.0.0.0:9999",
			Address:          response.MakeAddress("0.0.0.0", 9999),
			TrafficDirection: envoy_core_v3.TrafficDirection_INBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					TransportSocket: xdsNewPublicTransportSocket(t, snap),
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.rbac", &envoy_network_rbac_v3.RBAC{
							Rules:      &envoy_rbac_v3.RBAC{},
							StatPrefix: "connect_authz",
						}),
						xdsNewFilter(t, "envoy.filters.network.tcp_proxy", &envoy_tcp_proxy_v3.TcpProxy{
							ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{
								Cluster: "local_app",
							},
							StatPrefix: "public_listener",
						}),
					},
				},
			},
		}
	case "tcp:db":
		return &envoy_listener_v3.Listener{
			Name:             "db:127.0.0.1:9191",
			Address:          response.MakeAddress("127.0.0.1", 9191),
			TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.tcp_proxy", &envoy_tcp_proxy_v3.TcpProxy{
							ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{
								Cluster: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
							},
							StatPrefix: "upstream.db.default.default.dc1",
						}),
					},
				},
			},
		}
	case "http2:db":
		return &envoy_listener_v3.Listener{
			Name:             "db:127.0.0.1:9191",
			Address:          response.MakeAddress("127.0.0.1", 9191),
			TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.http_connection_manager", &envoy_http_v3.HttpConnectionManager{
							HttpFilters: []*envoy_http_v3.HttpFilter{
								xdsNewHttpFilter(t, "envoy.filters.http.router", &envoy_http_router_v3.Router{}),
							},
							RouteSpecifier: &envoy_http_v3.HttpConnectionManager_RouteConfig{
								RouteConfig: makeTestRoute(t, "http2:db:inline"),
							},
							StatPrefix: "upstream.db.default.default.dc1",
							Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
								RandomSampling: &envoy_type_v3.Percent{Value: 0},
							},
							UpgradeConfigs: []*envoy_http_v3.HttpConnectionManager_UpgradeConfig{
								{UpgradeType: "websocket"},
							},
							Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
						}),
					},
				},
			},
		}
	case "http2:db:rds":
		return &envoy_listener_v3.Listener{
			Name:             "db:127.0.0.1:9191",
			Address:          response.MakeAddress("127.0.0.1", 9191),
			TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.http_connection_manager", &envoy_http_v3.HttpConnectionManager{
							HttpFilters: []*envoy_http_v3.HttpFilter{
								xdsNewHttpFilter(t, "envoy.filters.http.router", &envoy_http_router_v3.Router{}),
							},
							RouteSpecifier: &envoy_http_v3.HttpConnectionManager_Rds{
								Rds: &envoy_http_v3.Rds{
									RouteConfigName: "db",
									ConfigSource:    xdsNewADSConfig(),
								},
							},
							StatPrefix: "upstream.db.default.default.dc1",
							Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
								RandomSampling: &envoy_type_v3.Percent{Value: 0},
							},
							UpgradeConfigs: []*envoy_http_v3.HttpConnectionManager_UpgradeConfig{
								{UpgradeType: "websocket"},
							},
							Http2ProtocolOptions: &envoy_core_v3.Http2ProtocolOptions{},
						}),
					},
				},
			},
		}
	case "http:db:rds":
		return &envoy_listener_v3.Listener{
			Name:             "db:127.0.0.1:9191",
			Address:          response.MakeAddress("127.0.0.1", 9191),
			TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.http_connection_manager", &envoy_http_v3.HttpConnectionManager{
							HttpFilters: []*envoy_http_v3.HttpFilter{
								xdsNewHttpFilter(t, "envoy.filters.http.router", &envoy_http_router_v3.Router{}),
							},
							RouteSpecifier: &envoy_http_v3.HttpConnectionManager_Rds{
								Rds: &envoy_http_v3.Rds{
									RouteConfigName: "db",
									ConfigSource:    xdsNewADSConfig(),
								},
							},
							StatPrefix: "upstream.db.default.default.dc1",
							Tracing: &envoy_http_v3.HttpConnectionManager_Tracing{
								RandomSampling: &envoy_type_v3.Percent{Value: 0},
							},
							UpgradeConfigs: []*envoy_http_v3.HttpConnectionManager_UpgradeConfig{
								{UpgradeType: "websocket"},
							},
							// HttpProtocolOptions: &envoy_core_v3.Http1ProtocolOptions{},
						}),
					},
				},
			},
		}
	case "tcp:geo-cache":
		return &envoy_listener_v3.Listener{
			Name:             "prepared_query:geo-cache:127.10.10.10:8181",
			Address:          response.MakeAddress("127.10.10.10", 8181),
			TrafficDirection: envoy_core_v3.TrafficDirection_OUTBOUND,
			FilterChains: []*envoy_listener_v3.FilterChain{
				{
					Filters: []*envoy_listener_v3.Filter{
						xdsNewFilter(t, "envoy.filters.network.tcp_proxy", &envoy_tcp_proxy_v3.TcpProxy{
							ClusterSpecifier: &envoy_tcp_proxy_v3.TcpProxy_Cluster{
								Cluster: "geo-cache.default.dc1.query.11111111-2222-3333-4444-555555555555.consul",
							},
							StatPrefix: "upstream.prepared_query_geo-cache",
						}),
					},
				},
			},
		}
	default:
		t.Fatalf("unexpected fixture name: %s", fixtureName)
		return nil
	}
}

func makeTestRoute(t *testing.T, fixtureName string) *envoy_route_v3.RouteConfiguration {
	switch fixtureName {
	case "http2:db", "http:db":
		return &envoy_route_v3.RouteConfiguration{
			Name:             "db",
			ValidateClusters: response.MakeBoolValue(true),
			VirtualHosts: []*envoy_route_v3.VirtualHost{
				{
					Name:    "db",
					Domains: []string{"*"},
					Routes: []*envoy_route_v3.Route{
						{
							Match: &envoy_route_v3.RouteMatch{
								PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
									Prefix: "/",
								},
							},
							Action: &envoy_route_v3.Route_Route{
								Route: &envoy_route_v3.RouteAction{
									ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{
										Cluster: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
									},
								},
							},
						},
					},
				},
			},
		}
	case "http2:db:inline":
		return &envoy_route_v3.RouteConfiguration{
			Name: "db",
			VirtualHosts: []*envoy_route_v3.VirtualHost{
				{
					Name:    "db.default.default.dc1",
					Domains: []string{"*"},
					Routes: []*envoy_route_v3.Route{
						{
							Match: &envoy_route_v3.RouteMatch{
								PathSpecifier: &envoy_route_v3.RouteMatch_Prefix{
									Prefix: "/",
								},
							},
							Action: &envoy_route_v3.Route_Route{
								Route: &envoy_route_v3.RouteAction{
									ClusterSpecifier: &envoy_route_v3.RouteAction_Cluster{
										Cluster: "db.default.dc1.internal.11111111-2222-3333-4444-555555555555.consul",
									},
								},
							},
						},
					},
				},
			},
		}
	default:
		t.Fatalf("unexpected fixture name: %s", fixtureName)
		return nil
	}
}

func requireProtocolVersionGauge(
	t *testing.T,
	scenario *testServerScenario,
	xdsVersion string,
	expected int,
) {
	data := scenario.sink.Data()
	require.Len(t, data, 1)

	item := data[0]
	require.Len(t, item.Gauges, 2)

	val, ok := item.Gauges["consul.xds.test.xds.server.streams;version="+xdsVersion]
	require.True(t, ok)

	require.Equal(t, "consul.xds.test.xds.server.streams", val.Name)
	require.Equal(t, expected, int(val.Value))
	require.Equal(t, []metrics.Label{{Name: "version", Value: xdsVersion}}, val.Labels)
}
