package consul

import (
	"github.com/hashicorp/consul/rpc"
)

// Catalog endpoint is used to manipulate the service catalog
type Catalog struct {
	*Server
}

/*
* Register : Registers that a node provides a given service
* Deregister : Deregisters that a node provides a given service
* RemoveNode: Used to force remove a node

* ListDatacenters: List the known datacenters
* ListServices : Lists the available services
* ListNodes : Lists the available nodes
* ServiceNodes: Returns the nodes that are part of a service
* NodeServices: Returns the services that a node is registered for
 */

// Register is used register that a node is providing a given service.
// Returns true if the entry was added, false if it already exists, or
// an error is returned.
func (c *Catalog) Register(args *rpc.RegisterRequest, reply *bool) error {
	if done, err := c.forward("Catalog.Register", args.Datacenter, args, reply); done {
		return err
	}

	// Run it through raft
	resp, err := c.raftApply(rpc.RegisterRequestType, args)
	if err != nil {
		c.logger.Printf("[ERR] Register failed: %v", err)
		return err
	}

	// Set the response
	*reply = resp.(bool)
	return nil
}

// Deregister is used to remove a service registration for a given node.
// Returns true if the entry was removed, false if it doesn't exist or
// an error is returned.
func (c *Catalog) Deregister(args *rpc.DeregisterRequest, reply *bool) error {
	return nil
}
