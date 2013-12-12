package consul

import (
	"github.com/hashicorp/consul/rpc"
)

// Catalog endpoint is used to manipulate the service catalog
type Catalog struct {
	srv *Server
}

/*
* Register : Registers that a node provides a given service
* Deregister : Deregisters that a node provides a given service

* ListDatacenters: List the known datacenters
* ListServices : Lists the available services
* ListNodes : Lists the available nodes
* ServiceNodes: Returns the nodes that are part of a service
* NodeServices: Returns the services that a node is registered for
 */

// Register is used register that a node is providing a given service.
func (c *Catalog) Register(args *rpc.RegisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Register", args.Datacenter, args, reply); done {
		return err
	}

	_, err := c.srv.raftApply(rpc.RegisterRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] Register failed: %v", err)
		return err
	}
	return nil
}

// Deregister is used to remove a service registration for a given node.
func (c *Catalog) Deregister(args *rpc.DeregisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Deregister", args.Datacenter, args, reply); done {
		return err
	}

	_, err := c.srv.raftApply(rpc.DeregisterRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] Deregister failed: %v", err)
		return err
	}
	return nil
}

// ListDatacenters is used to query for the list of known datacenters
func (c *Catalog) ListDatacenters(args *struct{}, reply *[]string) error {
	c.srv.remoteLock.RLock()
	defer c.srv.remoteLock.RUnlock()

	// Read the known DCs
	var dcs []string
	for dc := range c.srv.remoteConsuls {
		dcs = append(dcs, dc)
	}

	// Return
	*reply = dcs
	return nil
}

// ListNodes is used to query the nodes in a DC
func (c *Catalog) ListNodes(dc string, reply *rpc.Nodes) error {
	if done, err := c.srv.forward("Catalog.ListNodes", dc, dc, reply); done {
		return err
	}

	// Get the current nodes
	state := c.srv.fsm.State()
	rawNodes := state.Nodes()

	// Format the response
	nodes := rpc.Nodes(make([]rpc.Node, len(rawNodes)/2))
	for i := 0; i < len(rawNodes); i += 2 {
		nodes[i] = rpc.Node{rawNodes[i], rawNodes[i+1]}
	}

	*reply = nodes
	return nil
}

// ListServices is used to query the services in a DC
func (c *Catalog) ListServices(dc string, reply *rpc.Services) error {
	if done, err := c.srv.forward("Catalog.ListServices", dc, dc, reply); done {
		return err
	}

	// Get the current nodes
	state := c.srv.fsm.State()
	services := state.Services()

	*reply = services
	return nil
}
