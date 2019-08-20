package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
	"github.com/mitchellh/copystructure"
)

// The ConfigEntry endpoint is used to query centralized config information
type ConfigEntry struct {
	srv *Server
}

// Apply does an upsert of the given config entry.
func (c *ConfigEntry) Apply(args *structs.ConfigEntryRequest, reply *bool) error {
	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.forward("ConfigEntry.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "apply"}, time.Now())

	// Normalize and validate the incoming config entry.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}
	if err := args.Entry.Validate(); err != nil {
		return err
	}

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !args.Entry.CanWrite(rule) {
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
	if done, err := c.srv.forward("ConfigEntry.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "get"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	// Create a dummy config entry to check the ACL permissions.
	lookupEntry, err := structs.MakeConfigEntry(args.Kind, args.Name)
	if err != nil {
		return err
	}
	if rule != nil && !lookupEntry.CanRead(rule) {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entry, err := state.ConfigEntry(ws, args.Kind, args.Name)
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
	if done, err := c.srv.forward("ConfigEntry.List", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "list"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
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
			index, entries, err := state.ConfigEntriesByKind(ws, args.Kind)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions.
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if rule != nil && !entry.CanRead(rule) {
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
	if done, err := c.srv.forward("ConfigEntry.ListAll", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "listAll"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ConfigEntries(ws)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions.
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if rule != nil && !entry.CanRead(rule) {
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
	// Ensure that all config entry writes go to the primary datacenter. These will then
	// be replicated to all the other datacenters.
	args.Datacenter = c.srv.config.PrimaryDatacenter

	if done, err := c.srv.forward("ConfigEntry.Delete", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "delete"}, time.Now())

	// Normalize the incoming entry.
	if err := args.Entry.Normalize(); err != nil {
		return err
	}

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !args.Entry.CanWrite(rule) {
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
	if done, err := c.srv.forward("ConfigEntry.ResolveServiceConfig", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "resolve_service_config"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.ServiceRead(args.Name) {
		return acl.ErrPermissionDenied
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			reply.Reset()

			reply.MeshGateway.Mode = structs.MeshGatewayModeDefault
			// Pass the WatchSet to both the service and proxy config lookups. If either is updated
			// during the blocking query, this function will be rerun and these state store lookups
			// will both be current.
			index, serviceEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, args.Name)
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

			_, proxyEntry, err := state.ConfigEntry(ws, structs.ProxyDefaults, structs.ProxyConfigGlobal)
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
				reply.ProxyConfig = mapCopy.(map[string]interface{})
				reply.MeshGateway = proxyConf.MeshGateway
			}

			reply.Index = index

			if serviceConf != nil {
				if serviceConf.MeshGateway.Mode != structs.MeshGatewayModeDefault {
					reply.MeshGateway.Mode = serviceConf.MeshGateway.Mode
				}
				if serviceConf.Protocol != "" {
					if reply.ProxyConfig == nil {
						reply.ProxyConfig = make(map[string]interface{})
					}
					reply.ProxyConfig["protocol"] = serviceConf.Protocol
				}
			}

			// Extract the global protocol from proxyConf for upstream configs.
			var proxyConfGlobalProtocol interface{}
			if proxyConf != nil && proxyConf.Config != nil {
				proxyConfGlobalProtocol = proxyConf.Config["protocol"]
			}

			// Apply the upstream protocols to the upstream configs
			for _, upstream := range args.Upstreams {
				_, upstreamEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, upstream)
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

				// No upstream found; skip.
				if upstreamConf == nil {
					continue
				}

				// Fallback to proxyConf global protocol.
				protocol := proxyConfGlobalProtocol
				if upstreamConf.Protocol != "" {
					protocol = upstreamConf.Protocol
				}

				// Nothing to configure if a protocol hasn't been set.
				if protocol == nil {
					continue
				}

				if reply.UpstreamConfigs == nil {
					reply.UpstreamConfigs = make(map[string]map[string]interface{})
				}
				reply.UpstreamConfigs[upstream] = map[string]interface{}{
					"protocol": protocol,
				}
			}

			return nil
		})
}
