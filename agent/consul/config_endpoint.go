package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/mitchellh/copystructure"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

// The ConfigEntry endpoint is used to query centralized config information
type ConfigEntry struct {
	srv *Server
}

// Apply does an upsert of the given config entry.
func (c *ConfigEntry) Apply(args *structs.ConfigEntryRequest, reply *bool) error {
	if err := c.srv.validateEnterpriseRequest(args.Entry.GetEnterpriseMeta(), true); err != nil {
		return err
	}

	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.ForwardRPC("ConfigEntry.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "apply"}, time.Now())

	entMeta := args.Entry.GetEnterpriseMeta()
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, entMeta, nil)
	if err != nil {
		return err
	}

	// Normalize and validate the incoming config entry.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}
	if err := args.Entry.Validate(); err != nil {
		return err
	}

	if authz != nil && !args.Entry.CanWrite(authz) {
		return acl.ErrPermissionDenied
	}

	if args.Op != structs.ConfigEntryUpsert && args.Op != structs.ConfigEntryUpsertCAS {
		args.Op = structs.ConfigEntryUpsert
	}
	resp, err := c.srv.raftApply(structs.ConfigEntryRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
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

	if done, err := c.srv.ForwardRPC("ConfigEntry.Get", args, args, reply); done {
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

	if authz != nil && !lookupEntry.CanRead(authz) {
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

	if done, err := c.srv.ForwardRPC("ConfigEntry.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "list"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	if args.Kind != "" && !structs.ValidateConfigEntryKind(args.Kind) {
		return fmt.Errorf("invalid config entry kind: %s", args.Kind)
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
				if authz != nil && !entry.CanRead(authz) {
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

// ListAll returns all the known configuration entries
func (c *ConfigEntry) ListAll(args *structs.DCSpecificRequest, reply *structs.IndexedGenericConfigEntries) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.ListAll", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "listAll"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, nil)
	if err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ConfigEntries(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions.
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if authz != nil && !entry.CanRead(authz) {
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
func (c *ConfigEntry) Delete(args *structs.ConfigEntryRequest, reply *struct{}) error {
	if err := c.srv.validateEnterpriseRequest(args.Entry.GetEnterpriseMeta(), true); err != nil {
		return err
	}

	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.ForwardRPC("ConfigEntry.Delete", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "delete"}, time.Now())

	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, args.Entry.GetEnterpriseMeta(), nil)
	if err != nil {
		return err
	}

	// Normalize the incoming entry.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}

	if authz != nil && !args.Entry.CanWrite(authz) {
		return acl.ErrPermissionDenied
	}

	args.Op = structs.ConfigEntryDelete
	resp, err := c.srv.raftApply(structs.ConfigEntryRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}

// ResolveServiceConfig
func (c *ConfigEntry) ResolveServiceConfig(args *structs.ServiceConfigRequest, reply *structs.ServiceConfigResponse) error {
	if err := c.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	if done, err := c.srv.ForwardRPC("ConfigEntry.ResolveServiceConfig", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "resolve_service_config"}, time.Now())

	var authzContext acl.AuthorizerContext
	authz, err := c.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}
	if authz != nil && authz.ServiceRead(args.Name, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var thisReply structs.ServiceConfigResponse

			thisReply.MeshGateway.Mode = structs.MeshGatewayModeDefault
			// Pass the WatchSet to both the service and proxy config lookups. If either is updated
			// during the blocking query, this function will be rerun and these state store lookups
			// will both be current.
			index, serviceEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, args.Name, &args.EnterpriseMeta)
			if err != nil {
				return err
			}
			var serviceConf *structs.ServiceConfigEntry
			var ok bool
			if serviceEntry != nil {
				serviceConf, ok = serviceEntry.(*structs.ServiceConfigEntry)
				if !ok {
					return fmt.Errorf("invalid service config type %T", serviceEntry)
				}
			}

			// Use the default enterprise meta to look up the global proxy defaults. In the future we may allow per-namespace proxy-defaults
			// but not yet.
			_, proxyEntry, err := state.ConfigEntry(ws, structs.ProxyDefaults, structs.ProxyConfigGlobal, structs.DefaultEnterpriseMeta())
			if err != nil {
				return err
			}
			var proxyConf *structs.ProxyConfigEntry
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
				thisReply.MeshGateway = proxyConf.MeshGateway
				thisReply.Expose = proxyConf.Expose
			}

			thisReply.Index = index

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
			}

			// Extract the global protocol from proxyConf for upstream configs.
			var proxyConfGlobalProtocol interface{}
			if proxyConf != nil && proxyConf.Config != nil {
				proxyConfGlobalProtocol = proxyConf.Config["protocol"]
			}

			// map the legacy request structure using only service names
			// to the new ServiceID type.
			upstreamIDs := args.UpstreamIDs
			legacyUpstreams := false

			if len(upstreamIDs) == 0 {
				legacyUpstreams = true

				upstreamIDs = make([]structs.ServiceID, 0)
				for _, upstream := range args.Upstreams {
					upstreamIDs = append(upstreamIDs, structs.NewServiceID(upstream, &args.EnterpriseMeta))
				}
			}

			usConfigs := make(map[structs.ServiceID]map[string]interface{})

			for _, upstream := range upstreamIDs {
				_, upstreamEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, upstream.ID, &upstream.EnterpriseMeta)
				if err != nil {
					return err
				}
				var upstreamConf *structs.ServiceConfigEntry
				var ok bool
				if upstreamEntry != nil {
					upstreamConf, ok = upstreamEntry.(*structs.ServiceConfigEntry)
					if !ok {
						return fmt.Errorf("invalid service config type %T", upstreamEntry)
					}
				}

				// Fallback to proxyConf global protocol.
				protocol := proxyConfGlobalProtocol
				if upstreamConf != nil && upstreamConf.Protocol != "" {
					protocol = upstreamConf.Protocol
				}

				// Nothing to configure if a protocol hasn't been set.
				if protocol == nil {
					continue
				}

				usConfigs[upstream] = map[string]interface{}{
					"protocol": protocol,
				}
			}

			// don't allocate the slices just to not fill them
			if len(usConfigs) == 0 {
				*reply = thisReply
				return nil
			}

			if legacyUpstreams {
				if thisReply.UpstreamConfigs == nil {
					thisReply.UpstreamConfigs = make(map[string]map[string]interface{})
				}
				for us, conf := range usConfigs {
					thisReply.UpstreamConfigs[us.ID] = conf
				}
			} else {
				if thisReply.UpstreamIDConfigs == nil {
					thisReply.UpstreamIDConfigs = make(structs.UpstreamConfigs, 0, len(usConfigs))
				}

				for us, conf := range usConfigs {
					thisReply.UpstreamIDConfigs = append(thisReply.UpstreamIDConfigs, structs.UpstreamConfig{Upstream: us, Config: conf})
				}
			}

			*reply = thisReply
			return nil
		})
}
