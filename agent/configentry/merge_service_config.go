package configentry

import (
	"fmt"

	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/imdario/mergo"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

type StateStore interface {
	ReadResolvedServiceConfigEntries(memdb.WatchSet, string, *acl.EnterpriseMeta, []structs.ServiceID, structs.ProxyMode) (uint64, *ResolvedServiceConfigSet, error)
}

// MergeNodeServiceWithCentralConfig merges a service instance (NodeService) with the
// proxy-defaults/global and service-defaults/:service config entries.
// This common helper is used by the blocking query function of different RPC endpoints
// that need to return a fully resolved service defintion.
func MergeNodeServiceWithCentralConfig(
	ws memdb.WatchSet,
	state StateStore,
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

	defaults, err := ComputeResolvedServiceConfig(
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
	// So to store upstream-specific flags from central config, we add entries to ns.Proxy.Upstreams with those values.
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

		preMergeProtocol, found := us.Config["protocol"]
		// Merge in everything else that is read from the map
		if err := mergo.Merge(&us.Config, remoteCfg.Config); err != nil {
			return nil, err
		}

		// Reset the protocol to its pre-merged version for peering upstreams.
		if us.DestinationPeer != "" {
			if found {
				us.Config["protocol"] = preMergeProtocol
			} else {
				delete(us.Config, "protocol")
			}
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
