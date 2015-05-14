package consul

import (
	"sync"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/coordinate"
)

type Coordinate struct {
	srv              *Server
	updateLastSent   time.Time
	updateBuffer     []*structs.CoordinateUpdateRequest
	updateBufferLock sync.Mutex
}

// GetLAN returns the the LAN coordinate of a node.
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
			if coord == nil {
				reply.Coord = nil
			} else {
				reply.Coord = coord.Coord
			}
			return err
		})
}

// GetWAN returns the WAN coordinates of the servers in a given datacenter.
//
// Note that the server does not necessarily know about *all* servers in the given datacenter.
// It just returns the coordinates of those that it knows.
func (c *Coordinate) GetWAN(args *structs.DCSpecificRequest, reply *[]*coordinate.Coordinate) error {
	if args.Datacenter == c.srv.config.Datacenter {
		*reply = make([]*coordinate.Coordinate, 1)
		(*reply)[0] = c.srv.GetWANCoordinate()
	} else {
		servers := c.srv.remoteConsuls[args.Datacenter] // servers in the specified DC
		*reply = make([]*coordinate.Coordinate, 0)
		for i := 0; i < len(servers); i++ {
			if coord := c.srv.serfWAN.GetCachedCoordinate(servers[i].Name); coord != nil {
				*reply = append(*reply, coord)
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

	c.updateBufferLock.Lock()
	c.updateBuffer = append(c.updateBuffer, args)
	if time.Since(c.updateLastSent) > c.srv.config.CoordinateUpdatePeriod || len(c.updateBuffer) > c.srv.config.CoordinateUpdateMaxBatchSize {
		c.srv.logger.Printf("sending update for %v", args.Node)
		// Apply the potentially time-consuming transaction out of band
		go func() {
			defer c.updateBufferLock.Unlock()
			_, err := c.srv.raftApply(structs.CoordinateRequestType, c.updateBuffer)
			// We clear the buffer regardless of whether the raft transaction succeeded, just so the
			// buffer doesn't keep growing without bound.
			c.updateBuffer = nil
			c.updateLastSent = time.Now()

			if err != nil {
				c.srv.logger.Printf("[ERR] consul.coordinate: Update failed: %v", err)
			}
		}()
	} else {
		c.updateBufferLock.Unlock()
	}

	return nil
}
