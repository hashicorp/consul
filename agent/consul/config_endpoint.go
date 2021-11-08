package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var ConfigSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"config_entry", "apply"},
		Help: "",
	},
	{
		Name: []string{"config_entry", "get"},
		Help: "",
	},
	{
		Name: []string{"config_entry", "list"},
		Help: "",
	},
	{
		Name: []string{"config_entry", "listAll"},
		Help: "",
	},
	{
		Name: []string{"config_entry", "delete"},
		Help: "",
	},
	{
		Name: []string{"config_entry", "resolve_service_config"},
		Help: "",
	},
}

// The ConfigEntry endpoint is used to query centralized config information
type ConfigEntry struct {
	srv    *Server
	logger hclog.Logger
}

// Apply does an upsert of the given config entry.
func (c *ConfigEntry) Apply(args *structs.ConfigEntryRequest, reply *bool) error {
	if err := c.srv.validateEnterpriseRequest(args.Entry.GetEnterpriseMeta(), true); err != nil {
		return err
	}

	err := gateWriteToSecondary(args.Datacenter, c.srv.config.Datacenter, c.srv.config.PrimaryDatacenter, args.Entry.GetKind())
	if err != nil {
		return err
	}

	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.ForwardRPC("ConfigEntry.Apply", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "apply"}, time.Now())

	entMeta := args.Entry.GetEnterpriseMeta()
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, entMeta, nil)
	if err != nil {
		return err
	}

	if err := c.preflightCheck(args.Entry.GetKind()); err != nil {
		return err
	}

	// Normalize and validate the incoming config entry as if it came from a user.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}
	if err := args.Entry.Validate(); err != nil {
		return err
	}

	if !args.Entry.CanWrite(authz) {
		return acl.ErrPermissionDenied
	}

	if args.Op != structs.ConfigEntryUpsert && args.Op != structs.ConfigEntryUpsertCAS {
		args.Op = structs.ConfigEntryUpsert
	}
	resp, err := c.srv.raftApply(structs.ConfigEntryRequestType, args)
	if err != nil {
		return err
	}
	if respBool, ok := resp.(bool); ok {
		*reply = respBool
	}

	return nil
}

// Get returns a single config entry by Kind/Name.
func (c *ConfigEntry) Get(args *structs.ConfigEntryQuery, reply *structs.ConfigEntryResponse) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.Get", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "get"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	// Create a dummy config entry to check the ACL permissions.
	lookupEntry, err := structs.MakeConfigEntry(args.Kind, args.Name)
	if err != nil {
		return err
	}
	lookupEntry.GetEnterpriseMeta().Merge(&args.EnterpriseMeta)

	if !lookupEntry.CanRead(authz) {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entry, err := state.ConfigEntry(ws, args.Kind, args.Name, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index = index
			if entry == nil {
				return nil
			}

			reply.Entry = entry
			return nil
		})
}

// List returns all the config entries of the given kind. If Kind is blank,
// all existing config entries will be returned.
func (c *ConfigEntry) List(args *structs.ConfigEntryQuery, reply *structs.IndexedConfigEntries) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.List", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "list"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if args.Kind != "" {
		if _, err := structs.MakeConfigEntry(args.Kind, ""); err != nil {
			return fmt.Errorf("invalid config entry kind: %s", args.Kind)
		}
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ConfigEntriesByKind(ws, args.Kind, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions.
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if !entry.CanRead(authz) {
					continue
				}
				filteredEntries = append(filteredEntries, entry)
			}

			reply.Kind = args.Kind
			reply.Index = index
			reply.Entries = filteredEntries
			return nil
		})
}

var configEntryKindsFromConsul_1_8_0 = []string{
	structs.ServiceDefaults,
	structs.ProxyDefaults,
	structs.ServiceRouter,
	structs.ServiceSplitter,
	structs.ServiceResolver,
	structs.IngressGateway,
	structs.TerminatingGateway,
}

