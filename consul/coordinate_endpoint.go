package consul

import (
	"sync"
	"time"

	"github.com/hashicorp/consul/consul/structs"
)

// Coordinate manages queries and updates for network coordinates.
type Coordinate struct {
	// srv is a pointer back to the server.
	srv *Server

	// updateLastSent is the last time we flushed pending coordinate updates
	// to the Raft log. CoordinateUpdatePeriod is used to control how long we
	// wait before doing an update (that time, or hitting more than the
	// configured CoordinateUpdateMaxBatchSize, whichever comes first).
	updateLastSent time.Time

	// updateBuffer holds the pending coordinate updates, waiting to be
	// flushed to the Raft log.
	updateBuffer []*structs.CoordinateUpdateRequest

	// updateBufferLock manages concurrent access to updateBuffer.
	updateBufferLock sync.Mutex
}

// NewCoordinate returns a new Coordinate endpoint.
func NewCoordinate(srv *Server) *Coordinate {
	return &Coordinate{
		srv:            srv,
		updateLastSent: time.Now(),
	}
}

// Update handles requests to update the LAN coordinate of a node.
func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}

	c.updateBufferLock.Lock()
	defer c.updateBufferLock.Unlock()
	c.updateBuffer = append(c.updateBuffer, args)

	// Process updates in batches to avoid tons of small transactions against
	// the Raft log.
	shouldFlush := time.Since(c.updateLastSent) > c.srv.config.CoordinateUpdatePeriod ||
		len(c.updateBuffer) > c.srv.config.CoordinateUpdateMaxBatchSize
	if shouldFlush {
		// This transaction could take a while so we don't block here.
		buf := c.updateBuffer
		go func() {
			_, err := c.srv.raftApply(structs.CoordinateRequestType, buf)
			if err != nil {
				c.srv.logger.Printf("[ERR] consul.coordinate: Update failed: %v", err)
			}
		}()

		// We clear the buffer regardless of whether the raft transaction
		// succeeded, just so the buffer doesn't keep growing without bound.
		c.updateLastSent = time.Now()
		c.updateBuffer = nil
	}

	return nil
}

// Get returns the coordinate of the given node in the LAN.
func (c *Coordinate) Get(args *structs.NodeSpecificRequest, reply *structs.IndexedCoordinate) error {
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
