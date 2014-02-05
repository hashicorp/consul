package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
)

// Health endpoint is used to query the health information
type Health struct {
	srv *Server
}

// ChecksInState is used to get all the checks in a given state
func (h *Health) ChecksInState(args *structs.ChecksInStateRequest,
	reply *structs.HealthChecks) error {
	if done, err := h.srv.forward("Health.ChecksInState", args.Datacenter, args, reply); done {
		return err
	}

	// Get the state specific checks
	state := h.srv.fsm.State()
	_, checks := state.ChecksInState(args.State)
	*reply = checks
	return nil
}

// NodeChecks is used to get all the checks for a node
func (h *Health) NodeChecks(args *structs.NodeSpecificRequest,
	reply *structs.HealthChecks) error {
	if done, err := h.srv.forward("Health.NodeChecks", args.Datacenter, args, reply); done {
		return err
	}

	// Get the node checks
	state := h.srv.fsm.State()
	_, checks := state.NodeChecks(args.Node)
	*reply = checks
	return nil
}

// ServiceChecks is used to get all the checks for a service
func (h *Health) ServiceChecks(args *structs.ServiceSpecificRequest,
	reply *structs.HealthChecks) error {
	// Reject if tag filtering is on
	if args.TagFilter {
		return fmt.Errorf("Tag filtering is not supported")
	}

	// Potentially forward
	if done, err := h.srv.forward("Health.ServiceChecks", args.Datacenter, args, reply); done {
		return err
	}

	// Get the service checks
	state := h.srv.fsm.State()
	_, checks := state.ServiceChecks(args.ServiceName)
	*reply = checks
	return nil
}

// ServiceNodes returns all the nodes registered as part of a service including health info
func (h *Health) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.CheckServiceNodes) error {
	if done, err := h.srv.forward("Health.ServiceNodes", args.Datacenter, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide service name")
	}

	// Get the nodes
	state := h.srv.fsm.State()
	var nodes structs.CheckServiceNodes
	if args.TagFilter {
		_, nodes = state.CheckServiceTagNodes(args.ServiceName, args.ServiceTag)
	} else {
		_, nodes = state.CheckServiceNodes(args.ServiceName)
	}

	*reply = nodes
	return nil
}
