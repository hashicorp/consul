package consul

import (
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/coordinate"
)

type Coordinate struct {
	srv *Server
}

var (
	// We batch updates and send them together every 30 seconds, or every 1000 updates,
	// whichever comes sooner
	updatePeriod       = time.Duration(30) * time.Second
	updateBatchMaxSize = 1000

	updateBuffer   []*structs.CoordinateUpdateRequest
	updateLastSent time.Time
)

func init() {
	updateBuffer = nil
	updateLastSent = time.Now()
}

// Get returns the the LAN coordinate of a node.
func (c *Coordinate) GetLAN(args *structs.NodeSpecificRequest, reply *structs.IndexedCoordinate) error {
	if done, err := c.srv.forward("Coordinate.GetLAN", args, args, reply); done {
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

// Get returns the the WAN coordinate of a datacenter.
func (c *Coordinate) GetWAN(args *structs.DCSpecificRequest, reply *coordinate.Coordinate) error {
	if args.Datacenter == c.srv.config.Datacenter {
		*reply = *c.srv.GetWANCoordinate()
	} else {
		servers := c.srv.remoteConsuls[args.Datacenter] // servers in the specified DC
		for i := 0; i < len(servers); i++ {
			if coord := c.srv.serfWAN.GetCachedCoordinate(servers[i].Name); coord != nil {
				*reply = *coord
			}
		}
	}

	return nil
}

// Update updates the the LAN coordinate of a node.
func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}

	updateBuffer = append(updateBuffer, args)
	if time.Since(updateLastSent) > updatePeriod || len(updateBuffer) > updateBatchMaxSize {
		_, err := c.srv.raftApply(structs.CoordinateRequestType, updateBuffer)
		// We clear the buffer regardless of whether the raft transaction succeeded, just so the
		// buffer doesn't keep growing without bound.
		updateBuffer = nil
		updateLastSent = time.Now()

		if err != nil {
			c.srv.logger.Printf("[ERR] consul.coordinate: Update failed: %v", err)
			return err
		}
	}

	return nil
}
