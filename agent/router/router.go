// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package router

import (
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/types"
)

// Router keeps track of a set of network areas and their associated Serf
// membership of Consul servers. It then indexes this by datacenter to provide
// healthy routes to servers by datacenter.
type Router struct {
	// logger is used for diagnostic output.
	logger hclog.Logger

	// localDatacenter has the name of the router's home datacenter. This is
	// used to short-circuit RTT calculations for local servers.
	localDatacenter string

	// serverName has the name of the router's server. This is used to
	// short-circuit pinging to itself.
	serverName string

	// areas maps area IDs to structures holding information about that
	// area.
	areas map[types.AreaID]*areaInfo

	// managers is an index from datacenter names to a list of server
	// managers for that datacenter. This is used to quickly lookup routes.
	managers map[string][]*Manager

	// routeFn is a hook to actually do the routing.
	routeFn func(datacenter string) (*Manager, *metadata.Server, bool)

	// grpcServerTracker is used to balance grpc connections across servers,
	// and has callbacks for adding or removing a server.
	grpcServerTracker ServerTracker

	// isShutdown prevents adding new routes to a router after it is shut
	// down.
	isShutdown bool

	// This top-level lock covers all the internal state.
	sync.RWMutex
}

// RouterSerfCluster is an interface wrapper around Serf in order to make this
// easier to unit test.
type RouterSerfCluster interface {
	NumNodes() int
	Members() []serf.Member
	GetCoordinate() (*coordinate.Coordinate, error)
	GetCachedCoordinate(name string) (*coordinate.Coordinate, bool)
}

// managerInfo holds a server manager for a datacenter along with its associated
// shutdown channel.
type managerInfo struct {
	// manager is notified about servers for this datacenter.
	manager *Manager

	// shutdownCh is only given to this manager so we can shut it down when
	// all servers for this datacenter are gone.
	shutdownCh chan struct{}
}

// areaInfo holds information about a given network area.
type areaInfo struct {
	// cluster is the Serf instance for this network area.
	cluster RouterSerfCluster

	// pinger is used to ping servers in this network area when trying to
	// find a new, healthy server to talk to.
	pinger Pinger

	// managers maps datacenter names to managers for that datacenter in
	// this area.
	managers map[string]*managerInfo

	// useTLS specifies whether to use TLS to communicate for this network area.
	useTLS bool
}

// NewRouter returns a new Router with the given configuration.
func NewRouter(logger hclog.Logger, localDatacenter, serverName string, tracker ServerTracker) *Router {
	if logger == nil {
		logger = hclog.New(&hclog.LoggerOptions{})
	}
	if tracker == nil {
		tracker = NoOpServerTracker{}
	}

	router := &Router{
		logger:            logger.Named(logging.Router),
		localDatacenter:   localDatacenter,
		serverName:        serverName,
		areas:             make(map[types.AreaID]*areaInfo),
		managers:          make(map[string][]*Manager),
		grpcServerTracker: tracker,
	}

	// Hook the direct route lookup by default.
	router.routeFn = router.findDirectRoute

	return router
}

// Shutdown removes all areas from the router, which stops all their respective
// managers. No new areas can be added after the router is shut down.
func (r *Router) Shutdown() {
	r.Lock()
	defer r.Unlock()

	for areaID, area := range r.areas {
		for datacenter, info := range area.managers {
			r.removeManagerFromIndex(datacenter, info.manager)
			close(info.shutdownCh)
		}

		delete(r.areas, areaID)
	}

	r.isShutdown = true
}

// AddArea registers a new network area with the router.
func (r *Router) AddArea(areaID types.AreaID, cluster RouterSerfCluster, pinger Pinger) error {
	r.Lock()
	defer r.Unlock()

	if r.isShutdown {
		return fmt.Errorf("cannot add area, router is shut down")
	}

	if _, ok := r.areas[areaID]; ok {
		return fmt.Errorf("area ID %q already exists", areaID)
	}

	area := &areaInfo{
		cluster:  cluster,
		pinger:   pinger,
		managers: make(map[string]*managerInfo),
	}
	r.areas[areaID] = area

	// always ensure we have a started manager for the LAN area
	if areaID == types.AreaLAN {
		r.logger.Info("Initializing LAN area manager")
		r.maybeInitializeManager(area, r.localDatacenter)
	}

	// Do an initial populate of the manager so that we don't have to wait
	// for events to fire. This lets us attempt to use all the known servers
	// initially, and then will quickly detect that they are failed if we
	// can't reach them.
	for _, m := range cluster.Members() {
		ok, parts := metadata.IsConsulServer(m)
		if !ok {
			if areaID != types.AreaLAN {
				r.logger.Warn("Non-server in server-only area",
					"non_server", m.Name,
					"area", areaID,
				)
			}
			continue
		}

		if err := r.addServer(areaID, area, parts); err != nil {
			return fmt.Errorf("failed to add server %q to area %q: %v", m.Name, areaID, err)
		}
	}

	return nil
}

