package servers

import (
	"fmt"
	"log"
	"sync"

	"github.com/hashicorp/consul/consul/agent"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/serf/coordinate"
	"github.com/hashicorp/serf/serf"
)

type Router struct {
	logger *log.Logger

	areas    map[types.AreaID]*areaInfo
	managers map[string][]*Manager

	// This top-level lock covers all the internal state.
	sync.RWMutex
}

// RouterSerfCluster is an interface wrapper around Serf in order to make this
// easier to unit test.
type RouterSerfCluster interface {
	NumNodes() int
	Members() []serf.Member
	GetCoordinate() (*coordinate.Coordinate, error)
	GetCachedCoordinate(name string) (coord *coordinate.Coordinate, ok bool)
}

type managerInfo struct {
	manager    *Manager
	shutdownCh chan struct{}
}

type areaInfo struct {
	cluster  RouterSerfCluster
	pinger   Pinger
	managers map[string]*managerInfo
}

func NewRouter(logger *log.Logger, shutdownCh chan struct{}) *Router {
	router := &Router{
		logger:   logger,
		areas:    make(map[types.AreaID]*areaInfo),
		managers: make(map[string][]*Manager),
	}

	// This will propagate a top-level shutdown to all the managers.
	go func() {
		<-shutdownCh
		router.Lock()
		defer router.Unlock()

		for _, area := range router.areas {
			for _, info := range area.managers {
				close(info.shutdownCh)
			}
		}

		router.areas = nil
		router.managers = nil
	}()

	return router
}

func (r *Router) AddArea(areaID types.AreaID, cluster RouterSerfCluster, pinger Pinger) error {
	r.Lock()
	defer r.Unlock()

	if _, ok := r.areas[areaID]; ok {
		return fmt.Errorf("area ID %q already exists", areaID)
	}

	r.areas[areaID] = &areaInfo{
		cluster:  cluster,
		pinger:   pinger,
		managers: make(map[string]*managerInfo),
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
			return
		}
	}
	panic("managers index out of sync")
}

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

func (r *Router) AddServer(areaID types.AreaID, s *agent.Server) error {
	r.Lock()
	defer r.Unlock()

	area, ok := r.areas[areaID]
	if !ok {
		return fmt.Errorf("area ID %q does not exist", areaID)
	}

	// Make the manager on the fly if this is the first we've seen of it,
	// and add it to the index.
	info, ok := area.managers[s.Datacenter]
	if !ok {
		shutdownCh := make(chan struct{})
		manager := New(r.logger, shutdownCh, area.cluster, area.pinger)
		info = &managerInfo{
			manager:    manager,
			shutdownCh: shutdownCh,
		}

		managers := r.managers[s.Datacenter]
		r.managers[s.Datacenter] = append(managers, manager)
	}

	info.manager.AddServer(s)
	return nil
}

func (r *Router) RemoveServer(areaID types.AreaID, s *agent.Server) error {
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

	// If this manager is empty then remove it so we don't accumulate cruft
	// and waste time during request routing.
	if num := info.manager.NumServers(); num == 0 {
		r.removeManagerFromIndex(s.Datacenter, info.manager)
		close(info.shutdownCh)
		delete(area.managers, s.Datacenter)
	}

	return nil
}

func (r *Router) FailServer(areaID types.AreaID, s *agent.Server) error {
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

func (r *Router) GetDatacenters() []string {
	r.RLock()
	defer r.RUnlock()

	dcs := make([]string, 0, len(r.managers))
	for dc, _ := range r.managers {
		dcs = append(dcs, dc)
	}
	return dcs
}

func (r *Router) GetDatacenterMaps() ([]structs.DatacenterMap, error) {
	r.RLock()
	defer r.RUnlock()

	var maps []structs.DatacenterMap
	for areaID, info := range r.areas {
		index := make(map[string]structs.Coordinates)
		for _, m := range info.cluster.Members() {
			ok, parts := agent.IsConsulServer(m)
			if !ok {
				r.logger.Printf("[WARN]: consul: Non-server %q in server-only area %q",
					m.Name, areaID)
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

func (r *Router) FindRoute(datacenter string) (*Manager, *agent.Server, bool) {
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
		if s := manager.FindServer(); s != nil {
			return manager, s, true
		}
	}

	// Didn't find a route (even via an unhealthy server).
	return nil, nil, false
}
