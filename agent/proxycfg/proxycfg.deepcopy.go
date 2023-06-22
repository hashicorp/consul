// generated by deep-copy -pointer-receiver -o ./proxycfg.deepcopy.go -type ConfigSnapshot -type ConfigSnapshotUpstreams -type PeerServersValue -type PeeringServiceValue -type configSnapshotAPIGateway -type configSnapshotConnectProxy -type configSnapshotIngressGateway -type configSnapshotMeshGateway -type configSnapshotTerminatingGateway ./; DO NOT EDIT.

package proxycfg

import (
	"context"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
	"github.com/hashicorp/consul/types"
	"time"
)

// DeepCopy generates a deep copy of *ConfigSnapshot
func (o *ConfigSnapshot) DeepCopy() *ConfigSnapshot {
	var cp ConfigSnapshot = *o
	if o.ServiceLocality != nil {
		cp.ServiceLocality = new(structs.Locality)
		*cp.ServiceLocality = *o.ServiceLocality
	}
	if o.ServiceMeta != nil {
		cp.ServiceMeta = make(map[string]string, len(o.ServiceMeta))
		for k2, v2 := range o.ServiceMeta {
			cp.ServiceMeta[k2] = v2
		}
	}
	if o.TaggedAddresses != nil {
		cp.TaggedAddresses = make(map[string]structs.ServiceAddress, len(o.TaggedAddresses))
		for k2, v2 := range o.TaggedAddresses {
			cp.TaggedAddresses[k2] = v2
		}
	}
	{
		retV := o.Proxy.DeepCopy()
		cp.Proxy = *retV
	}
	if o.JWTProviders != nil {
		cp.JWTProviders = make(map[string]*structs.JWTProviderConfigEntry, len(o.JWTProviders))
		for k2, v2 := range o.JWTProviders {
			var cp_JWTProviders_v2 *structs.JWTProviderConfigEntry
			if v2 != nil {
				cp_JWTProviders_v2 = new(structs.JWTProviderConfigEntry)
				*cp_JWTProviders_v2 = *v2
				if v2.JSONWebKeySet != nil {
					cp_JWTProviders_v2.JSONWebKeySet = new(structs.JSONWebKeySet)
					*cp_JWTProviders_v2.JSONWebKeySet = *v2.JSONWebKeySet
					if v2.JSONWebKeySet.Local != nil {
						cp_JWTProviders_v2.JSONWebKeySet.Local = new(structs.LocalJWKS)
						*cp_JWTProviders_v2.JSONWebKeySet.Local = *v2.JSONWebKeySet.Local
					}
					if v2.JSONWebKeySet.Remote != nil {
						cp_JWTProviders_v2.JSONWebKeySet.Remote = new(structs.RemoteJWKS)
						*cp_JWTProviders_v2.JSONWebKeySet.Remote = *v2.JSONWebKeySet.Remote
						if v2.JSONWebKeySet.Remote.RetryPolicy != nil {
							cp_JWTProviders_v2.JSONWebKeySet.Remote.RetryPolicy = new(structs.JWKSRetryPolicy)
							*cp_JWTProviders_v2.JSONWebKeySet.Remote.RetryPolicy = *v2.JSONWebKeySet.Remote.RetryPolicy
							if v2.JSONWebKeySet.Remote.RetryPolicy.RetryPolicyBackOff != nil {
								cp_JWTProviders_v2.JSONWebKeySet.Remote.RetryPolicy.RetryPolicyBackOff = new(structs.RetryPolicyBackOff)
								*cp_JWTProviders_v2.JSONWebKeySet.Remote.RetryPolicy.RetryPolicyBackOff = *v2.JSONWebKeySet.Remote.RetryPolicy.RetryPolicyBackOff
							}
						}
					}
				}
				if v2.Audiences != nil {
					cp_JWTProviders_v2.Audiences = make([]string, len(v2.Audiences))
					copy(cp_JWTProviders_v2.Audiences, v2.Audiences)
				}
				if v2.Locations != nil {
					cp_JWTProviders_v2.Locations = make([]*structs.JWTLocation, len(v2.Locations))
					copy(cp_JWTProviders_v2.Locations, v2.Locations)
					for i5 := range v2.Locations {
						if v2.Locations[i5] != nil {
							cp_JWTProviders_v2.Locations[i5] = new(structs.JWTLocation)
							*cp_JWTProviders_v2.Locations[i5] = *v2.Locations[i5]
							if v2.Locations[i5].Header != nil {
								cp_JWTProviders_v2.Locations[i5].Header = new(structs.JWTLocationHeader)
								*cp_JWTProviders_v2.Locations[i5].Header = *v2.Locations[i5].Header
							}
							if v2.Locations[i5].QueryParam != nil {
								cp_JWTProviders_v2.Locations[i5].QueryParam = new(structs.JWTLocationQueryParam)
								*cp_JWTProviders_v2.Locations[i5].QueryParam = *v2.Locations[i5].QueryParam
							}
							if v2.Locations[i5].Cookie != nil {
								cp_JWTProviders_v2.Locations[i5].Cookie = new(structs.JWTLocationCookie)
								*cp_JWTProviders_v2.Locations[i5].Cookie = *v2.Locations[i5].Cookie
							}
						}
					}
				}
				if v2.Forwarding != nil {
					cp_JWTProviders_v2.Forwarding = new(structs.JWTForwardingConfig)
					*cp_JWTProviders_v2.Forwarding = *v2.Forwarding
				}
				if v2.CacheConfig != nil {
					cp_JWTProviders_v2.CacheConfig = new(structs.JWTCacheConfig)
					*cp_JWTProviders_v2.CacheConfig = *v2.CacheConfig
				}
				if v2.Meta != nil {
					cp_JWTProviders_v2.Meta = make(map[string]string, len(v2.Meta))
					for k5, v5 := range v2.Meta {
						cp_JWTProviders_v2.Meta[k5] = v5
					}
				}
			}
			cp.JWTProviders[k2] = cp_JWTProviders_v2
		}
	}
	if o.Roots != nil {
		cp.Roots = o.Roots.DeepCopy()
	}
	{
		retV := o.ConnectProxy.DeepCopy()
		cp.ConnectProxy = *retV
	}
	{
		retV := o.TerminatingGateway.DeepCopy()
		cp.TerminatingGateway = *retV
	}
	{
		retV := o.MeshGateway.DeepCopy()
		cp.MeshGateway = *retV
	}
	{
		retV := o.IngressGateway.DeepCopy()
		cp.IngressGateway = *retV
	}
	{
		retV := o.APIGateway.DeepCopy()
		cp.APIGateway = *retV
	}
	return &cp
}

