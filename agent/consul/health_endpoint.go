package consul

import (
	"fmt"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// Health endpoint is used to query the health information
type Health struct {
	srv *Server
}

// ChecksInState is used to get all the checks in a given state
func (h *Health) ChecksInState(args *structs.ChecksInStateRequest,
	reply *structs.IndexedHealthChecks) error {
	if done, err := h.srv.forward("Health.ChecksInState", args, args, reply); done {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var checks structs.HealthChecks
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, checks, err = state.ChecksInStateByNodeMeta(ws, args.State, args.NodeMetaFilters)
			} else {
				index, checks, err = state.ChecksInState(ws, args.State)
			}
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return h.srv.sortNodesByDistanceFrom(args.Source, reply.HealthChecks)
		})
}

// NodeChecks is used to get all the checks for a node
func (h *Health) NodeChecks(args *structs.NodeSpecificRequest,
	reply *structs.IndexedHealthChecks) error {
	if done, err := h.srv.forward("Health.NodeChecks", args, args, reply); done {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, checks, err := state.NodeChecks(ws, args.Node)
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			return h.srv.filterACL(args.Token, reply)
		})
}

// ServiceChecks is used to get all the checks for a service
func (h *Health) ServiceChecks(args *structs.ServiceSpecificRequest,
	reply *structs.IndexedHealthChecks) error {
	// Reject if tag filtering is on
	if args.TagFilter {
		return fmt.Errorf("Tag filtering is not supported")
	}

	// Potentially forward
	if done, err := h.srv.forward("Health.ServiceChecks", args, args, reply); done {
		return err
	}

	return h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var checks structs.HealthChecks
			var err error
			if len(args.NodeMetaFilters) > 0 {
				index, checks, err = state.ServiceChecksByNodeMeta(ws, args.ServiceName, args.NodeMetaFilters)
			} else {
				index, checks, err = state.ServiceChecks(ws, args.ServiceName)
			}
			if err != nil {
				return err
			}
			reply.Index, reply.HealthChecks = index, checks
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return h.srv.sortNodesByDistanceFrom(args.Source, reply.HealthChecks)
		})
}

// ServiceNodes returns all the nodes registered as part of a service including health info
func (h *Health) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.IndexedCheckServiceNodes) error {
	if done, err := h.srv.forward("Health.ServiceNodes", args, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide service name")
	}

	err := h.srv.blockingQuery(
		&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			var index uint64
			var nodes structs.CheckServiceNodes
			var err error
			if args.TagFilter {
				index, nodes, err = state.CheckServiceTagNodes(ws, args.ServiceName, args.ServiceTag)
			} else {
				index, nodes, err = state.CheckServiceNodes(ws, args.ServiceName)
			}
			if err != nil {
				return err
			}

			reply.Index, reply.Nodes = index, nodes
			if len(args.NodeMetaFilters) > 0 {
				reply.Nodes = nodeMetaFilter(args.NodeMetaFilters, reply.Nodes)
			}
			if err := h.srv.filterACL(args.Token, reply); err != nil {
				return err
			}
			return h.srv.sortNodesByDistanceFrom(args.Source, reply.Nodes)
		})

	// Provide some metrics
	if err == nil {
		metrics.IncrCounterWithLabels([]string{"consul", "health", "service", "query"}, 1,
			[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		metrics.IncrCounterWithLabels([]string{"health", "service", "query"}, 1,
			[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		if args.ServiceTag != "" {
			metrics.IncrCounterWithLabels([]string{"consul", "health", "service", "query-tag"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}, {Name: "tag", Value: args.ServiceTag}})
			metrics.IncrCounterWithLabels([]string{"health", "service", "query-tag"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}, {Name: "tag", Value: args.ServiceTag}})
		}
		if len(reply.Nodes) == 0 {
			metrics.IncrCounterWithLabels([]string{"consul", "health", "service", "not-found"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}})
			metrics.IncrCounterWithLabels([]string{"health", "service", "not-found"}, 1,
				[]metrics.Label{{Name: "service", Value: args.ServiceName}})
		}
	}
	return err
}
