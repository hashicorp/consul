package consul

import (
	"encoding/binary"
	"fmt"
	"net"
	"runtime"
	"strconv"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

/*
 * Contains an entry for each private block:
 * 10.0.0.0/8
 * 100.64.0.0/10
 * 127.0.0.0/8
 * 169.254.0.0/16
 * 172.16.0.0/12
 * 192.168.0.0/16
 */
var privateBlocks []*net.IPNet

func init() {
	// Add each private block
	privateBlocks = make([]*net.IPNet, 6)

	_, block, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[0] = block

	_, block, err = net.ParseCIDR("100.64.0.0/10")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[1] = block

	_, block, err = net.ParseCIDR("127.0.0.0/8")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[2] = block

	_, block, err = net.ParseCIDR("169.254.0.0/16")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[3] = block

	_, block, err = net.ParseCIDR("172.16.0.0/12")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[4] = block

	_, block, err = net.ParseCIDR("192.168.0.0/16")
	if err != nil {
		panic(fmt.Sprintf("Bad cidr. Got %v", err))
	}
	privateBlocks[5] = block
}

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

// Returns if the given IP is in a private block
func isPrivateIP(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	for _, priv := range privateBlocks {
		if priv.Contains(ip) {
			return true
		}
	}
	return false
}

// Returns addresses from interfaces that is up
func activeInterfaceAddresses() ([]net.Addr, error) {
	var upAddrs []net.Addr
	var loAddrs []net.Addr

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interfaces: %v", err)
	}

	for _, iface := range interfaces {
		// Require interface to be up
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
		}

		if iface.Flags&net.FlagLoopback != 0 {
			loAddrs = append(loAddrs, addresses...)
			continue
		}

		upAddrs = append(upAddrs, addresses...)
	}

	if len(upAddrs) == 0 {
		return loAddrs, nil
	}

	return upAddrs, nil
}

// GetPrivateIP is used to return the first private IP address
// associated with an interface on the machine
func GetPrivateIP() (net.IP, error) {
	addresses, err := activeInterfaceAddresses()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
	}

	return getPrivateIP(addresses)
}

func getPrivateIP(addresses []net.Addr) (net.IP, error) {
	var candidates []net.IP

	// Find private IPv4 address
	for _, rawAddr := range addresses {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}

		if ip.To4() == nil {
			continue
		}
		if !isPrivateIP(ip.String()) {
			continue
		}
		candidates = append(candidates, ip)
	}
	numIps := len(candidates)
	switch numIps {
	case 0:
		return nil, fmt.Errorf("No private IP address found")
	case 1:
		return candidates[0], nil
	default:
		return nil, fmt.Errorf("Multiple private IPs found. Please configure one.")
	}

}

// GetPublicIPv6 is used to return the first public IP address
// associated with an interface on the machine
func GetPublicIPv6() (net.IP, error) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return nil, fmt.Errorf("Failed to get interface addresses: %v", err)
	}

	return getPublicIPv6(addresses)
}

func isUniqueLocalAddress(ip net.IP) bool {
	return len(ip) == net.IPv6len && ip[0] == 0xfc && ip[1] == 0x00
}

func getPublicIPv6(addresses []net.Addr) (net.IP, error) {
	var candidates []net.IP

	// Find public IPv6 address
	for _, rawAddr := range addresses {
		var ip net.IP
		switch addr := rawAddr.(type) {
		case *net.IPAddr:
			ip = addr.IP
		case *net.IPNet:
			ip = addr.IP
		default:
			continue
		}

		if ip.To4() != nil {
			continue
		}

		if ip.IsLinkLocalUnicast() || isUniqueLocalAddress(ip) || ip.IsLoopback() {
			continue
		}
		candidates = append(candidates, ip)
	}
	numIps := len(candidates)
	switch numIps {
	case 0:
		return nil, fmt.Errorf("No public IPv6 address found")
	case 1:
		return candidates[0], nil
	default:
		return nil, fmt.Errorf("Multiple public IPv6 addresses found. Please configure one.")
	}
}

// Converts bytes to an integer
func bytesToUint64(b []byte) uint64 {
	return binary.BigEndian.Uint64(b)
}

// Converts a uint to a byte slice
func uint64ToBytes(u uint64) []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
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
