package metadata

import (
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

// Key is used in maps and for equality tests.  A key is based on endpoints.
type Key struct {
	name string
}

// Equal compares two Key objects
func (k *Key) Equal(x *Key) bool {
	return k.name == x.name
}

// Server is used to return details of a consul server
type Server struct {
	Name         string
	ID           string
	Datacenter   string
	Segment      string
	Port         int
	SegmentAddrs map[string]string
	SegmentPorts map[string]int
	WanJoinPort  int
	Bootstrap    bool
	Expect       int
	Build        version.Version
	Version      int
	RaftVersion  int
	NonVoter     bool
	Addr         net.Addr
	Status       serf.MemberStatus

	// If true, use TLS when connecting to this server
	UseTLS bool
}

// Key returns the corresponding Key
func (s *Server) Key() *Key {
	return &Key{
		name: s.Name,
	}
}

// String returns a string representation of Server
func (s *Server) String() string {
	var addrStr, networkStr string
	if s.Addr != nil {
		addrStr = s.Addr.String()
		networkStr = s.Addr.Network()
	}

	return fmt.Sprintf("%s (Addr: %s/%s) (DC: %s)", s.Name, networkStr, addrStr, s.Datacenter)
}

var versionFormat = regexp.MustCompile(`\d+\.\d+\.\d+`)

// IsConsulServer returns true if a serf member is a consul server
// agent. Returns a bool and a pointer to the Server.
func IsConsulServer(m serf.Member) (bool, *Server) {
	if m.Tags["role"] != "consul" {
		return false, nil
	}

	datacenter := m.Tags["dc"]
	segment := m.Tags["segment"]
	_, bootstrap := m.Tags["bootstrap"]
	_, useTLS := m.Tags["use_tls"]

	expect := 0
	expect_str, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expect_str)
		if err != nil {
			return false, nil
		}
	}

	port_str := m.Tags["port"]
	port, err := strconv.Atoi(port_str)
	if err != nil {
		return false, nil
	}

	segment_addrs := make(map[string]string)
	segment_ports := make(map[string]int)
	for name, value := range m.Tags {
		if strings.HasPrefix(name, "sl_") {
			addr, port, err := net.SplitHostPort(value)
			if err != nil {
				return false, nil
			}
			segment_port, err := strconv.Atoi(port)
			if err != nil {
				return false, nil
			}

			segment_name := strings.TrimPrefix(name, "sl_")
			segment_addrs[segment_name] = addr
			segment_ports[segment_name] = segment_port
		}
	}

	build_version, err := Build(&m)
	if err != nil {
		return false, nil
	}

	wan_join_port := 0
	wan_join_port_str, ok := m.Tags["wan_join_port"]
	if ok {
		wan_join_port, err = strconv.Atoi(wan_join_port_str)
		if err != nil {
			return false, nil
		}
	}

	vsn_str := m.Tags["vsn"]
	vsn, err := strconv.Atoi(vsn_str)
	if err != nil {
		return false, nil
	}

	raft_vsn := 0
	raft_vsn_str, ok := m.Tags["raft_vsn"]
	if ok {
		raft_vsn, err = strconv.Atoi(raft_vsn_str)
		if err != nil {
			return false, nil
		}
	}

	_, nonVoter := m.Tags["nonvoter"]

	addr := &net.TCPAddr{IP: m.Addr, Port: port}

	parts := &Server{
		Name:         m.Name,
		ID:           m.Tags["id"],
		Datacenter:   datacenter,
		Segment:      segment,
		Port:         port,
		SegmentAddrs: segment_addrs,
		SegmentPorts: segment_ports,
		WanJoinPort:  wan_join_port,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		Build:        *build_version,
		Version:      vsn,
		RaftVersion:  raft_vsn,
		Status:       m.Status,
		NonVoter:     nonVoter,
		UseTLS:       useTLS,
	}
	return true, parts
}