// DeepCopy generates a deep copy of *ConfigSnapshotUpstreams
func (o *ConfigSnapshotUpstreams) DeepCopy() *ConfigSnapshotUpstreams {
	var cp ConfigSnapshotUpstreams = *o
	if o.Leaf != nil {
		cp.Leaf = new(structs.IssuedCert)
		*cp.Leaf = *o.Leaf
	}
	if o.MeshConfig != nil {
		cp.MeshConfig = o.MeshConfig.DeepCopy()
	}
	if o.DiscoveryChain != nil {
		cp.DiscoveryChain = make(map[UpstreamID]*structs.CompiledDiscoveryChain, len(o.DiscoveryChain))
		for k2, v2 := range o.DiscoveryChain {
			var cp_DiscoveryChain_v2 *structs.CompiledDiscoveryChain
			if v2 != nil {
				cp_DiscoveryChain_v2 = v2.DeepCopy()
			}
			cp.DiscoveryChain[k2] = cp_DiscoveryChain_v2
		}
	}
	if o.WatchedDiscoveryChains != nil {
		cp.WatchedDiscoveryChains = make(map[UpstreamID]context.CancelFunc, len(o.WatchedDiscoveryChains))
		for k2, v2 := range o.WatchedDiscoveryChains {
			cp.WatchedDiscoveryChains[k2] = v2
		}
	}
	if o.WatchedUpstreams != nil {
		cp.WatchedUpstreams = make(map[UpstreamID]map[string]context.CancelFunc, len(o.WatchedUpstreams))
		for k2, v2 := range o.WatchedUpstreams {
			var cp_WatchedUpstreams_v2 map[string]context.CancelFunc
			if v2 != nil {
				cp_WatchedUpstreams_v2 = make(map[string]context.CancelFunc, len(v2))
				for k3, v3 := range v2 {
					cp_WatchedUpstreams_v2[k3] = v3
				}
			}
			cp.WatchedUpstreams[k2] = cp_WatchedUpstreams_v2
		}
	}
	if o.WatchedUpstreamEndpoints != nil {
		cp.WatchedUpstreamEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes, len(o.WatchedUpstreamEndpoints))
		for k2, v2 := range o.WatchedUpstreamEndpoints {
			var cp_WatchedUpstreamEndpoints_v2 map[string]structs.CheckServiceNodes
			if v2 != nil {
				cp_WatchedUpstreamEndpoints_v2 = make(map[string]structs.CheckServiceNodes, len(v2))
				for k3, v3 := range v2 {
					var cp_WatchedUpstreamEndpoints_v2_v3 structs.CheckServiceNodes
					cp_WatchedUpstreamEndpoints_v2_v3 = v3.DeepCopy()
					cp_WatchedUpstreamEndpoints_v2[k3] = cp_WatchedUpstreamEndpoints_v2_v3
				}
			}
			cp.WatchedUpstreamEndpoints[k2] = cp_WatchedUpstreamEndpoints_v2
		}
	}
	cp.UpstreamPeerTrustBundles = o.UpstreamPeerTrustBundles.DeepCopy()
	if o.WatchedGateways != nil {
		cp.WatchedGateways = make(map[UpstreamID]map[string]context.CancelFunc, len(o.WatchedGateways))
		for k2, v2 := range o.WatchedGateways {
			var cp_WatchedGateways_v2 map[string]context.CancelFunc
			if v2 != nil {
				cp_WatchedGateways_v2 = make(map[string]context.CancelFunc, len(v2))
				for k3, v3 := range v2 {
					cp_WatchedGateways_v2[k3] = v3
				}
			}
			cp.WatchedGateways[k2] = cp_WatchedGateways_v2
		}
	}
	if o.WatchedGatewayEndpoints != nil {
		cp.WatchedGatewayEndpoints = make(map[UpstreamID]map[string]structs.CheckServiceNodes, len(o.WatchedGatewayEndpoints))
		for k2, v2 := range o.WatchedGatewayEndpoints {
			var cp_WatchedGatewayEndpoints_v2 map[string]structs.CheckServiceNodes
			if v2 != nil {
				cp_WatchedGatewayEndpoints_v2 = make(map[string]structs.CheckServiceNodes, len(v2))
				for k3, v3 := range v2 {
					var cp_WatchedGatewayEndpoints_v2_v3 structs.CheckServiceNodes
					cp_WatchedGatewayEndpoints_v2_v3 = v3.DeepCopy()
					cp_WatchedGatewayEndpoints_v2[k3] = cp_WatchedGatewayEndpoints_v2_v3
				}
			}
			cp.WatchedGatewayEndpoints[k2] = cp_WatchedGatewayEndpoints_v2
		}
	}
	cp.WatchedLocalGWEndpoints = o.WatchedLocalGWEndpoints.DeepCopy()
	if o.UpstreamConfig != nil {
		cp.UpstreamConfig = make(map[UpstreamID]*structs.Upstream, len(o.UpstreamConfig))
		for k2, v2 := range o.UpstreamConfig {
			var cp_UpstreamConfig_v2 *structs.Upstream
			if v2 != nil {
				cp_UpstreamConfig_v2 = v2.DeepCopy()
			}
			cp.UpstreamConfig[k2] = cp_UpstreamConfig_v2
		}
	}
	if o.PassthroughUpstreams != nil {
		cp.PassthroughUpstreams = make(map[UpstreamID]map[string]map[string]struct{}, len(o.PassthroughUpstreams))
		for k2, v2 := range o.PassthroughUpstreams {
			var cp_PassthroughUpstreams_v2 map[string]map[string]struct{}
			if v2 != nil {
				cp_PassthroughUpstreams_v2 = make(map[string]map[string]struct{}, len(v2))
				for k3, v3 := range v2 {
					var cp_PassthroughUpstreams_v2_v3 map[string]struct{}
					if v3 != nil {
						cp_PassthroughUpstreams_v2_v3 = make(map[string]struct{}, len(v3))
						for k4, v4 := range v3 {
							cp_PassthroughUpstreams_v2_v3[k4] = v4
						}
					}
					cp_PassthroughUpstreams_v2[k3] = cp_PassthroughUpstreams_v2_v3
				}
			}
			cp.PassthroughUpstreams[k2] = cp_PassthroughUpstreams_v2
		}
	}
	if o.PassthroughIndices != nil {
		cp.PassthroughIndices = make(map[string]indexedTarget, len(o.PassthroughIndices))
		for k2, v2 := range o.PassthroughIndices {
			cp.PassthroughIndices[k2] = v2
		}
	}
	if o.IntentionUpstreams != nil {
		cp.IntentionUpstreams = make(map[UpstreamID]struct{}, len(o.IntentionUpstreams))
		for k2, v2 := range o.IntentionUpstreams {
			cp.IntentionUpstreams[k2] = v2
		}
	}
	if o.PeeredUpstreams != nil {
		cp.PeeredUpstreams = make(map[UpstreamID]struct{}, len(o.PeeredUpstreams))
		for k2, v2 := range o.PeeredUpstreams {
			cp.PeeredUpstreams[k2] = v2
		}
	}
	cp.PeerUpstreamEndpoints = o.PeerUpstreamEndpoints.DeepCopy()
	if o.PeerUpstreamEndpointsUseHostnames != nil {
		cp.PeerUpstreamEndpointsUseHostnames = make(map[UpstreamID]struct{}, len(o.PeerUpstreamEndpointsUseHostnames))
		for k2, v2 := range o.PeerUpstreamEndpointsUseHostnames {
			cp.PeerUpstreamEndpointsUseHostnames[k2] = v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *PeerServersValue
func (o *PeerServersValue) DeepCopy() *PeerServersValue {
	var cp PeerServersValue = *o
	if o.Addresses != nil {
		cp.Addresses = make([]structs.ServiceAddress, len(o.Addresses))
		copy(cp.Addresses, o.Addresses)
	}
	return &cp
}

// DeepCopy generates a deep copy of *PeeringServiceValue
func (o *PeeringServiceValue) DeepCopy() *PeeringServiceValue {
	var cp PeeringServiceValue = *o
	cp.Nodes = o.Nodes.DeepCopy()
	return &cp
}

// DeepCopy generates a deep copy of *configSnapshotAPIGateway
func (o *configSnapshotAPIGateway) DeepCopy() *configSnapshotAPIGateway {
	var cp configSnapshotAPIGateway = *o
	{
		retV := o.ConfigSnapshotUpstreams.DeepCopy()
		cp.ConfigSnapshotUpstreams = *retV
	}
	if o.TLSConfig.SDS != nil {
		cp.TLSConfig.SDS = new(structs.GatewayTLSSDSConfig)
		*cp.TLSConfig.SDS = *o.TLSConfig.SDS
	}
	if o.TLSConfig.CipherSuites != nil {
		cp.TLSConfig.CipherSuites = make([]types.TLSCipherSuite, len(o.TLSConfig.CipherSuites))
		copy(cp.TLSConfig.CipherSuites, o.TLSConfig.CipherSuites)
	}
	if o.GatewayConfig != nil {
		cp.GatewayConfig = new(structs.APIGatewayConfigEntry)
		*cp.GatewayConfig = *o.GatewayConfig
		if o.GatewayConfig.Listeners != nil {
			cp.GatewayConfig.Listeners = make([]structs.APIGatewayListener, len(o.GatewayConfig.Listeners))
			copy(cp.GatewayConfig.Listeners, o.GatewayConfig.Listeners)
			for i4 := range o.GatewayConfig.Listeners {
				{
					retV := o.GatewayConfig.Listeners[i4].DeepCopy()
					cp.GatewayConfig.Listeners[i4] = *retV
				}
			}
		}
		{
			retV := o.GatewayConfig.Status.DeepCopy()
			cp.GatewayConfig.Status = *retV
		}
		if o.GatewayConfig.Meta != nil {
			cp.GatewayConfig.Meta = make(map[string]string, len(o.GatewayConfig.Meta))
			for k4, v4 := range o.GatewayConfig.Meta {
				cp.GatewayConfig.Meta[k4] = v4
			}
		}
	}
	if o.BoundGatewayConfig != nil {
		cp.BoundGatewayConfig = o.BoundGatewayConfig.DeepCopy()
	}
	if o.Upstreams != nil {
		cp.Upstreams = make(map[structs.ResourceReference]listenerUpstreamMap, len(o.Upstreams))
		for k2, v2 := range o.Upstreams {
			var cp_Upstreams_v2 listenerUpstreamMap
			if v2 != nil {
				cp_Upstreams_v2 = make(map[IngressListenerKey]structs.Upstreams, len(v2))
				for k3, v3 := range v2 {
					var cp_Upstreams_v2_v3 structs.Upstreams
					if v3 != nil {
						cp_Upstreams_v2_v3 = make([]structs.Upstream, len(v3))
						copy(cp_Upstreams_v2_v3, v3)
						for i4 := range v3 {
							{
								retV := v3[i4].DeepCopy()
								cp_Upstreams_v2_v3[i4] = *retV
							}
						}
					}
					cp_Upstreams_v2[k3] = cp_Upstreams_v2_v3
				}
			}
			cp.Upstreams[k2] = cp_Upstreams_v2
		}
	}
	if o.UpstreamsSet != nil {
		cp.UpstreamsSet = make(map[structs.ResourceReference]upstreamIDSet, len(o.UpstreamsSet))
		for k2, v2 := range o.UpstreamsSet {
			var cp_UpstreamsSet_v2 upstreamIDSet
			if v2 != nil {
				cp_UpstreamsSet_v2 = make(map[UpstreamID]struct{}, len(v2))
				for k3, v3 := range v2 {
					cp_UpstreamsSet_v2[k3] = v3
				}
			}
			cp.UpstreamsSet[k2] = cp_UpstreamsSet_v2
		}
	}
	cp.HTTPRoutes = o.HTTPRoutes.DeepCopy()
	cp.TCPRoutes = o.TCPRoutes.DeepCopy()
	cp.Certificates = o.Certificates.DeepCopy()
	if o.Listeners != nil {
		cp.Listeners = make(map[string]structs.APIGatewayListener, len(o.Listeners))
		for k2, v2 := range o.Listeners {
			var cp_Listeners_v2 structs.APIGatewayListener
			{
				retV := v2.DeepCopy()
				cp_Listeners_v2 = *retV
			}
			cp.Listeners[k2] = cp_Listeners_v2
		}
	}
	if o.BoundListeners != nil {
		cp.BoundListeners = make(map[string]structs.BoundAPIGatewayListener, len(o.BoundListeners))
		for k2, v2 := range o.BoundListeners {
			var cp_BoundListeners_v2 structs.BoundAPIGatewayListener
			{
				retV := v2.DeepCopy()
				cp_BoundListeners_v2 = *retV
			}
			cp.BoundListeners[k2] = cp_BoundListeners_v2
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *configSnapshotConnectProxy
func (o *configSnapshotConnectProxy) DeepCopy() *configSnapshotConnectProxy {
	var cp configSnapshotConnectProxy = *o
	{
		retV := o.ConfigSnapshotUpstreams.DeepCopy()
		cp.ConfigSnapshotUpstreams = *retV
	}
	if o.InboundPeerTrustBundles != nil {
		cp.InboundPeerTrustBundles = make([]*pbpeering.PeeringTrustBundle, len(o.InboundPeerTrustBundles))
		copy(cp.InboundPeerTrustBundles, o.InboundPeerTrustBundles)
		for i2 := range o.InboundPeerTrustBundles {
			if o.InboundPeerTrustBundles[i2] != nil {
				cp.InboundPeerTrustBundles[i2] = o.InboundPeerTrustBundles[i2].DeepCopy()
			}
		}
	}
	if o.WatchedServiceChecks != nil {
		cp.WatchedServiceChecks = make(map[structs.ServiceID][]structs.CheckType, len(o.WatchedServiceChecks))
		for k2, v2 := range o.WatchedServiceChecks {
			var cp_WatchedServiceChecks_v2 []structs.CheckType
			if v2 != nil {
				cp_WatchedServiceChecks_v2 = make([]structs.CheckType, len(v2))
				copy(cp_WatchedServiceChecks_v2, v2)
				for i3 := range v2 {
					{
						retV := v2[i3].DeepCopy()
						cp_WatchedServiceChecks_v2[i3] = *retV
					}
				}
			}
			cp.WatchedServiceChecks[k2] = cp_WatchedServiceChecks_v2
		}
	}
	if o.PreparedQueryEndpoints != nil {
		cp.PreparedQueryEndpoints = make(map[UpstreamID]structs.CheckServiceNodes, len(o.PreparedQueryEndpoints))
		for k2, v2 := range o.PreparedQueryEndpoints {
			var cp_PreparedQueryEndpoints_v2 structs.CheckServiceNodes
			cp_PreparedQueryEndpoints_v2 = v2.DeepCopy()
			cp.PreparedQueryEndpoints[k2] = cp_PreparedQueryEndpoints_v2
		}
	}
	if o.Intentions != nil {
		cp.Intentions = make([]*structs.Intention, len(o.Intentions))
		copy(cp.Intentions, o.Intentions)
		for i2 := range o.Intentions {
			if o.Intentions[i2] != nil {
				cp.Intentions[i2] = o.Intentions[i2].DeepCopy()
			}
		}
	}
	cp.DestinationsUpstream = o.DestinationsUpstream.DeepCopy()
	cp.DestinationGateways = o.DestinationGateways.DeepCopy()
	return &cp
}

// DeepCopy generates a deep copy of *configSnapshotIngressGateway
func (o *configSnapshotIngressGateway) DeepCopy() *configSnapshotIngressGateway {
	var cp configSnapshotIngressGateway = *o
	{
		retV := o.ConfigSnapshotUpstreams.DeepCopy()
		cp.ConfigSnapshotUpstreams = *retV
	}
	if o.TLSConfig.SDS != nil {
		cp.TLSConfig.SDS = new(structs.GatewayTLSSDSConfig)
		*cp.TLSConfig.SDS = *o.TLSConfig.SDS
	}
	if o.TLSConfig.CipherSuites != nil {
		cp.TLSConfig.CipherSuites = make([]types.TLSCipherSuite, len(o.TLSConfig.CipherSuites))
		copy(cp.TLSConfig.CipherSuites, o.TLSConfig.CipherSuites)
	}
	if o.Hosts != nil {
		cp.Hosts = make([]string, len(o.Hosts))
		copy(cp.Hosts, o.Hosts)
	}
	if o.Upstreams != nil {
		cp.Upstreams = make(map[IngressListenerKey]structs.Upstreams, len(o.Upstreams))
		for k2, v2 := range o.Upstreams {
			var cp_Upstreams_v2 structs.Upstreams
			if v2 != nil {
				cp_Upstreams_v2 = make([]structs.Upstream, len(v2))
				copy(cp_Upstreams_v2, v2)
				for i3 := range v2 {
					{
						retV := v2[i3].DeepCopy()
						cp_Upstreams_v2[i3] = *retV
					}
				}
			}
			cp.Upstreams[k2] = cp_Upstreams_v2
		}
	}
	if o.UpstreamsSet != nil {
		cp.UpstreamsSet = make(map[UpstreamID]struct{}, len(o.UpstreamsSet))
		for k2, v2 := range o.UpstreamsSet {
			cp.UpstreamsSet[k2] = v2
		}
	}
	if o.Listeners != nil {
		cp.Listeners = make(map[IngressListenerKey]structs.IngressListener, len(o.Listeners))
		for k2, v2 := range o.Listeners {
			var cp_Listeners_v2 structs.IngressListener
			{
				retV := v2.DeepCopy()
				cp_Listeners_v2 = *retV
			}
			cp.Listeners[k2] = cp_Listeners_v2
		}
	}
	if o.Defaults.PassiveHealthCheck != nil {
		cp.Defaults.PassiveHealthCheck = new(structs.PassiveHealthCheck)
		*cp.Defaults.PassiveHealthCheck = *o.Defaults.PassiveHealthCheck
		if o.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx != nil {
			cp.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx = new(uint32)
			*cp.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx = *o.Defaults.PassiveHealthCheck.EnforcingConsecutive5xx
		}
		if o.Defaults.PassiveHealthCheck.MaxEjectionPercent != nil {
			cp.Defaults.PassiveHealthCheck.MaxEjectionPercent = new(uint32)
			*cp.Defaults.PassiveHealthCheck.MaxEjectionPercent = *o.Defaults.PassiveHealthCheck.MaxEjectionPercent
		}
		if o.Defaults.PassiveHealthCheck.BaseEjectionTime != nil {
			cp.Defaults.PassiveHealthCheck.BaseEjectionTime = new(time.Duration)
			*cp.Defaults.PassiveHealthCheck.BaseEjectionTime = *o.Defaults.PassiveHealthCheck.BaseEjectionTime
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *configSnapshotMeshGateway
func (o *configSnapshotMeshGateway) DeepCopy() *configSnapshotMeshGateway {
	var cp configSnapshotMeshGateway = *o
	if o.WatchedServices != nil {
		cp.WatchedServices = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedServices))
		for k2, v2 := range o.WatchedServices {
			cp.WatchedServices[k2] = v2
		}
	}
	if o.WatchedGateways != nil {
		cp.WatchedGateways = make(map[string]context.CancelFunc, len(o.WatchedGateways))
		for k2, v2 := range o.WatchedGateways {
			cp.WatchedGateways[k2] = v2
		}
	}
	if o.ServiceGroups != nil {
		cp.ServiceGroups = make(map[structs.ServiceName]structs.CheckServiceNodes, len(o.ServiceGroups))
		for k2, v2 := range o.ServiceGroups {
			var cp_ServiceGroups_v2 structs.CheckServiceNodes
			cp_ServiceGroups_v2 = v2.DeepCopy()
			cp.ServiceGroups[k2] = cp_ServiceGroups_v2
		}
	}
	if o.PeeringServices != nil {
		cp.PeeringServices = make(map[string]map[structs.ServiceName]PeeringServiceValue, len(o.PeeringServices))
		for k2, v2 := range o.PeeringServices {
			var cp_PeeringServices_v2 map[structs.ServiceName]PeeringServiceValue
			if v2 != nil {
				cp_PeeringServices_v2 = make(map[structs.ServiceName]PeeringServiceValue, len(v2))
				for k3, v3 := range v2 {
					var cp_PeeringServices_v2_v3 PeeringServiceValue
					{
						retV := v3.DeepCopy()
						cp_PeeringServices_v2_v3 = *retV
					}
					cp_PeeringServices_v2[k3] = cp_PeeringServices_v2_v3
				}
			}
			cp.PeeringServices[k2] = cp_PeeringServices_v2
		}
	}
	if o.WatchedPeeringServices != nil {
		cp.WatchedPeeringServices = make(map[string]map[structs.ServiceName]context.CancelFunc, len(o.WatchedPeeringServices))
		for k2, v2 := range o.WatchedPeeringServices {
			var cp_WatchedPeeringServices_v2 map[structs.ServiceName]context.CancelFunc
			if v2 != nil {
				cp_WatchedPeeringServices_v2 = make(map[structs.ServiceName]context.CancelFunc, len(v2))
				for k3, v3 := range v2 {
					cp_WatchedPeeringServices_v2[k3] = v3
				}
			}
			cp.WatchedPeeringServices[k2] = cp_WatchedPeeringServices_v2
		}
	}
	if o.WatchedPeers != nil {
		cp.WatchedPeers = make(map[string]context.CancelFunc, len(o.WatchedPeers))
		for k2, v2 := range o.WatchedPeers {
			cp.WatchedPeers[k2] = v2
		}
	}
	if o.ServiceResolvers != nil {
		cp.ServiceResolvers = make(map[structs.ServiceName]*structs.ServiceResolverConfigEntry, len(o.ServiceResolvers))
		for k2, v2 := range o.ServiceResolvers {
			var cp_ServiceResolvers_v2 *structs.ServiceResolverConfigEntry
			if v2 != nil {
				cp_ServiceResolvers_v2 = v2.DeepCopy()
			}
			cp.ServiceResolvers[k2] = cp_ServiceResolvers_v2
		}
	}
	if o.GatewayGroups != nil {
		cp.GatewayGroups = make(map[string]structs.CheckServiceNodes, len(o.GatewayGroups))
		for k2, v2 := range o.GatewayGroups {
			var cp_GatewayGroups_v2 structs.CheckServiceNodes
			cp_GatewayGroups_v2 = v2.DeepCopy()
			cp.GatewayGroups[k2] = cp_GatewayGroups_v2
		}
	}
	if o.FedStateGateways != nil {
		cp.FedStateGateways = make(map[string]structs.CheckServiceNodes, len(o.FedStateGateways))
		for k2, v2 := range o.FedStateGateways {
			var cp_FedStateGateways_v2 structs.CheckServiceNodes
			cp_FedStateGateways_v2 = v2.DeepCopy()
			cp.FedStateGateways[k2] = cp_FedStateGateways_v2
		}
	}
	cp.WatchedLocalServers = o.WatchedLocalServers.DeepCopy()
	if o.HostnameDatacenters != nil {
		cp.HostnameDatacenters = make(map[string]structs.CheckServiceNodes, len(o.HostnameDatacenters))
		for k2, v2 := range o.HostnameDatacenters {
			var cp_HostnameDatacenters_v2 structs.CheckServiceNodes
			cp_HostnameDatacenters_v2 = v2.DeepCopy()
			cp.HostnameDatacenters[k2] = cp_HostnameDatacenters_v2
		}
	}
	if o.ExportedServicesSlice != nil {
		cp.ExportedServicesSlice = make([]structs.ServiceName, len(o.ExportedServicesSlice))
		copy(cp.ExportedServicesSlice, o.ExportedServicesSlice)
	}
	if o.ExportedServicesWithPeers != nil {
		cp.ExportedServicesWithPeers = make(map[structs.ServiceName][]string, len(o.ExportedServicesWithPeers))
		for k2, v2 := range o.ExportedServicesWithPeers {
			var cp_ExportedServicesWithPeers_v2 []string
			if v2 != nil {
				cp_ExportedServicesWithPeers_v2 = make([]string, len(v2))
				copy(cp_ExportedServicesWithPeers_v2, v2)
			}
			cp.ExportedServicesWithPeers[k2] = cp_ExportedServicesWithPeers_v2
		}
	}
	if o.DiscoveryChain != nil {
		cp.DiscoveryChain = make(map[structs.ServiceName]*structs.CompiledDiscoveryChain, len(o.DiscoveryChain))
		for k2, v2 := range o.DiscoveryChain {
			var cp_DiscoveryChain_v2 *structs.CompiledDiscoveryChain
			if v2 != nil {
				cp_DiscoveryChain_v2 = v2.DeepCopy()
			}
			cp.DiscoveryChain[k2] = cp_DiscoveryChain_v2
		}
	}
	if o.WatchedDiscoveryChains != nil {
		cp.WatchedDiscoveryChains = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedDiscoveryChains))
		for k2, v2 := range o.WatchedDiscoveryChains {
			cp.WatchedDiscoveryChains[k2] = v2
		}
	}
	if o.MeshConfig != nil {
		cp.MeshConfig = o.MeshConfig.DeepCopy()
	}
	if o.Leaf != nil {
		cp.Leaf = new(structs.IssuedCert)
		*cp.Leaf = *o.Leaf
	}
	if o.PeerServers != nil {
		cp.PeerServers = make(map[string]PeerServersValue, len(o.PeerServers))
		for k2, v2 := range o.PeerServers {
			var cp_PeerServers_v2 PeerServersValue
			{
				retV := v2.DeepCopy()
				cp_PeerServers_v2 = *retV
			}
			cp.PeerServers[k2] = cp_PeerServers_v2
		}
	}
	if o.PeeringTrustBundles != nil {
		cp.PeeringTrustBundles = make([]*pbpeering.PeeringTrustBundle, len(o.PeeringTrustBundles))
		copy(cp.PeeringTrustBundles, o.PeeringTrustBundles)
		for i2 := range o.PeeringTrustBundles {
			if o.PeeringTrustBundles[i2] != nil {
				cp.PeeringTrustBundles[i2] = o.PeeringTrustBundles[i2].DeepCopy()
			}
		}
	}
	return &cp
}

// DeepCopy generates a deep copy of *configSnapshotTerminatingGateway
func (o *configSnapshotTerminatingGateway) DeepCopy() *configSnapshotTerminatingGateway {
	var cp configSnapshotTerminatingGateway = *o
	if o.MeshConfig != nil {
		cp.MeshConfig = o.MeshConfig.DeepCopy()
	}
	if o.WatchedServices != nil {
		cp.WatchedServices = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedServices))
		for k2, v2 := range o.WatchedServices {
			cp.WatchedServices[k2] = v2
		}
	}
	if o.WatchedIntentions != nil {
		cp.WatchedIntentions = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedIntentions))
		for k2, v2 := range o.WatchedIntentions {
			cp.WatchedIntentions[k2] = v2
		}
	}
	if o.Intentions != nil {
		cp.Intentions = make(map[structs.ServiceName]structs.SimplifiedIntentions, len(o.Intentions))
		for k2, v2 := range o.Intentions {
			var cp_Intentions_v2 structs.SimplifiedIntentions
			if v2 != nil {
				cp_Intentions_v2 = make([]*structs.Intention, len(v2))
				copy(cp_Intentions_v2, v2)
				for i3 := range v2 {
					if v2[i3] != nil {
						cp_Intentions_v2[i3] = v2[i3].DeepCopy()
					}
				}
			}
			cp.Intentions[k2] = cp_Intentions_v2
		}
	}
	if o.WatchedLeaves != nil {
		cp.WatchedLeaves = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedLeaves))
		for k2, v2 := range o.WatchedLeaves {
			cp.WatchedLeaves[k2] = v2
		}
	}
	if o.ServiceLeaves != nil {
		cp.ServiceLeaves = make(map[structs.ServiceName]*structs.IssuedCert, len(o.ServiceLeaves))
		for k2, v2 := range o.ServiceLeaves {
			var cp_ServiceLeaves_v2 *structs.IssuedCert
			if v2 != nil {
				cp_ServiceLeaves_v2 = new(structs.IssuedCert)
				*cp_ServiceLeaves_v2 = *v2
			}
			cp.ServiceLeaves[k2] = cp_ServiceLeaves_v2
		}
	}
	if o.WatchedConfigs != nil {
		cp.WatchedConfigs = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedConfigs))
		for k2, v2 := range o.WatchedConfigs {
			cp.WatchedConfigs[k2] = v2
		}
	}
	if o.ServiceConfigs != nil {
		cp.ServiceConfigs = make(map[structs.ServiceName]*structs.ServiceConfigResponse, len(o.ServiceConfigs))
		for k2, v2 := range o.ServiceConfigs {
			var cp_ServiceConfigs_v2 *structs.ServiceConfigResponse
			if v2 != nil {
				cp_ServiceConfigs_v2 = v2.DeepCopy()
			}
			cp.ServiceConfigs[k2] = cp_ServiceConfigs_v2
		}
	}
	if o.WatchedResolvers != nil {
		cp.WatchedResolvers = make(map[structs.ServiceName]context.CancelFunc, len(o.WatchedResolvers))
		for k2, v2 := range o.WatchedResolvers {
			cp.WatchedResolvers[k2] = v2
		}
	}
	if o.ServiceResolvers != nil {
		cp.ServiceResolvers = make(map[structs.ServiceName]*structs.ServiceResolverConfigEntry, len(o.ServiceResolvers))
		for k2, v2 := range o.ServiceResolvers {
			var cp_ServiceResolvers_v2 *structs.ServiceResolverConfigEntry
			if v2 != nil {
				cp_ServiceResolvers_v2 = v2.DeepCopy()
			}
			cp.ServiceResolvers[k2] = cp_ServiceResolvers_v2
		}
	}
	if o.ServiceResolversSet != nil {
		cp.ServiceResolversSet = make(map[structs.ServiceName]bool, len(o.ServiceResolversSet))
		for k2, v2 := range o.ServiceResolversSet {
			cp.ServiceResolversSet[k2] = v2
		}
	}
	if o.ServiceGroups != nil {
		cp.ServiceGroups = make(map[structs.ServiceName]structs.CheckServiceNodes, len(o.ServiceGroups))
		for k2, v2 := range o.ServiceGroups {
			var cp_ServiceGroups_v2 structs.CheckServiceNodes
			cp_ServiceGroups_v2 = v2.DeepCopy()
			cp.ServiceGroups[k2] = cp_ServiceGroups_v2
		}
	}
	if o.GatewayServices != nil {
		cp.GatewayServices = make(map[structs.ServiceName]structs.GatewayService, len(o.GatewayServices))
		for k2, v2 := range o.GatewayServices {
			var cp_GatewayServices_v2 structs.GatewayService
			{
				retV := v2.DeepCopy()
				cp_GatewayServices_v2 = *retV
			}
			cp.GatewayServices[k2] = cp_GatewayServices_v2
		}
	}
	if o.DestinationServices != nil {
		cp.DestinationServices = make(map[structs.ServiceName]structs.GatewayService, len(o.DestinationServices))
		for k2, v2 := range o.DestinationServices {
			var cp_DestinationServices_v2 structs.GatewayService
			{
				retV := v2.DeepCopy()
				cp_DestinationServices_v2 = *retV
			}
			cp.DestinationServices[k2] = cp_DestinationServices_v2
		}
	}
	if o.HostnameServices != nil {
		cp.HostnameServices = make(map[structs.ServiceName]structs.CheckServiceNodes, len(o.HostnameServices))
		for k2, v2 := range o.HostnameServices {
			var cp_HostnameServices_v2 structs.CheckServiceNodes
			cp_HostnameServices_v2 = v2.DeepCopy()
			cp.HostnameServices[k2] = cp_HostnameServices_v2
		}
	}
	return &cp
}
