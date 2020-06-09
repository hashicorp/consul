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

// Intention manages the Connect intentions.
type Intention struct {
	// srv is a pointer back to the server.
	srv    *Server
	logger hclog.Logger
}

func (s *Intention) checkIntentionID(id string) (bool, error) {
	state := s.srv.fsm.State()
	if _, ixn, err := state.IntentionGet(nil, id); err != nil {
		return false, err
	} else if ixn != nil {
		return false, nil
	}

	return true, nil
}

// prepareApplyCreate validates that the requester has permissions to create the new intention,
// generates a new uuid for the intention and generally validates that the request is well-formed
func (s *Intention) prepareApplyCreate(ident structs.ACLIdentity, authz acl.Authorizer, entMeta *structs.EnterpriseMeta, args *structs.IntentionRequest) error {
	if !args.Intention.CanWrite(authz) {
		var accessorID string
		if ident != nil {
			accessorID = ident.ID()
		}
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Intention creation denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return acl.ErrPermissionDenied
	}

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Intention.ID != "" {
		return fmt.Errorf("ID must be empty when creating a new intention")
	}

	var err error
	args.Intention.ID, err = lib.GenerateUUID(s.checkIntentionID)
	if err != nil {
		return err
	}
	// Set the created at
	args.Intention.CreatedAt = time.Now().UTC()
	args.Intention.UpdatedAt = args.Intention.CreatedAt

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	args.Intention.DefaultNamespaces(entMeta)

	// Validate. We do not validate on delete since it is valid to only
	// send an ID in that case.
	// Set the precedence
	args.Intention.UpdatePrecedence()

	if err := args.Intention.Validate(); err != nil {
		return err
	}

	// make sure we set the hash prior to raft application
	args.Intention.SetHash()

	return nil
}

// prepareApplyUpdate validates that the requester has permissions on both the updated and existing
// intention as well as generally validating that the request is well-formed
func (s *Intention) prepareApplyUpdate(ident structs.ACLIdentity, authz acl.Authorizer, entMeta *structs.EnterpriseMeta, args *structs.IntentionRequest) error {
	if !args.Intention.CanWrite(authz) {
		var accessorID string
		if ident != nil {
			accessorID = ident.ID()
		}
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Update operation on intention denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return acl.ErrPermissionDenied
	}

	_, ixn, err := s.srv.fsm.State().IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil {
		return fmt.Errorf("Cannot modify non-existent intention: '%s'", args.Intention.ID)
	}

	// Perform the ACL check that we have write to the old intention too,
	// which must be true to perform any rename. This is the only ACL enforcement
	// done for deletions and a secondary enforcement for updates.
	if !ixn.CanWrite(authz) {
		var accessorID string
		if ident != nil {
			accessorID = ident.ID()
		}
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Update operation on intention denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return acl.ErrPermissionDenied
	}

	// We always update the updatedat field.
	args.Intention.UpdatedAt = time.Now().UTC()

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	args.Intention.DefaultNamespaces(entMeta)

	// Validate. We do not validate on delete since it is valid to only
	// send an ID in that case.
	// Set the precedence
	args.Intention.UpdatePrecedence()

	if err := args.Intention.Validate(); err != nil {
		return err
	}

	// make sure we set the hash prior to raft application
	args.Intention.SetHash()

	return nil
}

// prepareApplyDelete ensures that the intention specified by the ID in the request exists
// and that the requester is authorized to delete it
func (s *Intention) prepareApplyDelete(ident structs.ACLIdentity, authz acl.Authorizer, entMeta *structs.EnterpriseMeta, args *structs.IntentionRequest) error {
	// If this is not a create, then we have to verify the ID.
	state := s.srv.fsm.State()
	_, ixn, err := state.IntentionGet(nil, args.Intention.ID)
	if err != nil {
		return fmt.Errorf("Intention lookup failed: %v", err)
	}
	if ixn == nil {
		return fmt.Errorf("Cannot delete non-existent intention: '%s'", args.Intention.ID)
	}

	// Perform the ACL check that we have write to the old intention too,
	// which must be true to perform any rename. This is the only ACL enforcement
	// done for deletions and a secondary enforcement for updates.
	if !ixn.CanWrite(authz) {
		var accessorID string
		if ident != nil {
			accessorID = ident.ID()
		}
		// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
		s.logger.Warn("Deletion operation on intention denied due to ACLs", "intention", args.Intention.ID, "accessorID", accessorID)
		return acl.ErrPermissionDenied
	}

	return nil
}

