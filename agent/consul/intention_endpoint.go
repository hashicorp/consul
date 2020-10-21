package consul

import (
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-bexpr"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
)

var (
	// ErrIntentionNotFound is returned if the intention lookup failed.
	ErrIntentionNotFound = errors.New("Intention not found")
)

// NewIntentionEndpoint returns a new Intention endpoint.
func NewIntentionEndpoint(srv *Server, logger hclog.Logger) *Intention {
	return &Intention{
		srv:                 srv,
		logger:              logger,
		configEntryEndpoint: &ConfigEntry{srv},
	}
}

// Intention manages the Connect intentions.
type Intention struct {
	// srv is a pointer back to the server.
	srv    *Server
	logger hclog.Logger

	configEntryEndpoint *ConfigEntry
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
func (s *Intention) Apply(
	args *structs.IntentionRequest,
	reply *string) error {

	// Ensure that all service-intentions config entry writes go to the primary
	// datacenter. These will then be replicated to all the other datacenters.
	args.Datacenter = s.srv.config.PrimaryDatacenter

	if done, err := s.srv.ForwardRPC("Intention.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "intention", "apply"}, time.Now())
	defer metrics.MeasureSince([]string{"intention", "apply"}, time.Now())

	if err := s.legacyUpgradeCheck(); err != nil {
		return err
	}

	// Always set a non-nil intention to avoid nil-access below
	if args.Intention == nil {
		args.Intention = &structs.Intention{}
	}

	// Get the ACL token for the request for the checks below.
	var entMeta structs.EnterpriseMeta
	ident, authz, err := s.srv.ResolveTokenIdentityAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	var accessorID string
	if ident != nil {
		accessorID = ident.ID()
	}

	var (
		configOp    structs.ConfigEntryOp
		configEntry *structs.ServiceIntentionsConfigEntry
		legacyWrite bool
	)
	switch args.Op {
	case structs.IntentionOpCreate:
		legacyWrite = true
		configOp, configEntry, err = s.computeApplyChangesLegacyCreate(accessorID, authz, &entMeta, args)
	case structs.IntentionOpUpdate:
		legacyWrite = true
		configOp, configEntry, err = s.computeApplyChangesLegacyUpdate(accessorID, authz, &entMeta, args)
	case structs.IntentionOpUpsert:
		legacyWrite = false
		configOp, configEntry, err = s.computeApplyChangesUpsert(&entMeta, args)
	case structs.IntentionOpDelete:
		if args.Intention.ID == "" {
			legacyWrite = false
			configOp, configEntry, err = s.computeApplyChangesDelete(&entMeta, args)
		} else {
			legacyWrite = true
			configOp, configEntry, err = s.computeApplyChangesLegacyDelete(accessorID, authz, &entMeta, args)
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

	if legacyWrite {
		*reply = args.Intention.ID
	} else {
		*reply = ""
	}

	if configOp == "" {
		return nil // no-op
	}

	// Commit indirectly by invoking the other RPC handler directly.

	if configOp == structs.ConfigEntryDelete {
		configReq := &structs.ConfigEntryRequest{
			Datacenter:   args.Datacenter,
			WriteRequest: args.WriteRequest,
			Op:           structs.ConfigEntryDelete,
			Entry:        configEntry,
		}

		var ignored struct{}
		return s.configEntryEndpoint.Delete(configReq, &ignored)
	}

	if configOp != structs.ConfigEntryUpsertCAS {
		return fmt.Errorf("Invalid Intention config entry operation: %v", configOp)
	}

	configReq := &structs.ConfigEntryRequest{
		Datacenter:   args.Datacenter,
		WriteRequest: args.WriteRequest,
		Op:           structs.ConfigEntryUpsertCAS,
		Entry:        configEntry,
	}

	var normalizeAndValidateFn func(raw structs.ConfigEntry) error
	if legacyWrite {
		normalizeAndValidateFn = func(raw structs.ConfigEntry) error {
			entry := raw.(*structs.ServiceIntentionsConfigEntry)
			if err := entry.LegacyNormalize(); err != nil {
				return err
			}

			return entry.LegacyValidate()
		}
	}

	var applied bool
	if err = s.configEntryEndpoint.applyInternal(configReq, &applied, normalizeAndValidateFn); err != nil {
		return err
	}

	if !applied {
		return fmt.Errorf("config entry failed to persist due to CAS failure: kind=%q, name=%q", configEntry.Kind, configEntry.Name)
	}

	return nil
}

func (s *Intention) computeApplyChangesLegacyCreate(
	accessorID string,
	authz acl.Authorizer,
	entMeta *structs.EnterpriseMeta,
	args *structs.IntentionRequest,
) (structs.ConfigEntryOp, *structs.ServiceIntentionsConfigEntry, error) {
	// This variant is just for legacy UUID-based intentions.

	args.Intention.DefaultNamespaces(entMeta)

	// Even though the eventual config entry RPC will do an authz check and
	// validation, if we do them here too we can generate error messages that
	// make more sense for legacy edits.
	if !args.Intention.CanWrite(authz) {
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Intention creation denied due to ACLs", "accessorID", accessorID)
		return "", nil, acl.ErrPermissionDenied
	}

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Intention.ID != "" {
		return "", nil, fmt.Errorf("ID must be empty when creating a new intention")
	}

	var err error
	args.Intention.ID, err = lib.GenerateUUID(s.checkIntentionID)
	if err != nil {
		return "", nil, err
	}
	// Set the created at
	args.Intention.CreatedAt = time.Now().UTC()
	args.Intention.UpdatedAt = args.Intention.CreatedAt

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	if err := s.validateEnterpriseIntention(args.Intention); err != nil {
		return "", nil, err
	}

	//nolint:staticcheck
	if err := args.Intention.Validate(); err != nil {
		return "", nil, err
	}

	_, configEntry, err := s.srv.fsm.State().ConfigEntry(nil, structs.ServiceIntentions, args.Intention.DestinationName, args.Intention.DestinationEnterpriseMeta())
	if err != nil {
		return "", nil, fmt.Errorf("service-intentions config entry lookup failed: %v", err)
	}

	if configEntry == nil {
		return structs.ConfigEntryUpsertCAS, args.Intention.ToConfigEntry(true), nil
	}
	prevEntry := configEntry.(*structs.ServiceIntentionsConfigEntry)

	if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
		return "", nil, err
	}

	upsertEntry := prevEntry.Clone()
	upsertEntry.Sources = append(upsertEntry.Sources, args.Intention.ToSourceIntention(true))

	// NOTE: if the append of this source causes a duplicate source name the
	// config entry validation will fail so we don't have to check that
	// explicitly here.

	return structs.ConfigEntryUpsertCAS, upsertEntry, nil
}

func (s *Intention) computeApplyChangesLegacyUpdate(
	accessorID string,
	authz acl.Authorizer,
	entMeta *structs.EnterpriseMeta,
	args *structs.IntentionRequest,
) (structs.ConfigEntryOp, *structs.ServiceIntentionsConfigEntry, error) {
	// This variant is just for legacy UUID-based intentions.

	_, prevEntry, ixn, err := s.srv.fsm.State().IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return "", nil, fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil || prevEntry == nil {
		return "", nil, fmt.Errorf("Cannot modify non-existent intention: '%s'", args.Intention.ID)
	}

	if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
		return "", nil, err
	}

	// Even though the eventual config entry RPC will do an authz check and
	// validation, if we do them here too we can generate error messages that
	// make more sense for legacy edits.
	if !ixn.CanWrite(authz) {
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Update operation on intention denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return "", nil, acl.ErrPermissionDenied
	}

	args.Intention.DefaultNamespaces(entMeta)

	// Prior to v1.9.0 renames of the destination side of an intention were
	// allowed, but that behavior doesn't work anymore.
	if ixn.DestinationServiceName() != args.Intention.DestinationServiceName() {
		return "", nil, fmt.Errorf("Cannot modify DestinationNS or DestinationName for an intention once it exists.")
	}

	// We always update the updatedat field.
	args.Intention.UpdatedAt = time.Now().UTC()

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	if err := s.validateEnterpriseIntention(args.Intention); err != nil {
		return "", nil, err
	}

	// Validate. We do not validate on delete since it is valid to only
	// send an ID in that case.
	//nolint:staticcheck
	if err := args.Intention.Validate(); err != nil {
		return "", nil, err
	}

	upsertEntry := prevEntry.Clone()

	foundMatch := upsertEntry.UpdateSourceByLegacyID(
		args.Intention.ID,
		args.Intention.ToSourceIntention(true),
	)
	if !foundMatch {
		return "", nil, fmt.Errorf("Cannot modify non-existent intention: '%s'", args.Intention.ID)
	}

	return structs.ConfigEntryUpsertCAS, upsertEntry, nil
}

func (s *Intention) computeApplyChangesUpsert(
	entMeta *structs.EnterpriseMeta,
	args *structs.IntentionRequest,
) (structs.ConfigEntryOp, *structs.ServiceIntentionsConfigEntry, error) {
	// This variant is just for config-entry based intentions.

	if args.Intention.ID != "" {
		// This is a new-style only endpoint
		return "", nil, fmt.Errorf("ID must not be specified")
	}

	args.Intention.DefaultNamespaces(entMeta)

	prevEntry, err := s.getServiceIntentionsConfigEntry(args.Intention.DestinationName, args.Intention.DestinationEnterpriseMeta())
	if err != nil {
		return "", nil, err
	}

	// TODO(intentions): have service-intentions validation functions
	// return structured errors so that we can rewrite the field prefix
	// here so that the validation errors are not misleading.
	if prevEntry == nil {
		// Meta is NOT permitted here, as it would need to be persisted on
		// the enclosing config entry.
		if len(args.Intention.Meta) > 0 {
			return "", nil, fmt.Errorf("Meta must not be specified")
		}

		upsertEntry := args.Intention.ToConfigEntry(false)

		return structs.ConfigEntryUpsertCAS, upsertEntry, nil
	}

	upsertEntry := prevEntry.Clone()

	if len(args.Intention.Meta) > 0 {
		// Meta is NOT permitted here, but there is one exception. If
		// you are updating a previous record, but that record lives
		// within a config entry that itself has Meta, then you may
		// incidentally ship the Meta right back to consul.
		//
		// In that case if Meta is provided, it has to be a perfect
		// match for what is already on the enclosing config entry so
		// it's safe to discard.
		if !equalStringMaps(upsertEntry.Meta, args.Intention.Meta) {
			return "", nil, fmt.Errorf("Meta must not be specified, or should be unchanged during an update.")
		}

		// Now it is safe to discard
		args.Intention.Meta = nil
	}

	sn := args.Intention.SourceServiceName()

	upsertEntry.UpsertSourceByName(sn, args.Intention.ToSourceIntention(false))

	return structs.ConfigEntryUpsertCAS, upsertEntry, nil
}

func (s *Intention) computeApplyChangesLegacyDelete(
	accessorID string,
	authz acl.Authorizer,
	entMeta *structs.EnterpriseMeta,
	args *structs.IntentionRequest,
) (structs.ConfigEntryOp, *structs.ServiceIntentionsConfigEntry, error) {
	_, prevEntry, ixn, err := s.srv.fsm.State().IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return "", nil, fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil || prevEntry == nil {
		return "", nil, fmt.Errorf("Cannot delete non-existent intention: '%s'", args.Intention.ID)
	}

	if err := checkLegacyIntentionApplyAllowed(prevEntry); err != nil {
		return "", nil, err
	}

	// Even though the eventual config entry RPC will do an authz check and
	// validation, if we do them here too we can generate error messages that
	// make more sense for legacy edits.
	if !ixn.CanWrite(authz) {
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Deletion operation on intention denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return "", nil, acl.ErrPermissionDenied
	}

	upsertEntry := prevEntry.Clone()

	deleted := upsertEntry.DeleteSourceByLegacyID(args.Intention.ID)
	if !deleted {
		return "", nil, fmt.Errorf("Cannot delete non-existent intention: '%s'", args.Intention.ID)
	}

	if upsertEntry == nil || len(upsertEntry.Sources) == 0 {
		return structs.ConfigEntryDelete, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           prevEntry.Name,
			EnterpriseMeta: prevEntry.EnterpriseMeta,
		}, nil
	}

	return structs.ConfigEntryUpsertCAS, upsertEntry, nil
}

func (s *Intention) computeApplyChangesDelete(
	entMeta *structs.EnterpriseMeta,
	args *structs.IntentionRequest,
) (structs.ConfigEntryOp, *structs.ServiceIntentionsConfigEntry, error) {
	args.Intention.DefaultNamespaces(entMeta)

	prevEntry, err := s.getServiceIntentionsConfigEntry(args.Intention.DestinationName, args.Intention.DestinationEnterpriseMeta())
	if err != nil {
		return "", nil, err
	}

	if prevEntry == nil {
		return "", nil, nil // no op means no-op
	}

	// NOTE: validation errors may be misleading!

	upsertEntry := prevEntry.Clone()

	sn := args.Intention.SourceServiceName()

	deleted := upsertEntry.DeleteSourceByName(sn)
	if !deleted {
		return "", nil, nil // no op means no-op
	}

	if upsertEntry == nil || len(upsertEntry.Sources) == 0 {
		return structs.ConfigEntryDelete, &structs.ServiceIntentionsConfigEntry{
			Kind:           structs.ServiceIntentions,
			Name:           prevEntry.Name,
			EnterpriseMeta: prevEntry.EnterpriseMeta,
		}, nil
	}

	return structs.ConfigEntryUpsertCAS, upsertEntry, nil
}

func checkLegacyIntentionApplyAllowed(prevEntry *structs.ServiceIntentionsConfigEntry) error {
	if prevEntry == nil {
		return nil
	}
	if prevEntry.LegacyIDFieldsAreAllSet() {
		return nil
	}

	sn := prevEntry.DestinationServiceName()
	return fmt.Errorf("cannot use legacy intention API to edit intentions with a destination of %q after editing them via a service-intentions config entry", sn.String())
}

// Get returns a single intention by ID.
func (s *Intention) Get(
	args *structs.IntentionQueryRequest,
	reply *structs.IndexedIntentions) error {
	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.Get", args, args, reply); done {
		return err
	}

	// Get the ACL token for the request for the checks below.
	var entMeta structs.EnterpriseMeta
	if _, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil); err != nil {
		return err
	}

	if args.Exact != nil {
		// // Finish defaulting the namespace fields.
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
			if err := s.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			// If ACLs prevented any responses, error
			if len(reply.Intentions) == 0 {
				accessorID := s.aclAccessorID(args.Token)
				// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
				s.logger.Warn("Request to get intention denied due to ACLs", "intention", args.IntentionID, "accessorID", accessorID)
				return acl.ErrPermissionDenied
			}

			return nil
		},
	)
}

