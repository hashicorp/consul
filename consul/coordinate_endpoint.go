package consul

import (
	"fmt"
	"time"

	"github.com/hashicorp/consul/consul/structs"
)

// Coordinate manages queries and updates for network coordinates.
type Coordinate struct {
	// srv is a pointer back to the server.
	srv *Server

	// updateCh receives coordinate updates and applies them to the raft log
	// in batches so that we don't create tons of tiny transactions.
	updateCh chan *structs.Coordinate
}

// NewCoordinate returns a new Coordinate endpoint.
func NewCoordinate(srv *Server) *Coordinate {
	len := srv.config.CoordinateUpdateMaxBatchSize
	c := &Coordinate{
		srv:      srv,
		updateCh: make(chan *structs.Coordinate, len),
	}

	// This will flush all pending updates at a fixed period.
	go func() {
		for {
			select {
			case <-time.After(srv.config.CoordinateUpdatePeriod):
				c.batchApplyUpdates()
			case <-srv.shutdownCh:
				return
			}
		}
	}()

	return c
}

// batchApplyUpdates is a non-blocking routine that applies all pending updates
// to the Raft log.
func (c *Coordinate) batchApplyUpdates() {
	var updates []*structs.Coordinate
	for done := false; !done; {
		select {
		case update := <-c.updateCh:
			updates = append(updates, update)
		default:
			done = true
		}
	}

	if len(updates) > 0 {
		_, err := c.srv.raftApply(structs.CoordinateBatchUpdateType, updates)
		if err != nil {
			c.srv.logger.Printf("[ERR] consul.coordinate: Batch update failed: %v", err)
		}
	}
}

// Update inserts or updates the LAN coordinate of a node.
func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) error {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}

	// Perform a non-blocking write to the channel. We'd rather spill updates
	// than gum things up blocking here.
	update := &structs.Coordinate{Node: args.Node, Coord: args.Coord}
	select {
	case c.updateCh <- update:
		// This is a noop - we are done if the write went through.
	default:
		return fmt.Errorf("Coordinate update rate limit exceeded, increase SyncCoordinateInterval")
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
			var err error
			reply.Index, reply.Coord, err = state.CoordinateGet(args.Node)
			return err
		})
}
