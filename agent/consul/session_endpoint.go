// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"fmt"
	"time"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-memdb"
	"github.com/hashicorp/go-uuid"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var SessionEndpointSummaries = []prometheus.SummaryDefinition{
	{
		Name: []string{"session", "apply"},
		Help: "Measures the time spent applying a session update.",
	},
	{
		Name: []string{"session", "renew"},
		Help: "Measures the time spent renewing a session.",
	},
}

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
	if done, err := s.srv.ForwardRPC("Session.Apply", args, reply); done {
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
		if err := authz.ToAllowAuthorizer().SessionWriteAllowed(existing.Node, &authzContext); err != nil {
			return err
		}

	case structs.SessionCreate:
		if err := authz.ToAllowAuthorizer().SessionWriteAllowed(args.Session.Node, &authzContext); err != nil {
			return err
		}

	default:
		return fmt.Errorf("Invalid session operation %q", args.Op)
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
		return fmt.Errorf("apply failed: %w", err)
	}

	if args.Op == structs.SessionCreate && args.Session.TTL != "" {
		// If we created a session with a TTL, reset the expiration timer
		s.srv.resetSessionTimer(&args.Session)
	} else if args.Op == structs.SessionDestroy {
		// If we destroyed a session, it might potentially have a TTL,
		// and we need to clear the timer
		s.srv.clearSessionTimer(args.Session.ID)
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
	if done, err := s.srv.ForwardRPC("Session.Get", args, reply); done {
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
				return errNotFound
			}
			s.srv.filterACLWithAuthorizer(authz, reply)
			return nil
		})
}

// List is used to list all the active sessions
func (s *Session) List(args *structs.SessionSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.ForwardRPC("Session.List", args, reply); done {
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
			s.srv.filterACLWithAuthorizer(authz, reply)
			return nil
		})
}

// NodeSessions is used to get all the sessions for a particular node
func (s *Session) NodeSessions(args *structs.NodeSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.ForwardRPC("Session.NodeSessions", args, reply); done {
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
			s.srv.filterACLWithAuthorizer(authz, reply)
			return nil
		})
}

// Renew is used to renew the TTL on a single session
func (s *Session) Renew(args *structs.SessionSpecificRequest,
	reply *structs.IndexedSessions) error {
	if done, err := s.srv.ForwardRPC("Session.Renew", args, reply); done {
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

	if err := authz.ToAllowAuthorizer().SessionWriteAllowed(session.Node, &authzContext); err != nil {
		return err
	}

	// Reset the session TTL timer.
	reply.Sessions = structs.Sessions{session}
	if err := s.srv.resetSessionTimer(session); err != nil {
		s.logger.Error("Session renew failed", "error", err)
		return err
	}

	return nil
}
