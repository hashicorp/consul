package consul

import (
	"fmt"

	"github.com/hashicorp/consul/agent/configentry"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"
)

// mergeNodeServiceWithCentralConfig merges a service instance (NodeService) with the
// proxy-defaults/global and service-defaults/:service config entries.
// This common helper is used by the blocking query function of different RPC endpoints
// that need to return a fully resolved service defintion.
func mergeNodeServiceWithCentralConfig(
	ws memdb.WatchSet,
	state *state.Store,
	args *structs.ServiceSpecificRequest,
	ns *structs.NodeService,
	logger hclog.Logger) (uint64, *structs.NodeService, error) {

	serviceName := ns.Service
	var upstreams []structs.ServiceID
	if ns.IsSidecarProxy() {
		// This is a sidecar proxy, ignore the proxy service's config since we are
		// managed by the target service config.
		serviceName = ns.Proxy.DestinationServiceName

		// Also if we have any upstreams defined, add them to the defaults lookup request
		// so we can learn about their configs.
		for _, us := range ns.Proxy.Upstreams {
			if us.DestinationType == "" || us.DestinationType == structs.UpstreamDestTypeService {
				sid := us.DestinationID()
				sid.EnterpriseMeta.Merge(&ns.EnterpriseMeta)
				upstreams = append(upstreams, sid)
			}
		}
	}

	configReq := &structs.ServiceConfigRequest{
		Name:           serviceName,
		Datacenter:     args.Datacenter,
		QueryOptions:   args.QueryOptions,
		MeshGateway:    ns.Proxy.MeshGateway,
		Mode:           ns.Proxy.Mode,
		UpstreamIDs:    upstreams,
		EnterpriseMeta: ns.EnterpriseMeta,
	}

	// prefer using this vs directly calling the ConfigEntry.ResolveServiceConfig RPC
	// so as to pass down the same watch set to also watch on changes to
	// proxy-defaults/global and service-defaults.
	cfgIndex, configEntries, err := state.ReadResolvedServiceConfigEntries(
		ws,
		configReq.Name,
		&configReq.EnterpriseMeta,
		upstreams,
		configReq.Mode,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure looking up service config entries for %s: %v",
			ns.ID, err)
	}

	defaults, err := computeResolvedServiceConfig(
		configReq,
		upstreams,
		false,
		configEntries,
		logger,
	)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure computing service defaults for %s: %v",
			ns.ID, err)
	}

	mergedns, err := MergeServiceConfig(defaults, ns)
	if err != nil {
		return 0, nil, fmt.Errorf("Failure merging service definition with config entry defaults for %s: %v",
			ns.ID, err)
	}

	return cfgIndex, mergedns, nil
}

func computeResolvedServiceConfig(
	args *structs.ServiceConfigRequest,
	upstreamIDs []structs.ServiceID,
	legacyUpstreams bool,
	entries *configentry.ResolvedServiceConfigSet,
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

// MergeServiceConfig merges the service into defaults to produce the final effective
// config for the specified service.
func MergeServiceConfig(defaults *structs.ServiceConfigResponse, service *structs.NodeService) (*structs.NodeService, error) {
	if defaults == nil {
		return service, nil
	}

	// We don't want to change s.registration in place since it is our source of
	// truth about what was actually registered before defaults applied. So copy
	// it first.
	nsRaw, err := copystructure.Copy(service)
	if err != nil {
		return nil, err
	}

	// Merge proxy defaults
	ns := nsRaw.(*structs.NodeService)

	if err := mergo.Merge(&ns.Proxy.Config, defaults.ProxyConfig); err != nil {
		return nil, err
	}
	if err := mergo.Merge(&ns.Proxy.Expose, defaults.Expose); err != nil {
		return nil, err
	}

	if ns.Proxy.MeshGateway.Mode == structs.MeshGatewayModeDefault {
		ns.Proxy.MeshGateway.Mode = defaults.MeshGateway.Mode
	}
	if ns.Proxy.Mode == structs.ProxyModeDefault {
		ns.Proxy.Mode = defaults.Mode
	}
	if ns.Proxy.TransparentProxy.OutboundListenerPort == 0 {
		ns.Proxy.TransparentProxy.OutboundListenerPort = defaults.TransparentProxy.OutboundListenerPort
	}
	if !ns.Proxy.TransparentProxy.DialedDirectly {
		ns.Proxy.TransparentProxy.DialedDirectly = defaults.TransparentProxy.DialedDirectly
	}

	// remoteUpstreams contains synthetic Upstreams generated from central config (service-defaults.UpstreamConfigs).
	remoteUpstreams := make(map[structs.ServiceID]structs.Upstream)

	for _, us := range defaults.UpstreamIDConfigs {
		parsed, err := structs.ParseUpstreamConfigNoDefaults(us.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse upstream config map for %s: %v", us.Upstream.String(), err)
		}

		remoteUpstreams[us.Upstream] = structs.Upstream{
			DestinationNamespace: us.Upstream.NamespaceOrDefault(),
			DestinationPartition: us.Upstream.PartitionOrDefault(),
			DestinationName:      us.Upstream.ID,
			Config:               us.Config,
			MeshGateway:          parsed.MeshGateway,
			CentrallyConfigured:  true,
		}
	}

	// localUpstreams stores the upstreams seen from the local registration so that we can merge in the synthetic entries.
	// In transparent proxy mode ns.Proxy.Upstreams will likely be empty because users do not need to define upstreams explicitly.
	// So to store upstream-specific flags from central config, we add entries to ns.Proxy.Upstream with those values.
	localUpstreams := make(map[structs.ServiceID]struct{})

	// Merge upstream defaults into the local registration
	for i := range ns.Proxy.Upstreams {
		// Get a pointer not a value copy of the upstream struct
		us := &ns.Proxy.Upstreams[i]
		if us.DestinationType != "" && us.DestinationType != structs.UpstreamDestTypeService {
			continue
		}
		localUpstreams[us.DestinationID()] = struct{}{}

		remoteCfg, ok := remoteUpstreams[us.DestinationID()]
		if !ok {
			// No config defaults to merge
			continue
		}

		// The local upstream config mode has the highest precedence, so only overwrite when it's set to the default
		if us.MeshGateway.Mode == structs.MeshGatewayModeDefault {
			us.MeshGateway.Mode = remoteCfg.MeshGateway.Mode
		}

		// Merge in everything else that is read from the map
		if err := mergo.Merge(&us.Config, remoteCfg.Config); err != nil {
			return nil, err
		}

		// Delete the mesh gateway key from opaque config since this is the value that was resolved from
		// the servers and NOT the final merged value for this upstream.
		// Note that we use the "mesh_gateway" key and not other variants like "MeshGateway" because
		// UpstreamConfig.MergeInto and ResolveServiceConfig only use "mesh_gateway".
		delete(us.Config, "mesh_gateway")
	}

	// Ensure upstreams present in central config are represented in the local configuration.
	// This does not apply outside of transparent mode because in that situation every possible upstream already exists
	// inside of ns.Proxy.Upstreams.
	if ns.Proxy.Mode == structs.ProxyModeTransparent {
		for id, remote := range remoteUpstreams {
			if _, ok := localUpstreams[id]; ok {
				// Remote upstream is already present locally
				continue
			}

			ns.Proxy.Upstreams = append(ns.Proxy.Upstreams, remote)
		}
	}

	return ns, err
}