// GetServerMetadataByAddr returns server metadata by dc and address. If it
// didn't find anything, nil is returned.
func (r *Router) GetServerMetadataByAddr(dc, addr string) *metadata.Server {
	r.RLock()
	defer r.RUnlock()
	if ms, ok := r.managers[dc]; ok {
		for _, m := range ms {
			for _, s := range m.getServerList().servers {
				if s.Addr.String() == addr {
					return s
				}
			}
		}
	}
	return nil
}

// removeManagerFromIndex does cleanup to take a manager out of the index of
// datacenters. This assumes the lock is already held for writing, and will
// panic if the given manager isn't found.
func (r *Router) removeManagerFromIndex(datacenter string, manager *Manager) {
	managers := r.managers[datacenter]
	for i := 0; i < len(managers); i++ {
		if managers[i] == manager {
			r.managers[datacenter] = append(managers[:i], managers[i+1:]...)
			if len(r.managers[datacenter]) == 0 {
				delete(r.managers, datacenter)
			}
			return
		}
	}
	panic("managers index out of sync")
}

// Returns whether TLS is enabled for the given area ID
func (r *Router) TLSEnabled(areaID types.AreaID) (bool, error) {
	r.RLock()
	defer r.RUnlock()

	area, ok := r.areas[areaID]
	if !ok {
		return false, fmt.Errorf("area ID %q does not exist", areaID)
	}

	return area.useTLS, nil
}

// RemoveArea removes an existing network area from the router.
func (r *Router) RemoveArea(areaID types.AreaID) error {
	r.Lock()
	defer r.Unlock()

	area, ok := r.areas[areaID]
	if !ok {
		return fmt.Errorf("area ID %q does not exist", areaID)
	}

	// Remove all of this area's managers from the index and shut them down.
	for datacenter, info := range area.managers {
		r.removeManagerFromIndex(datacenter, info.manager)
		close(info.shutdownCh)
	}

	delete(r.areas, areaID)
	return nil
}

// maybeInitializeManager will initialize a new manager for the given area/dc
// if its not already created. Calling this function should only be done if
// holding a write lock on the Router.
func (r *Router) maybeInitializeManager(area *areaInfo, dc string) *Manager {
	info, ok := area.managers[dc]
	if ok {
		return info.manager
	}

	shutdownCh := make(chan struct{})
	rb := r.grpcServerTracker.NewRebalancer(dc)
	manager := New(r.logger, shutdownCh, area.cluster, area.pinger, r.serverName, rb)
	info = &managerInfo{
		manager:    manager,
		shutdownCh: shutdownCh,
	}
	area.managers[dc] = info

	managers := r.managers[dc]
	r.managers[dc] = append(managers, manager)
	go manager.Run()

	return manager
}

// addServer does the work of AddServer once the write lock is held.
func (r *Router) addServer(areaID types.AreaID, area *areaInfo, s *metadata.Server) error {
	// Make the manager on the fly if this is the first we've seen of it,
	// and add it to the index.
	manager := r.maybeInitializeManager(area, s.Datacenter)

	// If TLS is enabled for the area, set it on the server so the manager
	// knows to use TLS when pinging it.
	if area.useTLS {
		s.UseTLS = true
	}

	manager.AddServer(s)
	r.grpcServerTracker.AddServer(areaID, s)
	return nil
}

// AddServer should be called whenever a new server joins an area. This is
// typically hooked into the Serf event handler area for this area.
func (r *Router) AddServer(areaID types.AreaID, s *metadata.Server) error {
	r.Lock()
	defer r.Unlock()

	area, ok := r.areas[areaID]
	if !ok {
		return fmt.Errorf("area ID %q does not exist", areaID)
	}
	return r.addServer(areaID, area, s)
}

// RemoveServer should be called whenever a server is removed from an area. This
// is typically hooked into the Serf event handler area for this area.
func (r *Router) RemoveServer(areaID types.AreaID, s *metadata.Server) error {
	r.Lock()
	defer r.Unlock()

	area, ok := r.areas[areaID]
	if !ok {
		return fmt.Errorf("area ID %q does not exist", areaID)
	}

	// If the manager has already been removed we just quietly exit. This
	// can get called by Serf events, so the timing isn't totally
	// deterministic.
	info, ok := area.managers[s.Datacenter]
	if !ok {
		return nil
	}
	info.manager.RemoveServer(s)
	r.grpcServerTracker.RemoveServer(areaID, s)

	// If this manager is empty then remove it so we don't accumulate cruft
	// and waste time during request routing.
	if num := info.manager.NumServers(); num == 0 {
		r.removeManagerFromIndex(s.Datacenter, info.manager)
		close(info.shutdownCh)
		delete(area.managers, s.Datacenter)
	}

	return nil
}

