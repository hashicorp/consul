package consul

import (
	"fmt"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	memdb "github.com/hashicorp/go-memdb"
)

// The ConfigEntry endpoint is used to query centralized config information
type ConfigEntry struct {
	srv *Server
}

// Apply does an upsert of the given config entry.
func (c *ConfigEntry) Apply(args *structs.ConfigEntryRequest, reply *struct{}) error {
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
	if err := verifyConfigWriteACL(rule, args.Entry.GetKind(), args.Entry.GetName()); err != nil {
		return err
	}

	args.Op = structs.ConfigEntryUpsert
	resp, err := c.srv.raftApply(structs.ConfigEntryRequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	return nil
}

// Get returns a single config entry by Kind/Name.
func (c *ConfigEntry) Get(args *structs.ConfigEntryQuery, reply *structs.IndexedConfigEntries) error {
	if done, err := c.srv.forward("ConfigEntry.Get", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"config_entry", "get"}, time.Now())

	// Fetch the ACL token, if any.
	rule, err := c.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if err := verifyConfigReadACL(rule, args.Kind, args.Name); err != nil {
		return err
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
			reply.Entries = []structs.ConfigEntry{entry}
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

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entries, err := state.ConfigEntriesByKind(ws, args.Kind)
			if err != nil {
				return err
			}

			// Filter the entries returned by ACL permissions.
			// TODO(kyhavlov): should we handle the proxy config differently here since
			// it's a singleton?
			filteredEntries := make([]structs.ConfigEntry, 0, len(entries))
			for _, entry := range entries {
				if err := verifyConfigReadACL(rule, entry.GetKind(), entry.GetName()); err != nil {
					if acl.IsErrPermissionDenied(err) {
						continue
					} else {
						return err
					}
				}
				filteredEntries = append(filteredEntries, entry)
			}

			reply.Index = index
			reply.Entries = filteredEntries
			return nil
		})
}

// Delete deletes a config entry.
func (c *ConfigEntry) Delete(args *structs.ConfigEntryRequest, reply *struct{}) error {
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
	if err := verifyConfigWriteACL(rule, args.Entry.GetKind(), args.Entry.GetName()); err != nil {
		return err
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
			// Pass the WatchSet to both the service and proxy config lookups. If either is updated
			// during the blocking query, this function will be rerun and these state store lookups
			// will both be current.
			index, serviceEntry, err := state.ConfigEntry(ws, structs.ServiceDefaults, args.Name)
			if err != nil {
				return err
			}
			serviceConf, ok := serviceEntry.(*structs.ServiceConfigEntry)
			if !ok {
				return fmt.Errorf("invalid service config type %T", serviceEntry)
			}

			_, proxyEntry, err := state.ConfigEntry(ws, structs.ProxyDefaults, structs.ProxyConfigGlobal)
			if err != nil {
				return err
			}
			proxyConf, ok := proxyEntry.(*structs.ProxyConfigEntry)
			if !ok {
				return fmt.Errorf("invalid proxy config type %T", serviceEntry)
			}

			// Resolve the service definition by overlaying the service config onto the global
			// proxy config.
			definition := structs.ServiceDefinition{
				Name: args.Name,
			}
			if proxyConf != nil {
				definition.Proxy = &structs.ConnectProxyConfig{
					Config: proxyConf.Config,
				}
			}
			if serviceConf != nil {
				definition.Name = serviceConf.Name
			}

			reply.Index = index
			reply.Definition = definition
			return nil
		})
}

// verifyConfigReadACL checks whether the given ACL authorizer has permission
// to read the config entry of the given kind/name.
func verifyConfigReadACL(rule acl.Authorizer, kind, name string) error {
	if rule == nil {
		return nil
	}

	switch kind {
	case structs.ServiceDefaults:
		if !rule.ServiceRead(name) {
			return acl.ErrPermissionDenied
		}
	case structs.ProxyDefaults:
		if !rule.OperatorRead() {
			return acl.ErrPermissionDenied
		}
	default:
		return fmt.Errorf("unknown config entry type %q", kind)
	}

	return nil
}

// verifyConfigWriteACL checks whether the given ACL authorizer has permission
// to update the config entry of the given kind/name.
func verifyConfigWriteACL(rule acl.Authorizer, kind, name string) error {
	if rule == nil {
		return nil
	}

	switch kind {
	case structs.ServiceDefaults:
		if !rule.ServiceWrite(name, nil) {
			return acl.ErrPermissionDenied
		}
	case structs.ProxyDefaults:
		if !rule.OperatorWrite() {
			return acl.ErrPermissionDenied
		}
	default:
		return fmt.Errorf("unknown config entry type %q", kind)
	}

	return nil
}
