// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"fmt"
	"reflect"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	memdb "github.com/hashicorp/go-memdb"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/configentry"
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

	// Log any applicable warnings about the contents of the config entry.
	if warnEntry, ok := args.Entry.(structs.WarningConfigEntry); ok {
		warnings := warnEntry.Warnings()
		for _, warning := range warnings {
			c.logger.Warn(warning)
		}
	}

	if err := args.Entry.CanWrite(authz); err != nil {
		return err
	}

	if args.Op != structs.ConfigEntryUpsert && args.Op != structs.ConfigEntryUpsertCAS {
		args.Op = structs.ConfigEntryUpsert
	}

	if skip, err := c.shouldSkipOperation(args); err != nil {
		return err
	} else if skip {
		*reply = true
		return nil
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

// shouldSkipOperation returns true if the result of the operation has
// already happened and is safe to skip.
//
// It is ok if this incorrectly detects something as changed when it
// in fact has not, the important thing is that it doesn't do
// the reverse and incorrectly detect a change as a no-op.
func (c *ConfigEntry) shouldSkipOperation(args *structs.ConfigEntryRequest) (bool, error) {
	state := c.srv.fsm.State()
	_, currentEntry, err := state.ConfigEntry(nil, args.Entry.GetKind(), args.Entry.GetName(), args.Entry.GetEnterpriseMeta())
	if err != nil {
		return false, fmt.Errorf("error reading current config entry value: %w", err)
	}

	switch args.Op {
	case structs.ConfigEntryUpsert, structs.ConfigEntryUpsertCAS:
		return c.shouldSkipUpsertOperation(currentEntry, args.Entry)
	case structs.ConfigEntryDelete, structs.ConfigEntryDeleteCAS:
		return (currentEntry == nil), nil
	default:
		return false, fmt.Errorf("invalid config entry operation type: %v", args.Op)
	}
}

func (c *ConfigEntry) shouldSkipUpsertOperation(currentEntry, updatedEntry structs.ConfigEntry) (bool, error) {
	if currentEntry == nil {
		return false, nil
	}

	if currentEntry.GetKind() != updatedEntry.GetKind() ||
		currentEntry.GetName() != updatedEntry.GetName() ||
		!currentEntry.GetEnterpriseMeta().IsSame(updatedEntry.GetEnterpriseMeta()) {
		return false, nil
	}

	// The only reason a fully Normalized and Validated config entry may
	// legitimately differ from the persisted one is due to the embedded
	// RaftIndex.
	//
	// So, to intercept more no-op upserts we temporarily set the new config
	// entry's raft index field to that of the existing data for the purposes
	// of comparison, and then restore it.
	var (
		currentRaftIndex      = currentEntry.GetRaftIndex()
		userProvidedRaftIndex = updatedEntry.GetRaftIndex()

		currentRaftIndexCopy      = *currentRaftIndex
		userProvidedRaftIndexCopy = *userProvidedRaftIndex
	)

	*userProvidedRaftIndex = currentRaftIndexCopy         // change
	same := reflect.DeepEqual(currentEntry, updatedEntry) // compare
	*userProvidedRaftIndex = userProvidedRaftIndexCopy    // restore

	return same, nil
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

	if err := lookupEntry.CanRead(authz); err != nil {
		return err
	}

	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, entry, err := state.ConfigEntry(ws, args.Kind, args.Name, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Entry = index, entry
			if entry == nil {
				return errNotFound
			}
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

	// Filtering.
	// This is only supported for certain config entries.
	var filter *bexpr.Filter
	if args.Filter != "" {
		switch args.Kind {
		case structs.ServiceDefaults:
			f, err := bexpr.CreateFilter(args.Filter, nil, []*structs.ServiceConfigEntry{})
			if err != nil {
				return err
			}
			filter = f
		default:
			return fmt.Errorf("filtering not supported for config entry kind=%v", args.Kind)
		}
	}

	var (
		priorHash uint64
		ranOnce   bool
	)
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
				if err := entry.CanRead(authz); err != nil {
					// TODO we may wish to extract more details from this error to aid user comprehension
					reply.QueryMeta.ResultsFilteredByACLs = true
					continue
				}
				filteredEntries = append(filteredEntries, entry)
			}

			reply.Kind = args.Kind
			reply.Index = index
			reply.Entries = filteredEntries

			// Generate a hash of the content driving this response. Use it to
			// determine if the response is identical to a prior wakeup.
			newHash, err := hashstructure_v2.Hash(filteredEntries, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
			}

			if filter != nil {
				raw, err := filter.Execute(reply.Entries)
				if err != nil {
					return err
				}
				reply.Entries = raw.([]structs.ConfigEntry)
			}

			if ranOnce && priorHash == newHash {
				priorHash = newHash
				return errNotChanged
			} else {
				priorHash = newHash
				ranOnce = true
			}

			if len(reply.Entries) == 0 {
				return errNotFound
			}

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
				if err := entry.CanRead(authz); err != nil {
					// TODO we may wish to extract more details from this error to aid user comprehension
					reply.QueryMeta.ResultsFilteredByACLs = true
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

	if err := args.Entry.CanWrite(authz); err != nil {
		return err
	}

	// Only delete and delete-cas ops are supported. If the caller erroneously
	// sent something else, we assume they meant delete.
	switch args.Op {
	case structs.ConfigEntryDelete, structs.ConfigEntryDeleteCAS:
	default:
		args.Op = structs.ConfigEntryDelete
	}

	if skip, err := c.shouldSkipOperation(args); err != nil {
		return err
	} else if skip {
		reply.Deleted = true
		return nil
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
	if err := authz.ToAllowAuthorizer().ServiceReadAllowed(args.Name, &authzContext); err != nil {
		return err
	}

	var (
		priorHash uint64
		ranOnce   bool
	)
	return c.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			// Fetch all relevant config entries.
			index, entries, err := state.ReadResolvedServiceConfigEntries(
				ws,
				args.Name,
				&args.EnterpriseMeta,
				args.GetLocalUpstreamIDs(),
				args.Mode,
			)
			if err != nil {
				return err
			}

			// Generate a hash of the config entry content driving this
			// response. Use it to determine if the response is identical to a
			// prior wakeup.
			newHash, err := hashstructure_v2.Hash(entries, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
			}

			if ranOnce && priorHash == newHash {
				priorHash = newHash
				reply.Index = index
				// NOTE: the prior response is still alive inside of *reply, which
				// is desirable
				return errNotChanged
			} else {
				priorHash = newHash
				ranOnce = true
			}

			thisReply, err := configentry.ComputeResolvedServiceConfig(
				args,
				entries,
				c.logger,
			)
			if err != nil {
				return err
			}
			thisReply.Index = index

			*reply = *thisReply
			if entries.IsEmpty() {
				// No config entries factored into this reply; it's a default.
				return errNotFound
			}

			return nil
		})
}

func gateWriteToSecondary(targetDC, localDC, primaryDC, kind string) error {
	// ExportedServices entries are gated from interactions from secondary DCs
	// because non-default partitions cannot be created in secondaries
	// and services cannot be exported to another datacenter.
	if kind != structs.ExportedServices {
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
		return fmt.Errorf("exported-services writes in secondary datacenters must target the primary datacenter explicitly.")

	case targetDC != "" && targetDC != primaryDC:
		return fmt.Errorf("exported-services writes must not target secondary datacenters.")

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
