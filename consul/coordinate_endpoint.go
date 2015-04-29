package consul

import (
	"github.com/hashicorp/consul/consul/structs"
)

type Coordinate struct {
	srv *Server
}

// Get returns the the LAN coordinate of a node.
func (c *Coordinate) Get(args *structs.CoordinateGetRequest, reply *structs.IndexedCoordinate) error {
	if done, err := c.srv.forward("Coordinate.Get", args, args, reply); done {
		return err
	}

	state := c.srv.fsm.State()
	return c.srv.blockingRPC(&args.QueryOptions,
		&reply.QueryMeta,
		state.QueryTables("Coordinates"),
		func() error {
			idx, coord, err := state.CoordinateGet(args.Node)
			reply.Index = idx
			reply.Coord = coord.Coord
			return err
		})
}

// Update updates the the LAN coordinate of a node.
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
