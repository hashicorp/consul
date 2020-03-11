package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"
)

// Session endpoint is used to manipulate sessions for KV
type Session struct {
	srv    *Server
	logger hclog.Logger
}

// in v1.7.0 we renamed Session -> SessionID. While its more descriptive of what
// we actually expect, it did break the RPC API for the SessionSpecificRequest. Now
// we have to put back the original name and support both with the new name being
// the canonical name and the other being considered only when the main one is empty.
func fixupSessionSpecificRequest(args *structs.SessionSpecificRequest) {
	if args.SessionID == "" {
		args.SessionID = args.Session
	}
}

// Apply is used to apply a modifying request to the data store. This should
// only be used for operations that modify the data
func (s *Session) Apply(args *structs.SessionRequest, reply *string) error {
	if done, err := s.srv.forward("Session.Apply", args, args, reply); done {
		return err
	}
	defer metrics.MeasureSince([]string{"session", "apply"}, time.Now())

	// Verify the args
	if args.Session.ID == "" && args.Op == structs.SessionDestroy {
		return fmt.Errorf("Must provide ID")
	}
	if args.Session.Node == "" && args.Op == structs.SessionCreate {
		return fmt.Errorf("Must provide Node")
	}

	//  The entMeta to populate is the one in the Session struct, not SessionRequest
	//  This is because the Session is what is passed to downstream functions like raftApply
	var authzContext acl.AuthorizerContext

	// Fetch the ACL token, if any, and apply the policy.
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.Session.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.Session.EnterpriseMeta, true); err != nil {
		return err
	}

	if authz != nil && s.srv.config.ACLEnforceVersion8 {
		switch args.Op {
		case structs.SessionDestroy:
			state := s.srv.fsm.State()
			_, existing, err := state.SessionGet(nil, args.Session.ID, &args.Session.EnterpriseMeta)
			if err != nil {
				return fmt.Errorf("Session lookup failed: %v", err)
			}
			if existing == nil {
				return nil
			}
			if authz.SessionWrite(existing.Node, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}

		case structs.SessionCreate:
			if authz.SessionWrite(args.Session.Node, &authzContext) != acl.Allow {
				return acl.ErrPermissionDenied
			}

		default:
			return fmt.Errorf("Invalid session operation %q", args.Op)
		}
	}

	// Ensure that the specified behavior is allowed
	switch args.Session.Behavior {
	case "":
		// Default behavior to Release for backwards compatibility
		args.Session.Behavior = structs.SessionKeysRelease
	case structs.SessionKeysRelease:
	case structs.SessionKeysDelete:
	default:
		return fmt.Errorf("Invalid Behavior setting '%s'", args.Session.Behavior)
	}

	// Ensure the Session TTL is valid if provided
	if args.Session.TTL != "" {
		ttl, err := time.ParseDuration(args.Session.TTL)
		if err != nil {
			return fmt.Errorf("Session TTL '%s' invalid: %v", args.Session.TTL, err)
		}

		if ttl != 0 && (ttl < s.srv.config.SessionTTLMin || ttl > structs.SessionTTLMax) {
			return fmt.Errorf("Invalid Session TTL '%d', must be between [%v=%v]",
				ttl, s.srv.config.SessionTTLMin, structs.SessionTTLMax)
		}
	}

	// If this is a create, we must generate the Session ID. This must
	// be done prior to appending to the raft log, because the ID is not
	// deterministic. Once the entry is in the log, the state update MUST
	// be deterministic or the followers will not converge.
	if args.Op == structs.SessionCreate {
		// Generate a new session ID, verify uniqueness
		state := s.srv.fsm.State()
		for {
			var err error
			if args.Session.ID, err = uuid.GenerateUUID(); err != nil {
				s.logger.Error("UUID generation failed", "error", err)
				return err
			}
			_, sess, err := state.SessionGet(nil, args.Session.ID, &args.Session.EnterpriseMeta)
			if err != nil {
				s.logger.Error("Session lookup failed", "error", err)
				return err
			}
			if sess == nil {
				break
			}
		}
	}

	// Apply the update
	resp, err := s.srv.raftApply(structs.SessionRequestType, args)
	if err != nil {
		s.logger.Error("Apply failed", "error", err)
		return err
	}

	if args.Op == structs.SessionCreate && args.Session.TTL != "" {
		// If we created a session with a TTL, reset the expiration timer
		s.srv.resetSessionTimer(args.Session.ID, &args.Session)
	} else if args.Op == structs.SessionDestroy {
		// If we destroyed a session, it might potentially have a TTL,
		// and we need to clear the timer
		s.srv.clearSessionTimer(args.Session.ID)
	}

	if respErr, ok := resp.(error); ok {
		return respErr
	}

	// Check if the return type is a string
	if respString, ok := resp.(string); ok {
		*reply = respString
	}
	return nil
}

