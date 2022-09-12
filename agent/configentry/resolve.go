package configentry

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/agent/structs"
)

func ComputeResolvedServiceConfig(
	args *structs.ServiceConfigRequest,
	upstreamIDs []structs.ServiceID,
	legacyUpstreams bool,
	entries *ResolvedServiceConfigSet,
	logger hclog.Logger,
) (*structs.ServiceConfigResponse, error) {
	var thisReply structs.ServiceConfigResponse

	thisReply.MeshGateway.Mode = structs.MeshGatewayModeDefault

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

		// Extract the global protocol from proxyConf for upstream configs.
		rawProtocol := proxyConf.Config["protocol"]
		if rawProtocol != nil {
			var ok bool
			proxyConfGlobalProtocol, ok = rawProtocol.(string)
			if !ok {
				return nil, fmt.Errorf("invalid protocol type %T", rawProtocol)
			}
		}
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
		}
		if serviceConf.Protocol != "" {
			if thisReply.ProxyConfig == nil {
				thisReply.ProxyConfig = make(map[string]interface{})
			}
			thisReply.ProxyConfig["protocol"] = serviceConf.Protocol
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

		if serviceConf.MaxInboundConnections > 0 {
			if thisReply.ProxyConfig == nil {
				thisReply.ProxyConfig = map[string]interface{}{}
			}
			thisReply.ProxyConfig["max_inbound_connections"] = serviceConf.MaxInboundConnections
		}

		if serviceConf.LocalConnectTimeoutMs > 0 {
			if thisReply.ProxyConfig == nil {
				thisReply.ProxyConfig = map[string]interface{}{}
			}
			thisReply.ProxyConfig["local_connect_timeout_ms"] = serviceConf.LocalConnectTimeoutMs
		}

		if serviceConf.LocalRequestTimeoutMs > 0 {
			if thisReply.ProxyConfig == nil {
				thisReply.ProxyConfig = map[string]interface{}{}
			}
			thisReply.ProxyConfig["local_request_timeout_ms"] = serviceConf.LocalRequestTimeoutMs
		}

		thisReply.Meta = serviceConf.Meta
	}

	// First collect all upstreams into a set of seen upstreams.
	// Upstreams can come from:
	// - Explicitly from proxy registrations, and therefore as an argument to this RPC endpoint
	// - Implicitly from centralized upstream config in service-defaults
	seenUpstreams := map[structs.ServiceID]struct{}{}

	var (
		noUpstreamArgs = len(upstreamIDs) == 0 && len(args.Upstreams) == 0

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
	for _, sid := range upstreamIDs {
		if _, ok := seenUpstreams[sid]; !ok {
			seenUpstreams[sid] = struct{}{}
		}
	}

	// Then store upstreams inferred from service-defaults and mapify the overrides.
	var (
		upstreamConfigs  = make(map[structs.ServiceID]*structs.UpstreamConfig)
		upstreamDefaults *structs.UpstreamConfig
		// usConfigs stores the opaque config map for each upstream and is keyed on the upstream's ID.
		usConfigs = make(map[structs.ServiceID]map[string]interface{})
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
			seenUpstreams[override.ServiceID()] = struct{}{}
			upstreamConfigs[override.ServiceID()] = override
		}
		if serviceConf.UpstreamConfig.Defaults != nil {
			upstreamDefaults = serviceConf.UpstreamConfig.Defaults

			// Store the upstream defaults under a wildcard key so that they can be applied to
			// upstreams that are inferred from intentions and do not have explicit upstream configuration.
			cfgMap := make(map[string]interface{})
			upstreamDefaults.MergeInto(cfgMap)

			wildcard := structs.NewServiceID(structs.WildcardSpecifier, args.WithWildcardNamespace())
			usConfigs[wildcard] = cfgMap
		}
	}

	for upstream := range seenUpstreams {
		resolvedCfg := make(map[string]interface{})

		// The protocol of an upstream is resolved in this order:
		// 1. Default protocol from proxy-defaults (how all services should be addressed)
		// 2. Protocol for upstream service defined in its service-defaults (how the upstream wants to be addressed)
		// 3. Protocol defined for the upstream in the service-defaults.(upstream_config.defaults|upstream_config.overrides) of the downstream
		// 	  (how the downstream wants to address it)
		protocol := proxyConfGlobalProtocol

		upstreamSvcDefaults := entries.GetServiceDefaults(
			structs.NewServiceID(upstream.ID, &upstream.EnterpriseMeta),
		)
		if upstreamSvcDefaults != nil {
			if upstreamSvcDefaults.Protocol != "" {
				protocol = upstreamSvcDefaults.Protocol
			}
		}

		if protocol != "" {
			resolvedCfg["protocol"] = protocol
		}

		// Merge centralized defaults for all upstreams before configuration for specific upstreams
		if upstreamDefaults != nil {
			upstreamDefaults.MergeInto(resolvedCfg)
		}

		// The MeshGateway value from the proxy registration overrides the one from upstream_defaults
		// because it is specific to the proxy instance.
		//
		// The goal is to flatten the mesh gateway mode in this order:
		// 	0. Value from centralized upstream_defaults
		// 	1. Value from local proxy registration
		// 	2. Value from centralized upstream_config
		// 	3. Value from local upstream definition. This last step is done in the client's service manager.
		if !args.MeshGateway.IsZero() {
			resolvedCfg["mesh_gateway"] = args.MeshGateway
		}

		if upstreamConfigs[upstream] != nil {
			upstreamConfigs[upstream].MergeInto(resolvedCfg)
		}

		if len(resolvedCfg) > 0 {
			usConfigs[upstream] = resolvedCfg
		}
	}

	// don't allocate the slices just to not fill them
	if len(usConfigs) == 0 {
		return &thisReply, nil
	}

	if legacyUpstreams {
		// For legacy upstreams we return a map that is only keyed on the string ID, since they precede namespaces
		thisReply.UpstreamConfigs = make(map[string]map[string]interface{})

		for us, conf := range usConfigs {
			thisReply.UpstreamConfigs[us.ID] = conf
		}

	} else {
		thisReply.UpstreamIDConfigs = make(structs.OpaqueUpstreamConfigs, 0, len(usConfigs))

		for us, conf := range usConfigs {
			thisReply.UpstreamIDConfigs = append(thisReply.UpstreamIDConfigs,
				structs.OpaqueUpstreamConfig{Upstream: us, Config: conf})
		}
	}

	return &thisReply, nil
}
