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
	Addr         net.Addr
	Status       serf.MemberStatus
	NonVoter     bool

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
	expectStr, ok := m.Tags["expect"]
	var err error
	if ok {
		expect, err = strconv.Atoi(expectStr)
		if err != nil {
			return false, nil
		}
	}

	portStr := m.Tags["port"]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return false, nil
	}

	segmentAddrs := make(map[string]string)
	segmentPorts := make(map[string]int)
	for name, value := range m.Tags {
		if strings.HasPrefix(name, "sl_") {
			addr, port, err := net.SplitHostPort(value)
			if err != nil {
				return false, nil
			}
			segmentPort, err := strconv.Atoi(port)
			if err != nil {
				return false, nil
			}

			segmentName := strings.TrimPrefix(name, "sl_")
			segmentAddrs[segmentName] = addr
			segmentPorts[segmentName] = segmentPort
		}
	}

	buildVersion, err := Build(&m)
	if err != nil {
		return false, nil
	}

	wanJoinPort := 0
	wanJoinPortStr, ok := m.Tags["wan_join_port"]
	if ok {
		wanJoinPort, err = strconv.Atoi(wanJoinPortStr)
		if err != nil {
			return false, nil
		}
	}

	vsnStr := m.Tags["vsn"]
	vsn, err := strconv.Atoi(vsnStr)
	if err != nil {
		return false, nil
	}

	raftVsn := 0
	raftVsnStr, ok := m.Tags["raft_vsn"]
	if ok {
		raftVsn, err = strconv.Atoi(raftVsnStr)
		if err != nil {
			return false, nil
		}
	}

	// Check if the server is a non voter
	_, nonVoter := m.Tags["nonvoter"]

	addr := &net.TCPAddr{IP: m.Addr, Port: port}

	parts := &Server{
		Name:         m.Name,
		ID:           m.Tags["id"],
		Datacenter:   datacenter,
		Segment:      segment,
		Port:         port,
		SegmentAddrs: segmentAddrs,
		SegmentPorts: segmentPorts,
		WanJoinPort:  wanJoinPort,
		Bootstrap:    bootstrap,
		Expect:       expect,
		Addr:         addr,
		Build:        *buildVersion,
		Version:      vsn,
		RaftVersion:  raftVsn,
		Status:       m.Status,
		UseTLS:       useTLS,
		NonVoter:     nonVoter,
	}
	return true, parts
}