// FailServer should be called whenever a server is failed in an area. This
// is typically hooked into the Serf event handler area for this area. We will
// immediately shift traffic away from this server, but it will remain in the
// list of servers.
func (r *Router) FailServer(areaID types.AreaID, s *metadata.Server) error {
	r.RLock()
	defer r.RUnlock()

	area, ok := r.areas[areaID]
	if !ok {
		return fmt.Errorf("area ID %q does not exist", areaID)
	}

	// If the manager has already been removed we just quietly exit. This
	// can get called by Serf events, so the timing isn't totally
	// deterministic.
	info, ok := area.managers[s.Datacenter]
	if !ok {
		return nil
	}

	info.manager.NotifyFailedServer(s)
	return nil
}

// FindRoute returns a healthy server with a route to the given datacenter. The
// Boolean return parameter will indicate if a server was available. In some
// cases this may return a best-effort unhealthy server that can be used for a
// connection attempt. If any problem occurs with the given server, the caller
// should feed that back to the manager associated with the server, which is
// also returned, by calling NotifyFailedServer().
func (r *Router) FindRoute(datacenter string) (*Manager, *metadata.Server, bool) {
	return r.routeFn(datacenter)
}

// FindLANRoute returns a healthy server within the local datacenter. In some
// cases this may return a best-effort unhealthy server that can be used for a
// connection attempt. If any problem occurs with the given server, the caller
// should feed that back to the manager associated with the server, which is
// also returned, by calling NotifyFailedServer().
func (r *Router) FindLANRoute() (*Manager, *metadata.Server) {
	mgr := r.GetLANManager()

	if mgr == nil {
		return nil, nil
	}

	return mgr, mgr.FindServer()
}

// FindLANServer will look for a server in the local datacenter.
// This function may return a nil value if no server is available.
func (r *Router) FindLANServer() *metadata.Server {
	_, srv := r.FindLANRoute()
	return srv
}

// findDirectRoute looks for a route to the given datacenter if it's directly
// adjacent to the server.
func (r *Router) findDirectRoute(datacenter string) (*Manager, *metadata.Server, bool) {
	r.RLock()
	defer r.RUnlock()

	// Get the list of managers for this datacenter. This will usually just
	// have one entry, but it's possible to have a user-defined area + WAN.
	managers, ok := r.managers[datacenter]
	if !ok {
		return nil, nil, false
	}

	// Try each manager until we get a server.
	for _, manager := range managers {
		if manager.IsOffline() {
			continue
		}

		if s := manager.FindServer(); s != nil {
			return manager, s, true
		}
	}

	// Didn't find a route (even via an unhealthy server).
	return nil, nil, false
}

// CheckServers returns thwo things
// 1. bool to indicate whether any servers were processed
// 2. error if any propagated from the fn
//
// The fn called should return a bool indicating whether checks should continue and an error
// If an error is returned then checks will stop immediately
func (r *Router) CheckServers(dc string, fn func(srv *metadata.Server) bool) {
	r.RLock()
	defer r.RUnlock()

	managers, ok := r.managers[dc]
	if !ok {
		return
	}

	for _, m := range managers {
		if !m.checkServers(fn) {
			return
		}
	}
}

// GetDatacenters returns a list of datacenters known to the router, sorted by
// name.
func (r *Router) GetDatacenters() []string {
	r.RLock()
	defer r.RUnlock()

	dcs := make([]string, 0, len(r.managers))
	for dc := range r.managers {
		dcs = append(dcs, dc)
	}

	sort.Strings(dcs)
	return dcs
}

// GetRemoteDatacenters returns a list of remote datacenters known to the router, sorted by
// name.
func (r *Router) GetRemoteDatacenters(local string) []string {
	r.RLock()
	defer r.RUnlock()

	dcs := make([]string, 0, len(r.managers))
	for dc := range r.managers {
		if dc == local {
			continue
		}
		dcs = append(dcs, dc)
	}

	sort.Strings(dcs)
	return dcs
}

// HasDatacenter checks whether dc is defined in WAN
func (r *Router) HasDatacenter(dc string) bool {
	r.RLock()
	defer r.RUnlock()
	_, ok := r.managers[dc]
	return ok
}

// GetLANManager returns the Manager for the LAN area and the local datacenter
func (r *Router) GetLANManager() *Manager {
	r.RLock()
	defer r.RUnlock()

	area, ok := r.areas[types.AreaLAN]
	if !ok {
		return nil
	}

	managerInfo, ok := area.managers[r.localDatacenter]
	if !ok {
		return nil
	}

	return managerInfo.manager
}

