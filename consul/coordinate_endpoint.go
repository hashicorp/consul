package consul

import (
	"github.com/hashicorp/consul/consul/structs"
)

type Coordinate struct {
	srv *Server
}

// Get returns the the coordinate or a node.
//
// If the node is in the same datacenter, then the LAN coordinate of the node is
// returned.  If the node is in a remote DC, then the WAN coordinate of the node
// is returned.
func (c *Coordinate) Get(args *structs.NodeSpecificRequest, reply *structs.Coordinate) error {
	if done, err := c.srv.forward("Coordinate.Get", args, args, reply); done {
		return err
	}

	state := c.srv.fsm.State()
	_, coord, err := state.CoordinateGet(args.Node)
	if err != nil {
		return err
	}
	*reply = *coord

	return nil
}

func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}
	_, err := c.srv.raftApply(structs.CoordinateRequestType, args)
	if err != nil {
		c.srv.logger.Printf("[ERR] consul.coordinate: Update failed: %v", err)
		return err
	}
	return nil
}
