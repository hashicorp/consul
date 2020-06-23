package consul

import (
	"runtime"
	"strconv"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

// CanServersUnderstandProtocol checks to see if all the servers in the given
// list understand the given protocol version. If there are no servers in the
// list then this will return false.
func CanServersUnderstandProtocol(members []serf.Member, version uint8) (bool, error) {
	numServers, numWhoGrok := 0, 0
	for _, m := range members {
		if m.Tags["role"] != "consul" {
			continue
		}
		numServers++

		vsnMin, err := strconv.Atoi(m.Tags["vsn_min"])
		if err != nil {
			return false, err
		}

		vsnMax, err := strconv.Atoi(m.Tags["vsn_max"])
		if err != nil {
			return false, err
		}

		v := int(version)
		if (v >= vsnMin) && (v <= vsnMax) {
			numWhoGrok++
		}
	}
	return (numServers > 0) && (numWhoGrok == numServers), nil
}

// Returns if a member is a consul node. Returns a bool,
// and the datacenter.
func isConsulNode(m serf.Member) (bool, string) {
	if m.Tags["role"] != "node" {
		return false, ""
	}
	return true, m.Tags["dc"]
}

// runtimeStats is used to return various runtime information
func runtimeStats() map[string]string {
	return map[string]string{
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"version":    runtime.Version(),
		"max_procs":  strconv.FormatInt(int64(runtime.GOMAXPROCS(0)), 10),
		"goroutines": strconv.FormatInt(int64(runtime.NumGoroutine()), 10),
		"cpu_count":  strconv.FormatInt(int64(runtime.NumCPU()), 10),
	}
}

// checkServersProvider exists so that we can unit tests the requirements checking functions
// without having to spin up a whole agent/server.
type checkServersProvider interface {
	CheckServers(datacenter string, fn func(*metadata.Server) bool)
}

// serverRequirementsFn should inspect the given metadata.Server struct
// and return two booleans. The first indicates whether the given requirements
// are met. The second indicates whether this server should be considered filtered.
//
// The reason for the two booleans is so that a requirement function could "filter"
// out the left server members if we only want to consider things which are still
// around or likely to come back (failed state).
type serverRequirementFn func(*metadata.Server) (ok bool, filtered bool)

type serversMeetRequirementsState struct {
	// meetsRequirements is the callback to actual check for some specific requirement
	meetsRequirements serverRequirementFn

	// ok indicates whether all unfiltered servers meet the desired requirements
	ok bool

	// found is a boolean indicating that the meetsRequirement function accepted at
	// least one unfiltered server.
	found bool
}

func (s *serversMeetRequirementsState) update(srv *metadata.Server) bool {
	ok, filtered := s.meetsRequirements(srv)

	if filtered {
		// keep going but don't update any of the internal state as this server
		// was filtered by the requirements function
		return true
	}

	// mark that at least one server processed was not filtered
	s.found = true

	if !ok {
		// mark that at least one server does not meet the requirements
		s.ok = false

		// prevent continuing server evaluation
		return false
	}

	// this should already be set but this will prevent accidentally reusing
	// the state object from causing false-negatives.
	s.ok = true

	// continue evaluating servers
	return true
}

// ServersInDCMeetRequirements returns whether the given server members meet the requirements as defined by the
// callback function and whether at least one server remains unfiltered by the requirements function.
func ServersInDCMeetRequirements(provider checkServersProvider, datacenter string, meetsRequirements serverRequirementFn) (ok bool, found bool) {
	state := serversMeetRequirementsState{meetsRequirements: meetsRequirements, found: false, ok: true}

	provider.CheckServers(datacenter, state.update)

	return state.ok, state.found
}

// ServersInDCMeetMinimumVersion returns whether the given alive servers from a particular
// datacenter are at least on the given Consul version. This also returns whether any
// alive or failed servers are known in that datacenter (ignoring left and leaving ones)
func ServersInDCMeetMinimumVersion(provider checkServersProvider, datacenter string, minVersion *version.Version) (ok bool, found bool) {
	return ServersInDCMeetRequirements(provider, datacenter, func(srv *metadata.Server) (bool, bool) {
		if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
			// filter out the left servers as those should not be factored into our requirements
			return true, true
		}

		return !srv.Build.LessThan(minVersion), false
	})
}

// CheckServers implements the checkServersProvider interface for the Server
func (s *Server) CheckServers(datacenter string, fn func(*metadata.Server) bool) {
	if datacenter == s.config.Datacenter {
		// use the ServerLookup type for the local DC
		s.serverLookup.CheckServers(fn)
	} else {
		// use the router for all non-local DCs
		s.router.CheckServers(datacenter, fn)
	}
}

// CheckServers implements the checkServersProvider interface for the Client
func (c *Client) CheckServers(datacenter string, fn func(*metadata.Server) bool) {
	if datacenter != c.config.Datacenter {
		return
	}

	c.routers.CheckServers(fn)
}

type serversACLMode struct {
	// leader is the address of the leader
	leader string

	// mode indicates the overall ACL mode of the servers
	mode structs.ACLMode

	// leaderMode is the ACL mode of the leader server
	leaderMode structs.ACLMode

	// indicates that at least one server was processed
	found bool
}

func (s *serversACLMode) init(leader string) {
	s.leader = leader
	s.mode = structs.ACLModeEnabled
	s.leaderMode = structs.ACLModeUnknown
	s.found = false
}

func (s *serversACLMode) update(srv *metadata.Server) bool {
	if srv.Status != serf.StatusAlive && srv.Status != serf.StatusFailed {
		// they are left or something so regardless we treat these servers as meeting
		// the version requirement
		return true
	}

	// mark that we processed at least one server
	s.found = true

	if srvAddr := srv.Addr.String(); srvAddr == s.leader {
		s.leaderMode = srv.ACLs
	}

	switch srv.ACLs {
	case structs.ACLModeDisabled:
		// anything disabled means we cant enable ACLs
		s.mode = structs.ACLModeDisabled
	case structs.ACLModeEnabled:
		// do nothing
	case structs.ACLModeLegacy:
		// This covers legacy mode and older server versions that don't advertise ACL support
		if s.mode != structs.ACLModeDisabled && s.mode != structs.ACLModeUnknown {
			s.mode = structs.ACLModeLegacy
		}
	default:
		if s.mode != structs.ACLModeDisabled {
			s.mode = structs.ACLModeUnknown
		}
	}

	return true
}

// ServersGetACLMode checks all the servers in a particular datacenter and determines
// what the minimum ACL mode amongst them is and what the leaders ACL mode is.
// The "found" return value indicates whether there were any servers considered in
// this datacenter. If that is false then the other mode return values are meaningless
// as they will be ACLModeEnabled and ACLModeUnkown respectively.
func ServersGetACLMode(provider checkServersProvider, leaderAddr string, datacenter string) (found bool, mode structs.ACLMode, leaderMode structs.ACLMode) {
	var state serversACLMode
	state.init(leaderAddr)

	provider.CheckServers(datacenter, state.update)

	return state.found, state.mode, state.leaderMode
}