// Get is used to retrieve a single session
func (s *Session) Get(args *structs.SessionSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.forward("Session.Get", args, args, reply); done {
		return err
	}

	fixupSessionSpecificRequest(args)

	var authzContext acl.AuthorizerContext
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, session, err := state.SessionGet(ws, args.SessionID, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index = index
			if session != nil {
				reply.Sessions = structs.Sessions{session}
			} else {
				reply.Sessions = nil
			}
			if err := s.srv.filterACLWithAuthorizer(authz, reply); err != nil {
				return err
			}
			return nil
		})
}

// List is used to list all the active sessions
func (s *Session) List(args *structs.SessionSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.forward("Session.List", args, args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, sessions, err := state.SessionList(ws, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Sessions = index, sessions
			if err := s.srv.filterACLWithAuthorizer(authz, reply); err != nil {
				return err
			}
			return nil
		})
}

// NodeSessions is used to get all the sessions for a particular node
func (s *Session) NodeSessions(args *structs.NodeSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.forward("Session.NodeSessions", args, args, reply); done {
		return err
	}

	var authzContext acl.AuthorizerContext
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.EnterpriseMeta, false); err != nil {
		return err
	}

	return s.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, sessions, err := state.NodeSessions(ws, args.Node, &args.EnterpriseMeta)
			if err != nil {
				return err
			}

			reply.Index, reply.Sessions = index, sessions
			if err := s.srv.filterACLWithAuthorizer(authz, reply); err != nil {
				return err
			}
			return nil
		})
}

// Renew is used to renew the TTL on a single session
func (s *Session) Renew(args *structs.SessionSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.forward("Session.Renew", args, args, reply); done {
		return err
	}

	fixupSessionSpecificRequest(args)

	defer metrics.MeasureSince([]string{"session", "renew"}, time.Now())

	// Fetch the ACL token, if any, and apply the policy.
	var authzContext acl.AuthorizerContext
	authz, err := s.srv.ResolveTokenAndDefaultMeta(args.Token, &args.EnterpriseMeta, &authzContext)
	if err != nil {
		return err
	}

	if err := s.srv.validateEnterpriseRequest(&args.EnterpriseMeta, true); err != nil {
		return err
	}

	// Get the session, from local state.
	state := s.srv.fsm.State()
	index, session, err := state.SessionGet(nil, args.SessionID, &args.EnterpriseMeta)
	if err != nil {
		return err
	}

	reply.Index = index
	if session == nil {
		return nil
	}

	if authz != nil && s.srv.config.ACLEnforceVersion8 && authz.SessionWrite(session.Node, &authzContext) != acl.Allow {
		return acl.ErrPermissionDenied
	}

	// Reset the session TTL timer.
	reply.Sessions = structs.Sessions{session}
	if err := s.srv.resetSessionTimer(args.SessionID, session); err != nil {
		s.logger.Error("Session renew failed", "error", err)
		return err
	}

	return nil
}
