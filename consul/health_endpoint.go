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
	checks := state.ChecksInState(args.State)
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
	checks := state.NodeChecks(args.Node)
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
	checks := state.ServiceChecks(args.ServiceName)
	*reply = checks
	return nil
}
