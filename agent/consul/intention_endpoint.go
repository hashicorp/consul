// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	hashstructure_v2 "github.com/mitchellh/hashstructure/v2"

	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
)

var IntentionSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"consul", "intention", "apply"},
		Help: "Deprecated - please use intention_apply",
	},
	{
		Name: []string{"intention", "apply"},
		Help: "",
	},
}

var (
	// ErrIntentionNotFound is returned if the intention lookup failed.
	ErrIntentionNotFound = errors.New("Intention not found")
)

// Intention manages the Connect intentions.
type Intention struct {
	// srv is a pointer back to the server.
	srv    *Server
	logger hclog.Logger
}

func (s *Intention) checkIntentionID(id string) (bool, error) {
	state := s.srv.fsm.State()
	if _, _, ixn, err := state.IntentionGet(nil, id); err != nil {
		return false, err
	} else if ixn != nil {
		return false, nil
	}

	return true, nil
}

var ErrIntentionsNotUpgradedYet = errors.New("Intentions are read only while being upgraded to config entries")

// legacyUpgradeCheck fast fails a write request using the legacy intention
// RPCs if the system is known to be mid-upgrade. This is purely a perf
// optimization and the actual real enforcement happens in the FSM. It would be
// wasteful to round trip all the way through raft to have it fail for
// known-up-front reasons, hence why we check it twice.
func (s *Intention) legacyUpgradeCheck() error {
	usingConfigEntries, err := s.srv.fsm.State().AreIntentionsInConfigEntries()
	if err != nil {
		return fmt.Errorf("system metadata lookup failed: %v", err)
	}
	if !usingConfigEntries {
		return ErrIntentionsNotUpgradedYet
	}
	return nil
}

// Apply creates or updates an intention in the data store.
func (s *Intention) Apply(args *structs.IntentionRequest, reply *string) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if args.Intention != nil && args.Intention.SourcePeer != "" {
		return fmt.Errorf("SourcePeer field is not supported on this endpoint. Use config entries instead")
	}

	// Ensure that all service-intentions config entry writes go to the primary
	// datacenter. These will then be replicated to all the other datacenters.
	args.Datacenter = s.srv.config.PrimaryDatacenter

	if done, err := s.srv.ForwardRPC("Intention.Apply", args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "intention", "apply"}, time.Now())
	defer metrics.MeasureSince([]string{"intention", "apply"}, time.Now())

	if err := s.legacyUpgradeCheck(); err != nil {
		return err
	}

	if args.Mutation != nil {
		return fmt.Errorf("Mutation field is internal only and must not be set via RPC")
	}

	// Always set a non-nil intention to avoid nil-access below
	if args.Intention == nil {
		args.Intention = &structs.Intention{}
	}

	// Get the ACL token for the request for the checks below.
	var entMeta acl.EnterpriseMeta
	authz, err := s.srv.ACLResolver.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	accessorID := authz.AccessorID()
	var (
		mut         *structs.IntentionMutation
		legacyWrite bool
	)
	switch args.Op {
	case structs.IntentionOpCreate:
		legacyWrite = true
		mut, err = s.computeApplyChangesLegacyCreate(accessorID, authz, &entMeta, args)
	case structs.IntentionOpUpdate:
		legacyWrite = true
		mut, err = s.computeApplyChangesLegacyUpdate(accessorID, authz, &entMeta, args)
	case structs.IntentionOpUpsert:
		legacyWrite = false
		mut, err = s.computeApplyChangesUpsert(accessorID, authz, &entMeta, args)
	case structs.IntentionOpDelete:
		if args.Intention.ID == "" {
			legacyWrite = false
			mut, err = s.computeApplyChangesDelete(accessorID, authz, &entMeta, args)
		} else {
			legacyWrite = true
			mut, err = s.computeApplyChangesLegacyDelete(accessorID, authz, &entMeta, args)
		}
	case structs.IntentionOpDeleteAll:
		// This is an internal operation initiated by the leader and is not
		// exposed for general RPC use.
		return fmt.Errorf("Invalid Intention operation: %v", args.Op)
	default:
		return fmt.Errorf("Invalid Intention operation: %v", args.Op)
	}

	if err != nil {
		return err
	}
	if mut == nil {
		return nil // short circuit
	}

	if legacyWrite {
		*reply = args.Intention.ID
	} else {
		*reply = ""
	}

	// Switch to the config entry manipulating flavor:
	args.Mutation = mut
	args.Intention = nil

	_, err = s.srv.raftApply(structs.IntentionRequestType, args)
	return err
}