// List returns all the intentions.
func (s *Intention) List(
	args *structs.IntentionListRequest,
	reply *structs.IndexedIntentions) error {
	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.List", args, args, reply); done {
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

			if err := s.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			raw, err := filter.Execute(reply.Intentions)
			if err != nil {
				return err
			}
			reply.Intentions = raw.(structs.Intentions)

			return nil
		},
	)
}

// Match returns the set of intentions that match the given source/destination.
func (s *Intention) Match(
	args *structs.IntentionQueryRequest,
	reply *structs.IndexedIntentionMatches) error {
	// Forward if necessary
	if done, err := s.srv.ForwardRPC("Intention.Match", args, args, reply); done {
		return err
	}

	// Get the ACL token for the request for the checks below.
	var entMeta structs.EnterpriseMeta
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &entMeta, nil)
	if err != nil {
		return err
	}

	// Finish defaulting the namespace fields.
	for i := range args.Match.Entries {
		if args.Match.Entries[i].Namespace == "" {
			args.Match.Entries[i].Namespace = entMeta.NamespaceOrDefault()
		}
		if err := s.srv.validateEnterpriseIntentionNamespace(args.Match.Entries[i].Namespace, true); err != nil {
			return fmt.Errorf("Invalid match entry namespace %q: %v",
				args.Match.Entries[i].Namespace, err)
		}
	}

	if authz != nil {
		var authzContext acl.AuthorizerContext
		// Go through each entry to ensure we have intention:read for the resource.

		// TODO - should we do this instead of filtering the result set? This will only allow
		// queries for which the token has intention:read permissions on the requested side
		// of the service. Should it instead return all matches that it would be able to list.
		// if so we should remove this and call filterACL instead. Based on how this is used
		// its probably fine. If you have intention read on the source just do a source type
		// matching, if you have it on the dest then perform a dest type match.
		for _, entry := range args.Match.Entries {
			entry.FillAuthzContext(&authzContext)
			if prefix := entry.Name; prefix != "" && authz.IntentionRead(prefix, &authzContext) != acl.Allow {
				accessorID := s.aclAccessorID(args.Token)
				// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
				s.logger.Warn("Operation on intention prefix denied due to ACLs", "prefix", prefix, "accessorID", accessorID)
				return acl.ErrPermissionDenied
			}
		}
	}

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
func (s *Intention) Check(
	args *structs.IntentionQueryRequest,
	reply *structs.IntentionQueryCheckResponse) error {
	// Forward maybe
	if done, err := s.srv.ForwardRPC("Intention.Check", args, args, reply); done {
		return err
	}

	// Get the test args, and defensively guard against nil
	query := args.Check
	if query == nil {
		return errors.New("Check must be specified on args")
	}

	// Get the ACL token for the request for the checks below.
	var entMeta structs.EnterpriseMeta
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

	if err := s.srv.validateEnterpriseIntentionNamespace(query.SourceNS, false); err != nil {
		return fmt.Errorf("Invalid source namespace %q: %v", query.SourceNS, err)
	}
	if err := s.srv.validateEnterpriseIntentionNamespace(query.DestinationNS, false); err != nil {
		return fmt.Errorf("Invalid destination namespace %q: %v", query.DestinationNS, err)
	}

	// Build the URI
	var uri connect.CertURI
	switch query.SourceType {
	case structs.IntentionSourceConsul:
		uri = &connect.SpiffeIDService{
			Namespace: query.SourceNS,
			Service:   query.SourceName,
		}

	default:
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
		if authz != nil && authz.ServiceRead(prefix, &authzContext) != acl.Allow {
			accessorID := s.aclAccessorID(args.Token)
			// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
			s.logger.Warn("test on intention denied due to ACLs", "prefix", prefix, "accessorID", accessorID)
			return acl.ErrPermissionDenied
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
	defaultDecision := acl.Allow
	if authz != nil {
		defaultDecision = authz.IntentionDefaultAllow(nil)
	}

	state := s.srv.fsm.State()
	decision, err := state.IntentionDecision(uri, query.DestinationName, query.DestinationNS, defaultDecision)
	if err != nil {
		return fmt.Errorf("failed to get intention decision from (%s/%s) to (%s/%s): %v",
			query.SourceNS, query.SourceName, query.DestinationNS, query.DestinationName, err)
	}
	reply.Allowed = decision.Allowed

	return nil
}

// aclAccessorID is used to convert an ACLToken's secretID to its accessorID for non-
// critical purposes, such as logging. Therefore we interpret all errors as empty-string
// so we can safely log it without handling non-critical errors at the usage site.
func (s *Intention) aclAccessorID(secretID string) string {
	_, ident, err := s.srv.ResolveIdentityFromToken(secretID)
	if acl.IsErrNotFound(err) {
		return ""
	}
	if err != nil {
		s.logger.Debug("non-critical error resolving acl token accessor for logging", "error", err)
		return ""
	}
	if ident == nil {
		return ""
	}
	return ident.ID()
}

func (s *Intention) validateEnterpriseIntention(ixn *structs.Intention) error {
	if err := s.srv.validateEnterpriseIntentionNamespace(ixn.SourceNS, true); err != nil {
		return fmt.Errorf("Invalid source namespace %q: %v", ixn.SourceNS, err)
	}
	if err := s.srv.validateEnterpriseIntentionNamespace(ixn.DestinationNS, true); err != nil {
		return fmt.Errorf("Invalid destination namespace %q: %v", ixn.DestinationNS, err)
	}
	return nil
}

func (s *Intention) getServiceIntentionsConfigEntry(name string, entMeta *structs.EnterpriseMeta) (*structs.ServiceIntentionsConfigEntry, error) {
	_, raw, err := s.srv.fsm.State().ConfigEntry(nil, structs.ServiceIntentions, name, entMeta)
	if err != nil {
		return nil, fmt.Errorf("Intention lookup failed: %v", err)
	}

	if raw == nil {
		return nil, nil
	}

	configEntry, ok := raw.(*structs.ServiceIntentionsConfigEntry)
	if !ok {
		return nil, fmt.Errorf("invalid service config type %T", raw)
	}
	return configEntry, nil
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
