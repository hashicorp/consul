package consul

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
)

// Catalog endpoint is used to manipulate the service catalog
type Catalog struct {
	srv *Server
}

// Register is used register that a node is providing a given service.
func (c *Catalog) Register(args *structs.RegisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Register", args.Datacenter, args, reply); done {
		return err
	}

	// Verify the args
	if args.Node == "" || args.Address == "" {
		return fmt.Errorf("Must provide node and address")
	}

	if args.Service != nil {
		// If no service id, but service name, use default
		if args.Service.ID == "" && args.Service.Service != "" {
			args.Service.ID = args.Service.Service
		}

		// Verify ServiceName provided if ID
		if args.Service.ID != "" && args.Service.Service == "" {
			return fmt.Errorf("Must provide service name with ID")
		}
	}

	if args.Check != nil {
		if args.Check.CheckID == "" && args.Check.Name != "" {
			args.Check.CheckID = args.Check.Name
		}
		if args.Check.Node == "" {
			args.Check.Node = args.Node
		}
	}

	_, err := c.srv.raftApply(structs.RegisterRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] Register failed: %v", err)
		return err
	}
	return nil
}

// Deregister is used to remove a service registration for a given node.
func (c *Catalog) Deregister(args *structs.DeregisterRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Catalog.Deregister", args.Datacenter, args, reply); done {
		return err
	}

	// Verify the args
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	_, err := c.srv.raftApply(structs.DeregisterRequestType, args)
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
func (c *Catalog) ListNodes(dc string, reply *structs.Nodes) error {
	if done, err := c.srv.forward("Catalog.ListNodes", dc, dc, reply); done {
		return err
	}

	// Get the current nodes
	state := c.srv.fsm.State()
	nodes := state.Nodes()

	*reply = nodes
	return nil
}

// ListServices is used to query the services in a DC
func (c *Catalog) ListServices(dc string, reply *structs.Services) error {
	if done, err := c.srv.forward("Catalog.ListServices", dc, dc, reply); done {
		return err
	}

	// Get the current nodes
	state := c.srv.fsm.State()
	services := state.Services()

	*reply = services
	return nil
}

// ServiceNodes returns all the nodes registered as part of a service
func (c *Catalog) ServiceNodes(args *structs.ServiceSpecificRequest, reply *structs.ServiceNodes) error {
	if done, err := c.srv.forward("Catalog.ServiceNodes", args.Datacenter, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.ServiceName == "" {
		return fmt.Errorf("Must provide service name")
	}

	// Get the nodes
	state := c.srv.fsm.State()
	var nodes structs.ServiceNodes
	if args.TagFilter {
		nodes = state.ServiceTagNodes(args.ServiceName, args.ServiceTag)
	} else {
		nodes = state.ServiceNodes(args.ServiceName)
	}

	*reply = nodes
	return nil
}

// NodeServices returns all the services registered as part of a node
func (c *Catalog) NodeServices(args *structs.NodeSpecificRequest, reply *structs.NodeServices) error {
	if done, err := c.srv.forward("Catalog.NodeServices", args.Datacenter, args, reply); done {
		return err
	}

	// Verify the arguments
	if args.Node == "" {
		return fmt.Errorf("Must provide node")
	}

	// Get the node services
	state := c.srv.fsm.State()
	services := state.NodeServices(args.Node)

	*reply = *services
	return nil
}