func (s *Intention) computeApplyChangesLegacyCreate(
	accessorID string,
	authz acl.Authorizer,
	entMeta *acl.EnterpriseMeta,
	args *structs.IntentionRequest,
) (*structs.IntentionMutation, error) {
	// This variant is just for legacy UUID-based intentions.

	args.Intention.FillPartitionAndNamespace(entMeta, true)

	if !args.Intention.CanWrite(authz) {
		sn := args.Intention.SourceServiceName()
		dn := args.Intention.DestinationServiceName()
		s.logger.Debug("Intention creation denied due to ACLs",
			"source", sn.String(),
			"destination", dn.String(),
			"accessorID", acl.AliasIfAnonymousToken(accessorID))
		return nil, acl.ErrPermissionDenied
	}

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Intention.ID != "" {
		return nil, fmt.Errorf("ID must be empty when creating a new intention")
	}

	var err error
	args.Intention.ID, err = lib.GenerateUUID(s.checkIntentionID)
	if err != nil {
		return nil, err
	}
	// Set the created at
	args.Intention.CreatedAt = time.Now().UTC()
	args.Intention.UpdatedAt = args.Intention.CreatedAt

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	if err := s.validateEnterpriseIntention(args.Intention); err != nil {
		return nil, err
	}

	//nolint:staticcheck
	if err := args.Intention.Validate(); err != nil {
		return nil, err
	}

	// NOTE: if the append of this source causes a duplicate source name the
	// config entry validation will fail so we don't have to check that
	// explicitly here.

	mut := &structs.IntentionMutation{
		Destination: args.Intention.DestinationServiceName(),
		Value:       args.Intention.ToSourceIntention(true),
	}

	// Set the created/updated times. If this is an update instead of an insert
	// the UpdateOver() will fix it up appropriately.
	now := time.Now().UTC()
	mut.Value.LegacyCreateTime = timePointer(now)
	mut.Value.LegacyUpdateTime = timePointer(now)

	return mut, nil
}

func (s *Intention) computeApplyChangesLegacyUpdate(
	accessorID string,
	authz acl.Authorizer,
	entMeta *acl.EnterpriseMeta,
	args *structs.IntentionRequest,
) (*structs.IntentionMutation, error) {
	// This variant is just for legacy UUID-based intentions.

	_, _, ixn, err := s.srv.fsm.State().IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return nil, fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil {
		return nil, fmt.Errorf("Cannot modify non-existent intention: '%s'", args.Intention.ID)
	}

	if !ixn.CanWrite(authz) {
		s.logger.Debug("Update operation on intention denied due to ACLs",
			"intention", args.Intention.ID,
			"accessorID", acl.AliasIfAnonymousToken(accessorID))
		return nil, acl.ErrPermissionDenied
	}

	args.Intention.FillPartitionAndNamespace(entMeta, true)

	// Prior to v1.9.0 renames of the destination side of an intention were
	// allowed, but that behavior doesn't work anymore.
	if ixn.DestinationServiceName() != args.Intention.DestinationServiceName() {
		return nil, fmt.Errorf("Cannot modify Destination partition/namespace/name for an intention once it exists.")
	}

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	if err := s.validateEnterpriseIntention(args.Intention); err != nil {
		return nil, err
	}

	// Validate. We do not validate on delete since it is valid to only
	// send an ID in that case.
	//nolint:staticcheck
	if err := args.Intention.Validate(); err != nil {
		return nil, err
	}

	mut := &structs.IntentionMutation{
		ID:    args.Intention.ID,
		Value: args.Intention.ToSourceIntention(true),
	}

	// Set the created/updated times. If this is an update instead of an insert
	// the UpdateOver() will fix it up appropriately.
	now := time.Now().UTC()
	mut.Value.LegacyCreateTime = timePointer(now)
	mut.Value.LegacyUpdateTime = timePointer(now)

	return mut, nil
}

