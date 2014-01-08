package consul

import (
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

	// TODO
	return nil
}

// NodeChecks is used to get all the checks for a node
func (h *Health) NodeChecks(args *structs.NodeSpecificRequest,
	reply *structs.HealthChecks) error {
	if done, err := h.srv.forward("Health.NodeChecks", args.Datacenter, args, reply); done {
		return err
	}

	// TODO
	return nil
}

// ServiceChecks is used to get all the checks for a service
func (h *Health) ServiceChecks(args *structs.ServiceSpecificRequest,
	reply *structs.HealthChecks) error {
	if done, err := h.srv.forward("Health.ServiceChecks", args.Datacenter, args, reply); done {
		return err
	}

	// TODO
	return nil
}
