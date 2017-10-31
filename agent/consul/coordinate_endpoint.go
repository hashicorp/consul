package consul

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

// Coordinate manages queries and updates for network coordinates.
type Coordinate struct {
	// srv is a pointer back to the server.
	srv *Server

	// updates holds pending coordinate updates for the given nodes. This is
	// keyed by node:segment so we can get a coordinate for each segment for
	// servers, and we only track the latest update per node:segment.
	updates map[string]*structs.CoordinateUpdateRequest

	// updatesLock synchronizes access to the updates map.
	updatesLock sync.Mutex
}

// NewCoordinate returns a new Coordinate endpoint.
func NewCoordinate(srv *Server) *Coordinate {
	c := &Coordinate{
		srv:     srv,
		updates: make(map[string]*structs.CoordinateUpdateRequest),
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
				c.srv.logger.Printf("[WARN] consul.coordinate: Batch update failed: %v", err)
			}
		case <-c.srv.shutdownCh:
			return
		}
	}
}

// batchApplyUpdates applies all pending updates to the Raft log in a series of
// batches.
func (c *Coordinate) batchApplyUpdates() error {
	// Grab the pending updates and release the lock so we can still handle
	// incoming messages.
	c.updatesLock.Lock()
	pending := c.updates
	c.updates = make(map[string]*structs.CoordinateUpdateRequest)
	c.updatesLock.Unlock()

	// Enforce the rate limit.
	limit := c.srv.config.CoordinateUpdateBatchSize * c.srv.config.CoordinateUpdateMaxBatches
	size := len(pending)
	if size > limit {
		c.srv.logger.Printf("[WARN] consul.coordinate: Discarded %d coordinate updates", size-limit)
		size = limit
	}

	// Transform the map into a slice that we can feed to the Raft log in
	// batches.
	i := 0
	updates := make(structs.Coordinates, size)
	for _, update := range pending {
		if !(i < size) {
			break
		}

		updates[i] = &structs.Coordinate{
			Node:    update.Node,
			Segment: update.Segment,
			Coord:   update.Coord,
		}
		i++
	}

	// Apply the updates to the Raft log in batches.
	for start := 0; start < size; start += c.srv.config.CoordinateUpdateBatchSize {
		end := start + c.srv.config.CoordinateUpdateBatchSize
		if end > size {
			end = size
		}

		// We set the "safe to ignore" flag on this update type so old
		// servers don't crash if they see one of these.
		t := structs.CoordinateBatchUpdateType | structs.IgnoreUnknownTypeFlag

		slice := updates[start:end]
		resp, err := c.srv.raftApply(t, slice)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}
	}
	return nil
}

// Update inserts or updates the LAN coordinate of a node.
func (c *Coordinate) Update(args *structs.CoordinateUpdateRequest, reply *struct{}) (err error) {
	if done, err := c.srv.forward("Coordinate.Update", args, args, reply); done {
		return err
	}

	// Older clients can send coordinates with invalid numeric values like
	// NaN and Inf. We guard against these coming in, though newer clients
	// should never send these.
	if !args.Coord.IsValid() {
		return fmt.Errorf("invalid coordinate")
	}

	// Since this is a coordinate coming from some place else we harden this
	// and look for dimensionality problems proactively.
	coord, err := c.srv.serfLAN.GetCoordinate()
	if err != nil {
		return err
	}
	if !coord.IsCompatibleWith(args.Coord) {
		return fmt.Errorf("incompatible coordinate")
	}

	// Fetch the ACL token, if any, and enforce the node policy if enabled.
	rule, err := c.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && c.srv.config.ACLEnforceVersion8 {
		if !rule.NodeWrite(args.Node, nil) {
			return acl.ErrPermissionDenied
		}
	}

	// Add the coordinate to the map of pending updates.
	key := fmt.Sprintf("%s:%s", args.Node, args.Segment)
	c.updatesLock.Lock()
	c.updates[key] = args
	c.updatesLock.Unlock()
	return nil
}

// ListDatacenters returns the list of datacenters and their respective nodes
// and the raw coordinates of those nodes (if no coordinates are available for
// any of the nodes, the node list may be empty).
func (c *Coordinate) ListDatacenters(args *struct{}, reply *[]structs.DatacenterMap) error {
	maps, err := c.srv.router.GetDatacenterMaps()
	if err != nil {
		return err
	}

	// Strip the datacenter suffixes from all the node names.
	for i := range maps {
		suffix := fmt.Sprintf(".%s", maps[i].Datacenter)
		for j := range maps[i].Coordinates {
			node := maps[i].Coordinates[j].Node
			maps[i].Coordinates[j].Node = strings.TrimSuffix(node, suffix)
		}
	}

	*reply = maps
	return nil
}

// ListNodes returns the list of nodes with their raw network coordinates (if no
// coordinates are available for a node it won't appear in this list).
func (c *Coordinate) ListNodes(args *structs.DCSpecificRequest, reply *structs.IndexedCoordinates) error {
	if done, err := c.srv.forward("Coordinate.ListNodes", args, args, reply); done {
		return err
	}

	return c.srv.blockingQuery(&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, coords, err := state.Coordinates(ws)
			if err != nil {
				return err
			}

			reply.Index, reply.Coordinates = index, coords
			if err := c.srv.filterACL(args.Token, reply); err != nil {
				return err
			}

			return nil
		})
}

// Node returns the raw coordinates for a single node.
func (c *Coordinate) Node(args *structs.NodeSpecificRequest, reply *structs.IndexedCoordinates) error {
	if done, err := c.srv.forward("Coordinate.Node", args, args, reply); done {
		return err
	}

	// Fetch the ACL token, if any, and enforce the node policy if enabled.
	rule, err := c.srv.resolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && c.srv.config.ACLEnforceVersion8 {
		if !rule.NodeRead(args.Node) {
			return acl.ErrPermissionDenied
		}
	}

	return c.srv.blockingQuery(&args.QueryOptions,
		&reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, nodeCoords, err := state.Coordinate(args.Node, ws)
			if err != nil {
				return err
			}

			var coords structs.Coordinates
			for segment, coord := range nodeCoords {
				coords = append(coords, &structs.Coordinate{
					Node:    args.Node,
					Segment: segment,
					Coord:   coord,
				})
			}
			reply.Index, reply.Coordinates = index, coords
			return nil
		})
}
