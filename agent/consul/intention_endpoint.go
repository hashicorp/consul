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
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
)

var (
	// ErrIntentionNotFound is returned if the intention lookup failed.
	ErrIntentionNotFound = errors.New("Intention not found")
)

// Intention manages the Connect intentions.
type Intention struct {
	// srv is a pointer back to the server.
	srv *Server
}

// Apply creates or updates an intention in the data store.
func (s *Intention) Apply(
	args *structs.IntentionRequest,
	reply *string) error {
	if done, err := s.srv.forward("Intention.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"consul", "intention", "apply"}, time.Now())
	defer metrics.MeasureSince([]string{"intention", "apply"}, time.Now())

	// Always set a non-nil intention to avoid nil-access below
	if args.Intention == nil {
		args.Intention = &structs.Intention{}
	}

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Op == structs.IntentionOpCreate {
		if args.Intention.ID != "" {
			return fmt.Errorf("ID must be empty when creating a new intention")
		}

		state := s.srv.fsm.State()
		for {
			var err error
			args.Intention.ID, err = uuid.GenerateUUID()
			if err != nil {
				s.srv.logger.Printf("[ERR] consul.intention: UUID generation failed: %v", err)
				return err
			}

			_, ixn, err := state.IntentionGet(nil, args.Intention.ID)
			if err != nil {
				s.srv.logger.Printf("[ERR] consul.intention: intention lookup failed: %v", err)
				return err
			}
			if ixn == nil {
				break
			}
		}

		// Set the created at
		args.Intention.CreatedAt = time.Now().UTC()
	}
	*reply = args.Intention.ID

	// Get the ACL token for the request for the checks below.
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Perform the ACL check
	if prefix, ok := args.Intention.GetACLPrefix(); ok {
		if rule != nil && !rule.IntentionWrite(prefix) {
			s.srv.logger.Printf("[WARN] consul.intention: Operation on intention '%s' denied due to ACLs", args.Intention.ID)
			return acl.ErrPermissionDenied
		}
	}

	// If this is not a create, then we have to verify the ID.
	if args.Op != structs.IntentionOpCreate {
		state := s.srv.fsm.State()
		_, ixn, err := state.IntentionGet(nil, args.Intention.ID)
		if err != nil {
			return fmt.Errorf("Intention lookup failed: %v", err)
		}
		if ixn == nil {
			return fmt.Errorf("Cannot modify non-existent intention: '%s'", args.Intention.ID)
		}

		// Perform the ACL check that we have write to the old prefix too,
		// which must be true to perform any rename.
		if prefix, ok := ixn.GetACLPrefix(); ok {
			if rule != nil && !rule.IntentionWrite(prefix) {
				s.srv.logger.Printf("[WARN] consul.intention: Operation on intention '%s' denied due to ACLs", args.Intention.ID)
				return acl.ErrPermissionDenied
			}
		}
	}

	// We always update the updatedat field. This has no effect for deletion.
	args.Intention.UpdatedAt = time.Now().UTC()

	// Default source type
	if args.Intention.SourceType == "" {
		args.Intention.SourceType = structs.IntentionSourceConsul
	}

	// Until we support namespaces, we force all namespaces to be default
	if args.Intention.SourceNS == "" {
		args.Intention.SourceNS = structs.IntentionDefaultNamespace
	}
	if args.Intention.DestinationNS == "" {
		args.Intention.DestinationNS = structs.IntentionDefaultNamespace
	}

	// Validate. We do not validate on delete since it is valid to only
	// send an ID in that case.
	if args.Op != structs.IntentionOpDelete {
		// Set the precedence
		args.Intention.UpdatePrecedence()

		if err := args.Intention.Validate(); err != nil {
			return err
		}
	}

	// Commit
	resp, err := s.srv.raftApply(structs.IntentionRequestType, args)
	if err != nil {
		s.srv.logger.Printf("[ERR] consul.intention: Apply failed %v", err)
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
				s.srv.logger.Printf("[WARN] consul.intention: Request to get intention '%s' denied due to ACLs", args.IntentionID)
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

			return s.srv.filterACL(args.Token, reply)
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
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	if rule != nil {
		// We go through each entry and test the destination to check if it
		// matches.
		for _, entry := range args.Match.Entries {
			if prefix := entry.Name; prefix != "" && !rule.IntentionRead(prefix) {
				s.srv.logger.Printf("[WARN] consul.intention: Operation on intention prefix '%s' denied due to ACLs", prefix)
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
	rule, err := s.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}

	// Perform the ACL check. For Check we only require ServiceRead and
	// NOT IntentionRead because the Check API only returns pass/fail and
	// returns no other information about the intentions used.
	if prefix, ok := query.GetACLPrefix(); ok {
		if rule != nil && !rule.ServiceRead(prefix) {
			s.srv.logger.Printf("[WARN] consul.intention: test on intention '%s' denied due to ACLs", prefix)
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
	rule, err = s.srv.resolveToken("")
	if err != nil {
		return err
	}

	reply.Allowed = true
	if rule != nil {
		reply.Allowed = rule.IntentionDefaultAllow()
	}

	return nil
}