func (s *Intention) computeApplyChangesUpsert(
	accessorID string,
	authz acl.Authorizer,
	entMeta *acl.EnterpriseMeta,
	args *structs.IntentionRequest,
) (*structs.IntentionMutation, error) {
	// This variant is just for config-entry based intentions.

	if args.Intention.ID != "" {
		// This is a new-style only endpoint
		return nil, fmt.Errorf("ID must not be specified")
	}

	args.Intention.FillPartitionAndNamespace(entMeta, true)

	if !args.Intention.CanWrite(authz) {
		sn := args.Intention.SourceServiceName()
		dn := args.Intention.DestinationServiceName()
		s.logger.Debug("Intention upsert denied due to ACLs",
			"source", sn.String(),
			"destination", dn.String(),
			"accessorID", acl.AliasIfAnonymousToken(accessorID))
		return nil, acl.ErrPermissionDenied
	}

	_, prevEntry, err := s.srv.fsm.State().ConfigEntry(nil, structs.ServiceIntentions, args.Intention.DestinationName, args.Intention.DestinationEnterpriseMeta())
	if err != nil {
		return nil, fmt.Errorf("Intention lookup failed: %v", err)
	}

	if prevEntry == nil {
		// Meta is NOT permitted here, as it would need to be persisted on
		// the enclosing config entry.
		if len(args.Intention.Meta) > 0 {
			return nil, fmt.Errorf("Meta must not be specified")
		}
	} else {
		if len(args.Intention.Meta) > 0 {
			// Meta is NOT permitted here, but there is one exception. If
			// you are updating a previous record, but that record lives
			// within a config entry that itself has Meta, then you may
			// incidentally ship the Meta right back to consul.
			//
			// In that case if Meta is provided, it has to be a perfect
			// match for what is already on the enclosing config entry so
			// it's safe to discard.
			if !equalStringMaps(prevEntry.GetMeta(), args.Intention.Meta) {
				return nil, fmt.Errorf("Meta must not be specified, or should be unchanged during an update.")
			}

			// Now it is safe to discard
			args.Intention.Meta = nil
		}
	}

	return &structs.IntentionMutation{
		Destination: args.Intention.DestinationServiceName(),
		Source:      args.Intention.SourceServiceName(),
		Value:       args.Intention.ToSourceIntention(false),
	}, nil
}

func (s *Intention) computeApplyChangesLegacyDelete(
	accessorID string,
	authz acl.Authorizer,
	entMeta *acl.EnterpriseMeta,
	args *structs.IntentionRequest,
) (*structs.IntentionMutation, error) {
	_, _, ixn, err := s.srv.fsm.State().IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return nil, fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil {
		return nil, fmt.Errorf("Cannot delete non-existent intention: '%s'", args.Intention.ID)
	}

	if !ixn.CanWrite(authz) {
		s.logger.Debug("Deletion operation on intention denied due to ACLs",
			"intention", args.Intention.ID,
			"accessorID", acl.AliasIfAnonymousToken(accessorID))
		return nil, acl.ErrPermissionDenied
	}

	return &structs.IntentionMutation{
		ID: args.Intention.ID,
	}, nil
}