// Apply creates or updates an intention in the data store.
func (s *Intention) Apply(
	args *structs.IntentionRequest,
	reply *string) error {

	// Forward this request to the primary DC if we're a secondary that's replicating intentions.
	if s.srv.intentionReplicationEnabled() {
		args.Datacenter = s.srv.config.PrimaryDatacenter
	}

	if done, err := s.srv.forward("Intention.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "intention", "apply"}, time.Now())
	defer metrics.MeasureSince([]string{"intention", "apply"}, time.Now())

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

	switch args.Op {
	case structs.IntentionOpCreate:
		if err := s.prepareApplyCreate(ident, authz, &entMeta, args); err != nil {
			return err
		}
	case structs.IntentionOpUpdate:
		if err := s.prepareApplyUpdate(ident, authz, &entMeta, args); err != nil {
			return err
		}
	case structs.IntentionOpDelete:
		if err := s.prepareApplyDelete(ident, authz, &entMeta, args); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid Intention operation: %v", args.Op)
	}

	// setup the reply which will have been filled in by one of the 3 preparedApply* funcs
	*reply = args.Intention.ID

	// Commit
	resp, err := s.srv.raftApply(structs.IntentionRequestType, args)
	if err != nil {
		s.logger.Error("Raft apply failed", "error", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

// Get returns a single intention by ID.
func (s *Intention) Get(
	args *structs.IntentionQueryRequest,
	reply *structs.IndexedIntentions) error {
	// Forward if necessary
	if done, err := s.srv.forward("Intention.Get", args, args, reply); done {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, ixn, err := state.IntentionGet(ws, args.IntentionID)
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
	args *structs.DCSpecificRequest,
	reply *structs.IndexedIntentions) error {
	// Forward if necessary
	if done, err := s.srv.forward("Intention.List", args, args, reply); done {
		return err
	}

	filter, err := bexpr.CreateFilter(args.Filter, nil, reply.Intentions)
	if err != nil {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, ixns, err := state.Intentions(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Intentions = index, ixns
			if reply.Intentions == nil {
				reply.Intentions = make(structs.Intentions, 0)
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
	if done, err := s.srv.forward("Intention.Match", args, args, reply); done {
		return err
	}

	// Get the ACL token for the request for the checks below.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}

	if rule != nil {
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
			if prefix := entry.Name; prefix != "" && rule.IntentionRead(prefix, &authzContext) != acl.Allow {
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
// Note: Whenever the logic for this method is changed, you should take
// a look at the agent authorize endpoint (agent/agent_endpoint.go) since
// the logic there is similar.
func (s *Intention) Check(
	args *structs.IntentionQueryRequest,
	reply *structs.IntentionQueryCheckResponse) error {
	// Forward maybe
	if done, err := s.srv.forward("Intention.Check", args, args, reply); done {
		return err
	}

	// Get the test args, and defensively guard against nil
	query := args.Check
	if query == nil {
		return errors.New("Check must be specified on args")
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

	// Get the ACL token for the request for the checks below.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
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
		if rule != nil && rule.ServiceRead(prefix, &authzContext) != acl.Allow {
			accessorID := s.aclAccessorID(args.Token)
			// todo(kit) Migrate intention access denial logging over to audit logging when we implement it
			s.logger.Warn("test on intention denied due to ACLs", "prefix", prefix, "accessorID", accessorID)
			return acl.ErrPermissionDenied
		}
	}

	// Get the matches for this destination
	state := s.srv.fsm.State()
	_, matches, err := state.IntentionMatch(nil, &structs.IntentionQueryMatch{
		Type: structs.IntentionMatchDestination,
		Entries: []structs.IntentionMatchEntry{
			structs.IntentionMatchEntry{
				Namespace: query.DestinationNS,
				Name:      query.DestinationName,
			},
		},
	})
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		// This should never happen since the documented behavior of the
		// Match call is that it'll always return exactly the number of results
		// as entries passed in. But we guard against misbehavior.
		return errors.New("internal error loading matches")
	}

	// Check the authorization for each match
	for _, ixn := range matches[0] {
		if auth, ok := uri.Authorize(ixn); ok {
			reply.Allowed = auth
			return nil
		}
	}

	// No match, we need to determine the default behavior. We do this by
	// specifying the anonymous token token, which will get that behavior.
	// The default behavior if ACLs are disabled is to allow connections
	// to mimic the behavior of Consul itself: everything is allowed if
	// ACLs are disabled.
	//
	// NOTE(mitchellh): This is the same behavior as the agent authorize
	// endpoint. If this behavior is incorrect, we should also change it there
	// which is much more important.
	rule, err = s.srv.ResolveToken("")
	if err != nil {
		return err
	}

	reply.Allowed = true
	if rule != nil {
		reply.Allowed = rule.IntentionDefaultAllow(nil) == acl.Allow
	}

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