// datacenterSorter takes a list of DC names and a parallel vector of distances
// and implements sort.Interface, keeping both structures coherent and sorting
// by distance.
type datacenterSorter struct {
	Names []string
	Vec   []float64
}

// See sort.Interface.
func (n *datacenterSorter) Len() int {
	return len(n.Names)
}

// See sort.Interface.
func (n *datacenterSorter) Swap(i, j int) {
	n.Names[i], n.Names[j] = n.Names[j], n.Names[i]
	n.Vec[i], n.Vec[j] = n.Vec[j], n.Vec[i]
}

// See sort.Interface.
func (n *datacenterSorter) Less(i, j int) bool {
	return n.Vec[i] < n.Vec[j]
}

// GetDatacentersByDistance returns a list of datacenters known to the router,
// sorted by median RTT from this server to the servers in each datacenter. If
// there are multiple areas that reach a given datacenter, this will use the
// lowest RTT for the sort.
func (r *Router) GetDatacentersByDistance() ([]string, error) {
	r.RLock()
	defer r.RUnlock()

	// Go through each area and aggregate the median RTT from the current
	// server to the other servers in each datacenter.
	dcs := make(map[string]float64)
	for areaID, info := range r.areas {
		index := make(map[string][]float64)
		coord, err := info.cluster.GetCoordinate()
		if err != nil {
			return nil, err
		}

		for _, m := range info.cluster.Members() {
			ok, parts := metadata.IsConsulServer(m)
			if !ok {
				if areaID != types.AreaLAN {
					r.logger.Warn("Non-server in server-only area",
						"non_server", m.Name,
						"area", areaID,
						"func", "GetDatacentersByDistance",
					)
				}
				continue
			}

			if m.Status == serf.StatusLeft {
				r.logger.Debug("server in area left, skipping",
					"server", m.Name,
					"area", areaID,
					"func", "GetDatacentersByDistance",
				)
				continue
			}

			existing := index[parts.Datacenter]
			if parts.Datacenter == r.localDatacenter {
				// Everything in the local datacenter looks like zero RTT.
				index[parts.Datacenter] = append(existing, 0.0)
			} else {
				// It's OK to get a nil coordinate back, ComputeDistance
				// will put the RTT at positive infinity.
				other, _ := info.cluster.GetCachedCoordinate(parts.Name)
				rtt := lib.ComputeDistance(coord, other)
				index[parts.Datacenter] = append(existing, rtt)
			}
		}

		// Compute the median RTT between this server and the servers
		// in each datacenter. We accumulate the lowest RTT to each DC
		// in the master map, since a given DC might appear in multiple
		// areas.
		for dc, rtts := range index {
			sort.Float64s(rtts)
			rtt := rtts[len(rtts)/2]

			current, ok := dcs[dc]
			if !ok || (ok && rtt < current) {
				dcs[dc] = rtt
			}
		}
	}

	// First sort by DC name, since we do a stable sort later.
	names := make([]string, 0, len(dcs))
	for dc := range dcs {
		names = append(names, dc)
	}
	sort.Strings(names)

	// Then stable sort by median RTT.
	rtts := make([]float64, 0, len(dcs))
	for _, dc := range names {
		rtts = append(rtts, dcs[dc])
	}
	sort.Stable(&datacenterSorter{names, rtts})
	return names, nil
}

// GetDatacenterMaps returns a structure with the raw network coordinates of
// each known server, organized by datacenter and network area.
func (r *Router) GetDatacenterMaps() ([]structs.DatacenterMap, error) {
	r.RLock()
	defer r.RUnlock()

	var maps []structs.DatacenterMap
	for areaID, info := range r.areas {
		index := make(map[string]structs.Coordinates)
		for _, m := range info.cluster.Members() {
			ok, parts := metadata.IsConsulServer(m)
			if !ok {
				if areaID != types.AreaLAN {
					r.logger.Warn("Non-server in server-only area",
						"non_server", m.Name,
						"area", areaID,
						"func", "GetDatacenterMaps",
					)
				}
				continue
			}

			if m.Status == serf.StatusLeft {
				r.logger.Debug("server in area left, skipping",
					"server", m.Name,
					"area", areaID,
					"func", "GetDatacenterMaps",
				)
				continue
			}

			coord, ok := info.cluster.GetCachedCoordinate(parts.Name)
			if ok {
				entry := &structs.Coordinate{
					Node:  parts.Name,
					Coord: coord,
				}
				existing := index[parts.Datacenter]
				index[parts.Datacenter] = append(existing, entry)
			}
		}

		for dc, coords := range index {
			entry := structs.DatacenterMap{
				Datacenter:  dc,
				AreaID:      areaID,
				Coordinates: coords,
			}
			maps = append(maps, entry)
		}
	}
	return maps, nil
}