func (s *Intention) computeApplyChangesDelete(
	accessorID string,
	authz acl.Authorizer,
	entMeta *acl.EnterpriseMeta,
	args *structs.IntentionRequest,
) (*structs.IntentionMutation, error) {
	args.Intention.FillPartitionAndNamespace(entMeta, true)

	if !args.Intention.CanWrite(authz) {
		sn := args.Intention.SourceServiceName()
		dn := args.Intention.DestinationServiceName()
		s.logger.Debug("Intention delete denied due to ACLs",
			"source", sn.String(),
			"destination", dn.String(),
			"accessorID", acl.AliasIfAnonymousToken(accessorID))
		return nil, acl.ErrPermissionDenied
	}

	// Pre-flight to avoid pointless raft operations.
	exactIxn := args.Intention.ToExact()
	_, _, ixn, err := s.srv.fsm.State().IntentionGetExact(nil, exactIxn)
	if err != nil {
		return nil, fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil {
		return nil, nil // by-name deletions are idempotent
	}

	return &structs.IntentionMutation{
		Destination: args.Intention.DestinationServiceName(),
		Source:      args.Intention.SourceServiceName(),
	}, nil
}

// Get returns a single intention by ID.
func (s *Intention) Get(args *structs.IntentionQueryRequest, reply *structs.IndexedIntentions) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.Get", args, reply); done {
		return err
	}

	// Get the ACL token for the request for the checks below.
	var entMeta acl.EnterpriseMeta
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	if args.Exact != nil {
		// Finish defaulting the namespace fields.
		if args.Exact.SourceNS == "" {
			args.Exact.SourceNS = entMeta.NamespaceOrDefault()
		}
		if err := s.srv.validateEnterpriseIntentionNamespace(args.Exact.SourceNS, true); err != nil {
			return fmt.Errorf("Invalid SourceNS %q: %v", args.Exact.SourceNS, err)
		}

		if args.Exact.DestinationNS == "" {
			args.Exact.DestinationNS = entMeta.NamespaceOrDefault()
		}
		if err := s.srv.validateEnterpriseIntentionNamespace(args.Exact.DestinationNS, true); err != nil {
			return fmt.Errorf("Invalid DestinationNS %q: %v", args.Exact.DestinationNS, err)
		}
	}

	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var (
				index uint64
				ixn   *structs.Intention
				err   error
			)
			if args.IntentionID != "" {
				index, _, ixn, err = state.IntentionGet(ws, args.IntentionID)
			} else if args.Exact != nil {
				index, _, ixn, err = state.IntentionGetExact(ws, args.Exact)
			}

			if err != nil {
				return err
			}
			if ixn == nil {
				return ErrIntentionNotFound
			}

			reply.Index = index
			reply.Intentions = structs.Intentions{ixn}

			// Filter
			s.srv.filterACLWithAuthorizer(authz, reply)

			// If ACLs prevented any responses, error
			if len(reply.Intentions) == 0 {
				accessorID := authz.AccessorID()
				s.logger.Debug("Request to get intention denied due to ACLs",
					"intention", args.IntentionID,
					"accessorID", acl.AliasIfAnonymousToken(accessorID))
				return acl.ErrPermissionDenied
			}

			return nil
		},
	)
}

// List returns all the intentions.
func (s *Intention) List(args *structs.IntentionListRequest, reply *structs.IndexedIntentions) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.List", args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Intentions)
	if err != nil {
		return err
	}

	var authzContext acl.AuthorizerContext
	if _, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext); err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var (
				index      uint64
				ixns       structs.Intentions
				fromConfig bool
				err        error
			)
			if args.Legacy {
				index, ixns, err = state.LegacyIntentions(ws, &args.EnterpriseMeta)
			} else {
				index, ixns, fromConfig, err = state.Intentions(ws, &args.EnterpriseMeta)
			}
			if err != nil {
				return err
			}

			reply.Index, reply.Intentions = index, ixns
			if reply.Intentions == nil {
				reply.Intentions = make(structs.Intentions, 0)
			}

			if fromConfig {
				reply.DataOrigin = structs.IntentionDataOriginConfigEntries
			} else {
				reply.DataOrigin = structs.IntentionDataOriginLegacy
			}
			raw, err := filter.Execute(reply.Intentions)
			if err != nil {
				return err
			}
			reply.Intentions = raw.(structs.Intentions)

			// Note: we filter the results with ACLs *after* applying the user-supplied
			// bexpr filter, to ensure QueryMeta.ResultsFilteredByACLs does not include
			// results that would be filtered out even if the user did have permission.
			if err := s.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		},
	)
}

