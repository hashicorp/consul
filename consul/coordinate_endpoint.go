package consul

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/coordinate"
)

// Coordinate manages queries and updates for network coordinates.
type Coordinate struct {
	// srv is a pointer back to the server.
	srv *Server

	// updates holds pending coordinate updates for the given nodes.
	updates map[string]*coordinate.Coordinate

	// updatesLock synchronizes access to the updates map.
	updatesLock sync.Mutex
}

// NewCoordinate returns a new Coordinate endpoint.
func NewCoordinate(srv *Server) *Coordinate {
	c := &Coordinate{
		srv:      srv,
		updates: make(map[string]*coordinate.Coordinate),
	}

	go c.batchUpdate()
	return c
}

// batchUpdate is a long-running routine that flushes pending coordinates to the
// Raft log in batches.
func (c *Coordinate) batchUpdate() {
	for {
		select {
		case <-time.After(c.srv.config.CoordinateUpdatePeriod):
			if err := c.batchApplyUpdates(); err != nil {
				c.srv.logger.Printf("[ERR] consul.coordinate: Batch update failed: %v", err)
			}
		case <-c.srv.shutdownCh:
			return
		}
	}
}

// batchApplyUpdates applies all pending updates to the Raft log in a series of batches.
func (c *Coordinate) batchApplyUpdates() error {
	c.updatesLock.Lock()
	defer c.updatesLock.Unlock()

	// No matter what happens in here we should clear out any unprocessed
	// updates
	defer func() {
		if len(c.updates) > 0 {
			c.srv.logger.Printf("[ERR] Discarded %d coordinate updates; increase SyncCoordinateInterval", len(c.updates))
			c.updates = make(map[string]*coordinate.Coordinate)
		}

	}()

	batch := make([]*structs.Coordinate, 0, c.srv.config.CoordinateUpdateBatchSize)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		if _, err := c.srv.raftApply(structs.CoordinateBatchUpdateType, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	// Process up to the max configured number of updates.
	remaining := c.srv.config.CoordinateUpdateBatchSize * c.srv.config.CoordinateUpdateMaxBatches
	for node, coord := range(c.updates) {
		if remaining <= 0 {
			break
		}

		batch = append(batch, &structs.Coordinate{node, coord})
		delete(c.updates, node)
		remaining--

		if len(batch) == c.srv.config.CoordinateUpdateBatchSize {
			if err := flushBatch(); err != nil {
				return err
			}
		}
	}

	// Flush any leftovers from a partial batch.
	if err := flushBatch(); err != nil {
		return err
	}

	return nil
}

// Update inserts or updates the LAN coordinate of a node.
func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) (err error) {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}

	// Since this is a coordinate coming from some place else we harden this
	// and look for dimensionality problems proactively.
	coord, err := c.srv.serfLAN.GetCoordinate()
	if err != nil {
		return err
	}
	if !coord.IsCompatibleWith(args.Coord) {
		return fmt.Errorf("rejected bad coordinate: %v", args.Coord)
	}

	// Add the coordinate to the map of pending updates.
	c.updatesLock.Lock()
	c.updates[args.Node] = args.Coord
	c.updatesLock.Unlock()
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
