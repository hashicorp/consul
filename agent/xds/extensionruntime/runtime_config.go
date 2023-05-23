// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package extensionruntime

import (
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/envoyextensions/extensioncommon"
)

func GetRuntimeConfigurations(cfgSnap *proxycfg.ConfigSnapshot) map[api.CompoundServiceName][]extensioncommon.RuntimeConfig {
	extensionsMap := make(map[api.CompoundServiceName][]api.EnvoyExtension)
	upstreamMap := make(map[api.CompoundServiceName]*extensioncommon.UpstreamData)
	var kind api.ServiceKind
	extensionConfigurationsMap := make(map[api.CompoundServiceName][]extensioncommon.RuntimeConfig)

	trustDomain := ""
	if cfgSnap.Roots != nil {
		trustDomain = cfgSnap.Roots.TrustDomain
	}

	switch cfgSnap.Kind {
	case structs.ServiceKindConnectProxy:
		kind = api.ServiceKindConnectProxy
		outgoingKindByService := make(map[api.CompoundServiceName]api.ServiceKind)
		vipForService := make(map[api.CompoundServiceName]string)
		for uid, upstreamData := range cfgSnap.ConnectProxy.WatchedUpstreamEndpoints {
			sn := upstreamIDToCompoundServiceName(uid)

			for _, serviceNodes := range upstreamData {
				for _, serviceNode := range serviceNodes {
					if serviceNode.Service == nil {
						continue
					}
					vip := serviceNode.Service.TaggedAddresses[structs.TaggedAddressVirtualIP].Address
					if vip != "" {
						if _, ok := vipForService[sn]; !ok {
							vipForService[sn] = vip
						}
					}
					// Store the upstream's kind, and for ServiceKindTypical we don't do anything because we'll default
					// any unset upstreams to ServiceKindConnectProxy below.
					switch serviceNode.Service.Kind {
					case structs.ServiceKindTypical:
					default:
						outgoingKindByService[sn] = api.ServiceKind(serviceNode.Service.Kind)
					}
					// We only need the kind from one instance, so break once we find it.
					break
				}
			}
		}

		// TODO(peering): consider PeerUpstreamEndpoints in addition to DiscoveryChain
		// These are the discovery chains for upstreams which have the Envoy Extensions applied to the local service.
		for uid, dc := range cfgSnap.ConnectProxy.DiscoveryChain {
			compoundServiceName := upstreamIDToCompoundServiceName(uid)
			extensionsMap[compoundServiceName] = convertEnvoyExtensions(dc.EnvoyExtensions)

			meta := uid.EnterpriseMeta
			sni := connect.ServiceSNI(uid.Name, "", meta.NamespaceOrDefault(), meta.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
			outgoingKind, ok := outgoingKindByService[compoundServiceName]
			if !ok {
				outgoingKind = api.ServiceKindConnectProxy
			}

			upstreamMap[compoundServiceName] = &extensioncommon.UpstreamData{
				SNI:               map[string]struct{}{sni: {}},
				VIP:               vipForService[compoundServiceName],
				EnvoyID:           uid.EnvoyID(),
				OutgoingProxyKind: outgoingKind,
			}
		}
		// Adds extensions configured for the local service to the RuntimeConfig. This only applies to
		// connect-proxies because extensions are either global or tied to a specific service, so the terminating
		// gateway's Envoy resources for the local service (i.e not to upstreams) would never need to be modified.
		localSvc := api.CompoundServiceName{
			Name:      cfgSnap.Proxy.DestinationServiceName,
			Namespace: cfgSnap.ProxyID.NamespaceOrDefault(),
			Partition: cfgSnap.ProxyID.PartitionOrEmpty(),
		}
		extensionConfigurationsMap[localSvc] = []extensioncommon.RuntimeConfig{}
		cfgSnapExts := convertEnvoyExtensions(cfgSnap.Proxy.EnvoyExtensions)
		for _, ext := range cfgSnapExts {
			extCfg := extensioncommon.RuntimeConfig{
				EnvoyExtension:        ext,
				ServiceName:           localSvc,
				IsSourcedFromUpstream: false,
				Upstreams:             upstreamMap,
				Kind:                  kind,
				Protocol:              proxyConfigProtocol(cfgSnap.Proxy.Config),
			}
			extensionConfigurationsMap[localSvc] = append(extensionConfigurationsMap[localSvc], extCfg)
		}
	case structs.ServiceKindTerminatingGateway:
		kind = api.ServiceKindTerminatingGateway
		for svc, c := range cfgSnap.TerminatingGateway.ServiceConfigs {
			compoundServiceName := serviceNameToCompoundServiceName(svc)
			extensionsMap[compoundServiceName] = convertEnvoyExtensions(c.EnvoyExtensions)

			sni := connect.ServiceSNI(svc.Name, "", svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
			envoyID := proxycfg.NewUpstreamIDFromServiceName(svc)

			snis := map[string]struct{}{sni: {}}

			resolver, hasResolver := cfgSnap.TerminatingGateway.ServiceResolvers[svc]
			if hasResolver {
				for subsetName := range resolver.Subsets {
					sni := connect.ServiceSNI(svc.Name, subsetName, svc.NamespaceOrDefault(), svc.PartitionOrDefault(), cfgSnap.Datacenter, trustDomain)
					snis[sni] = struct{}{}
				}
			}

			upstreamMap[compoundServiceName] = &extensioncommon.UpstreamData{
				SNI:               snis,
				EnvoyID:           envoyID.EnvoyID(),
				OutgoingProxyKind: api.ServiceKindTerminatingGateway,
			}

		}
	}

	// If applicable, include extension configuration for remote upstreams of the local service.
	// This only applies to specific extensions authorized to apply to remote proxies.
	for svc, exts := range extensionsMap {
		extensionConfigurationsMap[svc] = []extensioncommon.RuntimeConfig{}
		for _, ext := range exts {
			if appliesToRemoteDownstreams(ext) {
				extCfg := extensioncommon.RuntimeConfig{
					EnvoyExtension:        ext,
					Kind:                  kind,
					ServiceName:           svc,
					IsSourcedFromUpstream: true,
					Upstreams:             upstreamMap,
					Protocol:              proxyConfigProtocol(cfgSnap.Proxy.Config),
				}
				extensionConfigurationsMap[svc] = append(extensionConfigurationsMap[svc], extCfg)
			}
		}
	}

	return extensionConfigurationsMap
}

func serviceNameToCompoundServiceName(svc structs.ServiceName) api.CompoundServiceName {
	return api.CompoundServiceName{
		Name:      svc.Name,
		Partition: svc.PartitionOrDefault(),
		Namespace: svc.NamespaceOrDefault(),
	}
}

func upstreamIDToCompoundServiceName(uid proxycfg.UpstreamID) api.CompoundServiceName {
	return api.CompoundServiceName{
		Name:      uid.Name,
		Partition: uid.PartitionOrDefault(),
		Namespace: uid.NamespaceOrDefault(),
	}
}

func convertEnvoyExtensions(structExtensions structs.EnvoyExtensions) []api.EnvoyExtension {
	return structExtensions.ToAPI()
}

func proxyConfigProtocol(cfg map[string]any) string {
	if p, exists := cfg["protocol"]; exists {
		if protocol, ok := p.(string); ok {
			return protocol
		}
	}
	return ""
}

// appliesToRemoteDownstreams returns true if the given extension should be applied to remote downstream proxies of the
// service targeted by the extension, rather than just the local proxy. In the context of GetRuntimeConfigurations, this
// determines whether the extension should apply to the local proxy (a downstream of the configured service).
//
// Currently, only the AWS Lambda and Validate extensions are allowed to apply to downstream proxies.
//
// See extensioncommon.RuntimeConfig.IsSourcedFromUpstream and UpstreamEnvoyExtender doc for more information. We make
// this check here out of precaution s.t. even if an unauthorized extension is erroneously constructed with the
// UpstreamEnvoyExtender, this check will not allow the upstream extension configuration to be provided.
func appliesToRemoteDownstreams(extension api.EnvoyExtension) bool {
	return extension.Name == api.BuiltinAWSLambdaExtension || extension.Name == api.BuiltinValidateExtension
}