// Match returns the set of intentions that match the given source/destination.
func (s *Intention) Match(args *structs.IntentionQueryRequest, reply *structs.IndexedIntentionMatches) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.Match", args, reply); done {
		return err
	}

	// Get the ACL token for the request for the checks below.
	var entMeta acl.EnterpriseMeta
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	// Finish defaulting the namespace and partition fields.
	for i := range args.Match.Entries {
		if args.Match.Entries[i].Namespace == "" {
			args.Match.Entries[i].Namespace = entMeta.NamespaceOrDefault()
		}
		if err := s.srv.validateEnterpriseIntentionNamespace(args.Match.Entries[i].Namespace, true); err != nil {
			return fmt.Errorf("Invalid match entry namespace %q: %v",
				args.Match.Entries[i].Namespace, err)
		}

		if args.Match.Entries[i].Partition == "" {
			args.Match.Entries[i].Partition = entMeta.PartitionOrDefault()
		}
		if err := s.srv.validateEnterpriseIntentionPartition(args.Match.Entries[i].Partition); err != nil {
			return fmt.Errorf("Invalid match entry partition %q: %v",
				args.Match.Entries[i].Partition, err)
		}
	}

	var authzContext acl.AuthorizerContext
	// Go through each entry to ensure we have intentions:read for the resource.

	// TODO - should we do this instead of filtering the result set? This will only allow
	// queries for which the token has intentions:read permissions on the requested side
	// of the service. Should it instead return all matches that it would be able to list.
	// if so we should remove this and call filterACL instead. Based on how this is used
	// its probably fine. If you have intention read on the source just do a source type
	// matching, if you have it on the dest then perform a dest type match.
	for _, entry := range args.Match.Entries {
		entry.FillAuthzContext(&authzContext)
		if prefix := entry.Name; prefix != "" {
			if err := authz.ToAllowAuthorizer().IntentionReadAllowed(prefix, &authzContext); err != nil {
				accessorID := authz.AccessorID()
				s.logger.Debug("Operation on intention prefix denied due to ACLs",
					"prefix", prefix,
					"accessorID", acl.AliasIfAnonymousToken(accessorID))
				return err
			}
		}
	}

	var (
		priorHash uint64
		ranOnce   bool
	)
	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, matches, err := state.IntentionMatch(ws, args.Match)
			if err != nil {
				return err
			}

			reply.Index = index
			reply.Matches = matches

			// Generate a hash of the intentions content driving this response.
			// Use it to determine if the response is identical to a prior
			// wakeup.
			newHash, err := hashstructure_v2.Hash(matches, hashstructure_v2.FormatV2, nil)
			if err != nil {
				return fmt.Errorf("error hashing reply for spurious wakeup suppression: %w", err)
			}

			if ranOnce && priorHash == newHash {
				priorHash = newHash
				return errNotChanged
			} else {
				priorHash = newHash
				ranOnce = true
			}

			hasData := false
			for _, match := range matches {
				if len(match) > 0 {
					hasData = true
					break
				}
			}

			if !hasData {
				return errNotFound
			}

			return nil
		},
	)
}