// ListAll returns all the known configuration entries
func (c *ConfigEntry) ListAll(args *structs.ConfigEntryListAllRequest, reply *structs.IndexedGenericConfigEntries) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.ListAll", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "listAll"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if len(args.Kinds) == 0 {
		args.Kinds = configEntryKindsFromConsul_1_8_0
	}

	kindMap := make(map[string]struct{})
	for _, kind := range args.Kinds {
		kindMap[kind] = struct{}{}
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ConfigEntries(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions or by the provided kinds.
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if !entry.CanRead(authz) {
					continue
				}
				// Doing this filter outside of memdb isn't terribly
				// performant. This kind filter is currently only used across
				// version upgrades, so in the common case we are going to
				// always return all of the data anyway, so it should be fine.
				// If that changes at some point, then we should move this down
				// into memdb.
				if _, ok := kindMap[entry.GetKind()]; !ok {
					continue
				}
				filteredEntries = append(filteredEntries, entry)
			}

			reply.Entries = filteredEntries
			reply.Index = index
			return nil
		})
}

// Delete deletes a config entry.
func (c *ConfigEntry) Delete(args *structs.ConfigEntryRequest, reply *structs.ConfigEntryDeleteResponse) error {
	if err := c.srv.validateEnterpriseRequest(args.Entry.GetEnterpriseMeta(), true); err != nil {
		return err
	}

	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.ForwardRPC("ConfigEntry.Delete", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "delete"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, args.Entry.GetEnterpriseMeta(), nil)
	if err != nil {
		return err
	}

	if err := c.preflightCheck(args.Entry.GetKind()); err != nil {
		return err
	}

	// Normalize the incoming entry.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}

	if !args.Entry.CanWrite(authz) {
		return acl.ErrPermissionDenied
	}

	// Only delete and delete-cas ops are supported. If the caller erroneously
	// sent something else, we assume they meant delete.
	switch args.Op {
	case structs.ConfigEntryDelete, structs.ConfigEntryDeleteCAS:
	default:
		args.Op = structs.ConfigEntryDelete
	}

	rsp, err := c.srv.raftApply(structs.ConfigEntryRequestType, args)
	if err != nil {
		return err
	}

	if args.Op == structs.ConfigEntryDeleteCAS {
		// In CAS deletions the FSM will return a boolean value indicating whether the
		// operation was successful.
		deleted, _ := rsp.(bool)
		reply.Deleted = deleted
	} else {
		// For non-CAS deletions any non-error result indicates a successful deletion.
		reply.Deleted = true
	}

	return nil
}

