package consul

import (
	"errors"
	"fmt"
	"time"

	"github.com/armon/go-metrics"
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

	// If no ID is provided, generate a new ID. This must be done prior to
	// appending to the Raft log, because the ID is not deterministic. Once
	// the entry is in the log, the state update MUST be deterministic or
	// the followers will not converge.
	if args.Op == structs.IntentionOpCreate && args.Intention.ID == "" {
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
	}
	*reply = args.Intention.ID

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

			// TODO: acl filtering

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
			// filterACL
			return nil
		},
	)
}