// Check tests a source/destination and returns whether it would be allowed
// or denied based on the current ACL configuration.
//
// NOTE: This endpoint treats any L7 intentions as DENY.
//
// Note: Whenever the logic for this method is changed, you should take
// a look at the agent authorize endpoint (agent/agent_endpoint.go) since
// the logic there is similar.
func (s *Intention) Check(args *structs.IntentionQueryRequest, reply *structs.IntentionQueryCheckResponse) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	// Forward maybe
	if done, err := s.srv.ForwardRPC("Intention.Check", args, reply); done {
		return err
	}

	// Get the test args, and defensively guard against nil
	query := args.Check
	if query == nil {
		return errors.New("Check must be specified on args")
	}

	// Get the ACL token for the request for the checks below.
	var entMeta acl.EnterpriseMeta
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	// Finish defaulting the namespace fields.
	if query.SourceNS == "" {
		query.SourceNS = entMeta.NamespaceOrDefault()
	}
	if query.DestinationNS == "" {
		query.DestinationNS = entMeta.NamespaceOrDefault()
	}
	if query.SourcePartition == "" {
		query.SourcePartition = entMeta.PartitionOrDefault()
	}
	if query.DestinationPartition == "" {
		query.DestinationPartition = entMeta.PartitionOrDefault()
	}

	if err := s.srv.validateEnterpriseIntentionNamespace(query.SourceNS, false); err != nil {
		return fmt.Errorf("Invalid source namespace %q: %v", query.SourceNS, err)
	}
	if err := s.srv.validateEnterpriseIntentionNamespace(query.DestinationNS, false); err != nil {
		return fmt.Errorf("Invalid destination namespace %q: %v", query.DestinationNS, err)
	}

	if query.SourceType != structs.IntentionSourceConsul {
		return fmt.Errorf("unsupported SourceType: %q", query.SourceType)
	}

	// Perform the ACL check. For Check we only require ServiceRead and
	// NOT IntentionRead because the Check API only returns pass/fail and
	// returns no other information about the intentions used. We could check
	// both the source and dest side but only checking dest also has the nice
	// benefit of only returning a passing status if the token would be able
	// to discover the dest service and connect to it.
	if prefix, ok := query.GetACLPrefix(); ok {
		var authzContext acl.AuthorizerContext
		query.FillAuthzContext(&authzContext)
		if err := authz.ToAllowAuthorizer().ServiceReadAllowed(prefix, &authzContext); err != nil {
			accessorID := authz.AccessorID()
			s.logger.Debug("test on intention denied due to ACLs",
				"prefix", prefix,
				"accessorID", acl.AliasIfAnonymousToken(accessorID))
			return err
		}
	}

	// Note: the default intention policy is like an intention with a
	// wildcarded destination in that it is limited to L4-only.

	// No match, we need to determine the default behavior. We do this by
	// fetching the default intention behavior from the resolved authorizer.
	// The default behavior if ACLs are disabled is to allow connections
	// to mimic the behavior of Consul itself: everything is allowed if
	// ACLs are disabled.
	//
	// NOTE(mitchellh): This is the same behavior as the agent authorize
	// endpoint. If this behavior is incorrect, we should also change it there
	// which is much more important.
	defaultDecision := authz.IntentionDefaultAllow(nil)

	store := s.srv.fsm.State()

	entry := structs.IntentionMatchEntry{
		Namespace: query.SourceNS,
		Partition: query.SourcePartition,
		Name:      query.SourceName,
	}
	_, intentions, err := store.IntentionMatchOne(nil, entry, structs.IntentionMatchSource, structs.IntentionTargetService)
	if err != nil {
		return fmt.Errorf("failed to query intentions for %s/%s", query.SourceNS, query.SourceName)
	}

	opts := state.IntentionDecisionOpts{
		Target:           query.DestinationName,
		Namespace:        query.DestinationNS,
		Partition:        query.DestinationPartition,
		Intentions:       intentions,
		MatchType:        structs.IntentionMatchDestination,
		DefaultDecision:  defaultDecision,
		AllowPermissions: false,
	}
	decision, err := store.IntentionDecision(opts)
	if err != nil {
		return fmt.Errorf("failed to get intention decision from (%s/%s) to (%s/%s): %v",
			query.SourceNS, query.SourceName, query.DestinationNS, query.DestinationName, err)
	}
	reply.Allowed = decision.Allowed

	return nil
}

func (s *Intention) validateEnterpriseIntention(ixn *structs.Intention) error {
	if err := s.srv.validateEnterpriseIntentionPartition(ixn.SourcePartition); err != nil {
		return fmt.Errorf("Invalid source partition %q: %v", ixn.SourcePartition, err)
	}
	if err := s.srv.validateEnterpriseIntentionNamespace(ixn.SourceNS, true); err != nil {
		return fmt.Errorf("Invalid source namespace %q: %v", ixn.SourceNS, err)
	}
	if err := s.srv.validateEnterpriseIntentionPartition(ixn.DestinationPartition); err != nil {
		return fmt.Errorf("Invalid destination partition %q: %v", ixn.DestinationPartition, err)
	}
	if err := s.srv.validateEnterpriseIntentionNamespace(ixn.DestinationNS, true); err != nil {
		return fmt.Errorf("Invalid destination namespace %q: %v", ixn.DestinationNS, err)
	}
	return nil
}

func equalStringMaps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}

	for k := range a {
		v, ok := b[k]
		if !ok || a[k] != v {
			return false
		}
	}

	return true
}