// ResolveServiceConfig
func (c *ConfigEntry) ResolveServiceConfig(args *structs.ServiceConfigRequest, reply *structs.ServiceConfigResponse) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.ResolveServiceConfig", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "resolve_service_config"}, time.Now())

	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}
	if authz.ServiceRead(args.Name, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var thisReply structs.ServiceConfigResponse

			thisReply.MeshGateway.Mode = structs.MeshGatewayModeDefault
			// TODO(freddy) Refactor this into smaller set of state store functions
			// Pass the WatchSet to both the service and proxy config lookups. If either is updated during the
			// blocking query, this function will be rerun and these state store lookups will both be current.
			// We use the default enterprise meta to look up the global proxy defaults because they are not namespaced.
			_, proxyEntry, err := state.ConfigEntry(ws, structs.ProxyDefaults, structs.ProxyConfigGlobal, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			var (
				proxyConf               *structs.ProxyConfigEntry
				proxyConfGlobalProtocol string
				ok                      bool
			)
			if proxyEntry != nil {
				proxyConf, ok = proxyEntry.(*structs.ProxyConfigEntry)
				if !ok {
					return fmt.Errorf("invalid proxy config type %T", proxyEntry)
				}
				// Apply the proxy defaults to the sidecar's proxy config
				mapCopy, err := copystructure.Copy(proxyConf.Config)
				if err != nil {
					return fmt.Errorf("failed to copy global proxy-defaults: %v", err)
				}
				thisReply.ProxyConfig = mapCopy.(map[string]interface{})
				thisReply.Mode = proxyConf.Mode
				thisReply.TransparentProxy = proxyConf.TransparentProxy
				thisReply.MeshGateway = proxyConf.MeshGateway
				thisReply.Expose = proxyConf.Expose

				// Extract the global protocol from proxyConf for upstream configs.
				rawProtocol := proxyConf.Config["protocol"]
				if rawProtocol != nil {
					proxyConfGlobalProtocol, ok = rawProtocol.(string)
					if !ok {
						return fmt.Errorf("invalid protocol type %T", rawProtocol)
					}
				}
			}

			index, serviceEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, args.Name, &args.EnterpriseMeta)
			if err != nil {
				return err
			}
			thisReply.Index = index

			var serviceConf *structs.ServiceConfigEntry
			if serviceEntry != nil {
				serviceConf, ok = serviceEntry.(*structs.ServiceConfigEntry)
				if !ok {
					return fmt.Errorf("invalid service config type %T", serviceEntry)
				}
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
			}

			// First collect all upstreams into a set of seen upstreams.
			// Upstreams can come from:
			// - Explicitly from proxy registrations, and therefore as an argument to this RPC endpoint
			// - Implicitly from centralized upstream config in service-defaults
			seenUpstreams := map[structs.ServiceID]struct{}{}

			upstreamIDs := args.UpstreamIDs
			legacyUpstreams := false

			var (
				noUpstreamArgs = len(upstreamIDs) == 0 && len(args.Upstreams) == 0

				// Check the args and the resolved value. If it was exclusively set via a config entry, then args.Mode
				// will never be transparent because the service config request does not use the resolved value.
				tproxy = args.Mode == structs.ProxyModeTransparent || thisReply.Mode == structs.ProxyModeTransparent
			)

			// The upstreams passed as arguments to this endpoint are the upstreams explicitly defined in a proxy registration.
			// If no upstreams were passed, then we should only returned the resolved config if the proxy in transparent mode.
			// Otherwise we would return a resolved upstream config to a proxy with no configured upstreams.
			if noUpstreamArgs && !tproxy {
				*reply = thisReply
				return nil
			}

			// The request is considered legacy if the deprecated args.Upstream was used
			if len(upstreamIDs) == 0 && len(args.Upstreams) > 0 {
				legacyUpstreams = true

				upstreamIDs = make([]structs.ServiceID, 0)
				for _, upstream := range args.Upstreams {
					// Before Consul namespaces were released, the Upstreams provided to the endpoint did not contain the namespace.
					// Because of this we attach the enterprise meta of the request, which will just be the default namespace.
					sid := structs.NewServiceID(upstream, &args.EnterpriseMeta)
					upstreamIDs = append(upstreamIDs, sid)
				}
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
						c.logger.Warn(
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

				_, upstreamSvcDefaults, err := state.ConfigEntry(ws, structs.ServiceDefaults, upstream.ID, &upstream.EnterpriseMeta)
				if err != nil {
					return err
				}
				if upstreamSvcDefaults != nil {
					cfg, ok := upstreamSvcDefaults.(*structs.ServiceConfigEntry)
					if !ok {
						return fmt.Errorf("invalid service config type %T", upstreamSvcDefaults)
					}
					if cfg.Protocol != "" {
						protocol = cfg.Protocol
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
				*reply = thisReply
				return nil
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

			*reply = thisReply
			return nil
		})
}

func gateWriteToSecondary(targetDC, localDC, primaryDC, kind string) error {
	// Partition exports are gated from interactions from secondary DCs
	// because non-default partitions cannot be created in secondaries
	// and services cannot be exported to another datacenter.
	if kind != structs.PartitionExports {
		return nil
	}
	if localDC == "" {
		// This should not happen because the datacenter is defaulted in DefaultConfig.
		return fmt.Errorf("unknown local datacenter")
	}

	if primaryDC == "" {
		primaryDC = localDC
	}

	switch {
	case targetDC == "" && localDC != primaryDC:
		return fmt.Errorf("partition-exports writes in secondary datacenters must target the primary datacenter explicitly.")

	case targetDC != "" && targetDC != primaryDC:
		return fmt.Errorf("partition-exports writes must not target secondary datacenters.")

	}
	return nil
}

// preflightCheck is meant to have kind-specific system validation outside of
// content validation. The initial use case is restricting the ability to do
// writes of service-intentions until the system is finished migration.
func (c *ConfigEntry) preflightCheck(kind string) error {
	switch kind {
	case structs.ServiceIntentions:
		// Exit early if Connect hasn't been enabled.
		if !c.srv.config.ConnectEnabled {
			return ErrConnectNotEnabled
		}

		usingConfigEntries, err := c.srv.fsm.State().AreIntentionsInConfigEntries()
		if err != nil {
			return fmt.Errorf("system metadata lookup failed: %v", err)
		}
		if !usingConfigEntries {
			return ErrIntentionsNotUpgradedYet
		}
	}

	return nil
}
