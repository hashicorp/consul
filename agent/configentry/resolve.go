package configentry

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/agent/structs"
)

func ComputeResolvedServiceConfig(
	args *structs.ServiceConfigRequest,
	entries *ResolvedServiceConfigSet,
	logger hclog.Logger,
) (*structs.ServiceConfigResponse, error) {
	var thisReply structs.ServiceConfigResponse

	thisReply.MeshGateway.Mode = structs.MeshGatewayModeDefault

	// Store the upstream defaults under a wildcard key so that they can be applied to
	// upstreams that are inferred from intentions and do not have explicit upstream configuration.
	wildcard := structs.PeeredServiceName{
		ServiceName: structs.NewServiceName(structs.WildcardSpecifier, args.WithWildcardNamespace()),
	}
	wildcardUpstreamDefaults := make(map[string]interface{})
	// resolvedConfigs stores the opaque config map for each upstream and is keyed on the upstream's ID.
	resolvedConfigs := make(map[structs.PeeredServiceName]map[string]interface{})

	// TODO(freddy) Refactor this into smaller set of state store functions
	// Pass the WatchSet to both the service and proxy config lookups. If either is updated during the
	// blocking query, this function will be rerun and these state store lookups will both be current.
	// We use the default enterprise meta to look up the global proxy defaults because they are not namespaced.

	var proxyConfGlobalProtocol string
	proxyConf := entries.GetProxyDefaults(args.PartitionOrDefault())
	if proxyConf != nil {
		// Apply the proxy defaults to the sidecar's proxy config
		mapCopy, err := copystructure.Copy(proxyConf.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to copy global proxy-defaults: %v", err)
		}

		thisReply.ProxyConfig = mapCopy.(map[string]interface{})
		thisReply.Mode = proxyConf.Mode
		thisReply.TransparentProxy = proxyConf.TransparentProxy
		thisReply.MeshGateway = proxyConf.MeshGateway
		thisReply.Expose = proxyConf.Expose
		thisReply.EnvoyExtensions = proxyConf.EnvoyExtensions
		thisReply.AccessLogs = proxyConf.AccessLogs

		// Only MeshGateway and Protocol should affect upstreams.
		// MeshGateway is strange. It's marshaled into UpstreamConfigs via the arbitrary map, but it
		// uses concrete fields everywhere else. We always take the explicit definition here for
		// wildcard upstreams and discard the user setting it via arbitrary map in proxy-defaults.
		if mgw, ok := thisReply.ProxyConfig["mesh_gateway"]; ok {
			wildcardUpstreamDefaults["mesh_gateway"] = mgw
		}
		if !proxyConf.MeshGateway.IsZero() {
			wildcardUpstreamDefaults["mesh_gateway"] = proxyConf.MeshGateway
		}

		// We explicitly DO NOT merge the protocol from proxy-defaults into the wildcard upstream here.
		// TProxy will try to use the data from the `wildcardUpstreamDefaults` as a source of truth, which is
		// normally correct to inherit from proxy-defaults. However, it is NOT correct for protocol.
		//
		// This edge-case is different for `protocol` from other fields, since the protocol can be
		// set on both the local `ServiceDefaults.UpstreamOverrides` and upstream `ServiceDefaults.Protocol`.
		// This means that when proxy-defaults is set, it would always be treated as an explicit override,
		// and take precedence over the protocol that is set on the discovery chain (which comes from the
		// service's preference in its service-defaults), which is wrong.
		//
		// When the upstream is not explicitly defined, we should only get the protocol from one of these locations:
		//   1. For tproxy non-peering services, it can be fetched via the discovery chain.
		//      The chain compiler merges the proxy-defaults protocol with the upstream's preferred service-defaults protocol.
		//   2. For tproxy non-peering services with default upstream overrides, it will come from the wildcard upstream overrides.
		//   3. For tproxy non-peering services with specific upstream overrides, it will come from the specific upstream override defined.
		//   4. For tproxy peering services, they do not honor the proxy-defaults, since they reside in a different cluster.
		//      The data will come from a separate peerMeta field.
		// In all of these cases, it is not necessary for the proxy-defaults to exist in the wildcard upstream.
		parsed, err := structs.ParseUpstreamConfigNoDefaults(mapCopy.(map[string]interface{}))
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream config map for proxy-defaults: %v", err)
		}
		proxyConfGlobalProtocol = parsed.Protocol
	}

	serviceConf := entries.GetServiceDefaults(
		structs.NewServiceID(args.Name, &args.EnterpriseMeta),
	)
	if serviceConf != nil {

		if serviceConf.Expose.Checks {
			thisReply.Expose.Checks = true
		}
		if len(serviceConf.Expose.Paths) >= 1 {
			thisReply.Expose.Paths = serviceConf.Expose.Paths
		}
		if serviceConf.MeshGateway.Mode != structs.MeshGatewayModeDefault {
			thisReply.MeshGateway.Mode = serviceConf.MeshGateway.Mode
			wildcardUpstreamDefaults["mesh_gateway"] = serviceConf.MeshGateway
		}
		if serviceConf.TransparentProxy.OutboundListenerPort != 0 {
			thisReply.TransparentProxy.OutboundListenerPort = serviceConf.TransparentProxy.OutboundListenerPort
		}
		if serviceConf.TransparentProxy.DialedDirectly {
			thisReply.TransparentProxy.DialedDirectly = serviceConf.TransparentProxy.DialedDirectly
		}
		if serviceConf.Mode != structs.ProxyModeDefault {
			thisReply.Mode = serviceConf.Mode
		}
		if serviceConf.Destination != nil {
			thisReply.Destination = *serviceConf.Destination
		}

		// Populate values for the proxy config map
		proxyConf := thisReply.ProxyConfig
		if proxyConf == nil {
			proxyConf = make(map[string]interface{})
		}
		if serviceConf.Protocol != "" {
			proxyConf["protocol"] = serviceConf.Protocol
		}
		if serviceConf.BalanceInboundConnections != "" {
			proxyConf["balance_inbound_connections"] = serviceConf.BalanceInboundConnections
		}
		if serviceConf.MaxInboundConnections > 0 {
			proxyConf["max_inbound_connections"] = serviceConf.MaxInboundConnections
		}
		if serviceConf.LocalConnectTimeoutMs > 0 {
			proxyConf["local_connect_timeout_ms"] = serviceConf.LocalConnectTimeoutMs
		}
		if serviceConf.LocalRequestTimeoutMs > 0 {
			proxyConf["local_request_timeout_ms"] = serviceConf.LocalRequestTimeoutMs
		}
		// Add the proxy conf to the response if any fields were populated
		if len(proxyConf) > 0 {
			thisReply.ProxyConfig = proxyConf
		}

		thisReply.Meta = serviceConf.Meta
		// Service defaults' envoy extensions are appended to the proxy defaults extensions so that proxy defaults
		// extensions are applied first.
		thisReply.EnvoyExtensions = append(thisReply.EnvoyExtensions, serviceConf.EnvoyExtensions...)
	}

	// First collect all upstreams into a set of seen upstreams.
	// Upstreams can come from:
	// - Explicitly from proxy registrations, and therefore as an argument to this RPC endpoint
	// - Implicitly from centralized upstream config in service-defaults
	seenUpstreams := map[structs.PeeredServiceName]struct{}{}

	var (
		noUpstreamArgs = len(args.UpstreamServiceNames) == 0 && len(args.UpstreamIDs) == 0

		// Check the args and the resolved value. If it was exclusively set via a config entry, then args.Mode
		// will never be transparent because the service config request does not use the resolved value.
		tproxy = args.Mode == structs.ProxyModeTransparent || thisReply.Mode == structs.ProxyModeTransparent
	)

	// The upstreams passed as arguments to this endpoint are the upstreams explicitly defined in a proxy registration.
	// If no upstreams were passed, then we should only return the resolved config if the proxy is in transparent mode.
	// Otherwise we would return a resolved upstream config to a proxy with no configured upstreams.
	if noUpstreamArgs && !tproxy {
		return &thisReply, nil
	}

	// First store all upstreams that were provided in the request
	for _, psn := range args.UpstreamServiceNames {
		if _, ok := seenUpstreams[psn]; !ok {
			seenUpstreams[psn] = struct{}{}
		}
	}
	// For 1.14, service-defaults overrides would apply to peer upstreams incorrectly
	// because the config merging logic was oblivious to the concept of a peer.
	// We replicate this behavior on legacy calls for backwards-compatibility.
	for _, sid := range args.UpstreamIDs {
		psn := structs.PeeredServiceName{
			ServiceName: structs.NewServiceName(sid.ID, &sid.EnterpriseMeta),
		}
		seenUpstreams[psn] = struct{}{}
	}

	// Then store upstreams inferred from service-defaults and mapify the overrides.
	var (
		upstreamDefaults  *structs.UpstreamConfig
		upstreamOverrides = make(map[structs.PeeredServiceName]*structs.UpstreamConfig)
	)
	if serviceConf != nil && serviceConf.UpstreamConfig != nil {
		for i, override := range serviceConf.UpstreamConfig.Overrides {
			if override.Name == "" {
				logger.Warn(
					"Skipping UpstreamConfig.Overrides entry without a required name field",
					"entryIndex", i,
					"kind", serviceConf.GetKind(),
					"name", serviceConf.GetName(),
					"namespace", serviceConf.GetEnterpriseMeta().NamespaceOrEmpty(),
				)
				continue // skip this impossible condition
			}
			psn := override.PeeredServiceName()
			seenUpstreams[psn] = struct{}{}
			upstreamOverrides[psn] = override
		}
		if serviceConf.UpstreamConfig.Defaults != nil {
			upstreamDefaults = serviceConf.UpstreamConfig.Defaults
			if upstreamDefaults.MeshGateway.Mode == structs.MeshGatewayModeDefault {
				upstreamDefaults.MeshGateway.Mode = thisReply.MeshGateway.Mode
			}
			upstreamDefaults.MergeInto(wildcardUpstreamDefaults)
			// Always add the wildcard upstream if a service-defaults default-upstream was configured.
			resolvedConfigs[wildcard] = wildcardUpstreamDefaults
		}
	}

	if !args.MeshGateway.IsZero() {
		wildcardUpstreamDefaults["mesh_gateway"] = args.MeshGateway
	}

	// Add the wildcard upstream if any fields were populated (it may have been already
	// added if a service-defaults exists). We likely could always add it without issues,
	// but this has been existing behavior, and many unit tests would break.
	if len(wildcardUpstreamDefaults) > 0 {
		resolvedConfigs[wildcard] = wildcardUpstreamDefaults
	}

	// For Consul 1.14.x, service-defaults would apply to either local or peer services as long
	// as the `name` matched. We introduce `legacyUpstreams` as a compatibility mode for:
	//   1. old agents, that are using the deprecated UpstreamIDs api
	//   2. Migrations to 1.15 that do not specify the "peer" field. The behavior should remain the same
	//      until the config entries are updates.
	//
	// This should be remove in Consul 1.16
	var hasPeerUpstream bool
	for _, override := range upstreamOverrides {
		if override.Peer != "" {
			hasPeerUpstream = true
			break
		}
	}
	legacyUpstreams := len(args.UpstreamIDs) > 0 || !hasPeerUpstream

	for upstream := range seenUpstreams {
		resolvedCfg := make(map[string]interface{})

		// The protocol of an upstream is resolved in this order:
		// 1. Default protocol from proxy-defaults (how all services should be addressed)
		// 2. Protocol for upstream service defined in its service-defaults (how the upstream wants to be addressed)
		// 3. Protocol defined for the upstream in the service-defaults.(upstream_config.defaults|upstream_config.overrides) of the downstream
		// 	  (how the downstream wants to address it)
		if proxyConfGlobalProtocol != "" {
			resolvedCfg["protocol"] = proxyConfGlobalProtocol
		}

		if err := mergo.MergeWithOverwrite(&resolvedCfg, wildcardUpstreamDefaults); err != nil {
			return nil, fmt.Errorf("failed to merge wildcard defaults into upstream: %v", err)
		}

		upstreamSvcDefaults := entries.GetServiceDefaults(upstream.ServiceName.ToServiceID())
		if upstreamSvcDefaults != nil {
			if upstreamSvcDefaults.Protocol != "" {
				resolvedCfg["protocol"] = upstreamSvcDefaults.Protocol
			}
		}

		// When dialing an upstream, the goal is to flatten the mesh gateway mode in this order
		// (larger number wins):
		//  1. Value from the proxy-defaults
		//  2. Value from top-level of service-defaults (ServiceDefaults.MeshGateway)
		//  3. Value from centralized upstream defaults (ServiceDefaults.UpstreamConfig.Defaults)
		//  4. Value from local proxy registration (NodeService.Proxy.MeshGateway)
		//  5. Value from centralized upstream override (ServiceDefaults.UpstreamConfig.Overrides)
		//  6. Value from local upstream definition (NodeService.Proxy.Upstreams[].MeshGateway)
		//
		// The MeshGateway value from upstream definitions in the proxy registration override
		// the one from UpstreamConfig.Defaults and UpstreamConfig.Overrides because they are
		// specific to the proxy instance.
		//
		// Step 6 is handled by the dialer's ServiceManager in MergeServiceConfig.

		// Start with the merged value from proxyConf and serviceConf. (steps 1-2)
		if !thisReply.MeshGateway.IsZero() {
			resolvedCfg["mesh_gateway"] = thisReply.MeshGateway
		}

		// Merge in the upstream defaults (step 3).
		if upstreamDefaults != nil {
			upstreamDefaults.MergeInto(resolvedCfg)
		}

		// Merge in the top-level mode from the proxy instance (step 4).
		if !args.MeshGateway.IsZero() {
			// This means each upstream inherits the value from the `NodeService.Proxy.MeshGateway` field.
			resolvedCfg["mesh_gateway"] = args.MeshGateway
		}

		// Merge in Overrides for the upstream (step 5).
		// In the legacy case, overrides only match on name. We remove the peer and try to match against
		// our map of overrides. We still want to check the full PSN in the map in case there is a specific
		// override that applies to peers.
		if legacyUpstreams {
			peerlessUpstream := upstream
			peerlessUpstream.Peer = ""
			if upstreamOverrides[peerlessUpstream] != nil {
				upstreamOverrides[peerlessUpstream].MergeInto(resolvedCfg)
			}
		}
		if upstreamOverrides[upstream] != nil {
			upstreamOverrides[upstream].MergeInto(resolvedCfg)
		}

		if len(resolvedCfg) > 0 {
			resolvedConfigs[upstream] = resolvedCfg
		}
	}

	// don't allocate the slices just to not fill them
	if len(resolvedConfigs) == 0 {
		return &thisReply, nil
	}

	if len(args.UpstreamIDs) > 0 {
		// DEPRECATED: Remove these legacy upstreams in Consul v1.16
		thisReply.UpstreamIDConfigs = make(structs.OpaqueUpstreamConfigsDeprecated, 0, len(resolvedConfigs))

		for us, conf := range resolvedConfigs {
			thisReply.UpstreamIDConfigs = append(thisReply.UpstreamIDConfigs,
				structs.OpaqueUpstreamConfigDeprecated{Upstream: us.ServiceName.ToServiceID(), Config: conf})
		}
	} else {
		thisReply.UpstreamConfigs = make(structs.OpaqueUpstreamConfigs, 0, len(resolvedConfigs))

		for us, conf := range resolvedConfigs {
			thisReply.UpstreamConfigs = append(thisReply.UpstreamConfigs,
				structs.OpaqueUpstreamConfig{Upstream: us, Config: conf})
		}
	}

	return &thisReply, nil
}
