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
	Name             string // <node>.<dc>
	ShortName        string // <node>
	ID               string
	Datacenter       string
	Segment          string
	Port             int
	SegmentAddrs     map[string]string
	SegmentPorts     map[string]int
	WanJoinPort      int
	LanJoinPort      int
	ExternalGRPCPort int
	Bootstrap        bool
	Expect           int
	Build            version.Version
	Version          int
	RaftVersion      int
	Addr             net.Addr
	Status           serf.MemberStatus
	ReadReplica      bool
	FeatureFlags     map[string]int

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
	featureFlags := make(map[string]int)
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
		} else if strings.HasPrefix(name, featureFlagPrefix) {
			featureName := strings.TrimPrefix(name, featureFlagPrefix)
			featureState, err := strconv.Atoi(value)
			if err != nil {
				return false, nil
			}
			featureFlags[featureName] = featureState
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

	externalGRPCPort := 0
	externalGRPCPortStr, ok := m.Tags["grpc_port"]
	if ok {
		externalGRPCPort, err = strconv.Atoi(externalGRPCPortStr)
		if err != nil {
			return false, nil
		}
		if externalGRPCPort < 1 {
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
	// DEPRECATED - remove looking for the nonvoter tag eventually once we don't have to support
	// read replicas running v1.8.x and below.
	_, nonVoter := m.Tags["nonvoter"]
	_, readReplica := m.Tags["read_replica"]

	addr := &net.TCPAddr{IP: m.Addr, Port: port}

	parts := &Server{
		Name:             m.Name,
		ShortName:        strings.TrimSuffix(m.Name, "."+datacenter),
		ID:               m.Tags["id"],
		Datacenter:       datacenter,
		Segment:          segment,
		Port:             port,
		SegmentAddrs:     segmentAddrs,
		SegmentPorts:     segmentPorts,
		WanJoinPort:      wanJoinPort,
		LanJoinPort:      int(m.Port),
		ExternalGRPCPort: externalGRPCPort,
		Bootstrap:        bootstrap,
		Expect:           expect,
		Addr:             addr,
		Build:            *buildVersion,
		Version:          vsn,
		RaftVersion:      raftVsn,
		Status:           m.Status,
		UseTLS:           useTLS,
		// DEPRECATED - remove nonVoter check once support for that tag is removed
		ReadReplica:  nonVoter || readReplica,
		FeatureFlags: featureFlags,
	}
	return true, parts
}

// TODO(ACL-Legacy-Compat): remove in phase 2
const TagACLs = "acls"

const featureFlagPrefix = "ft_"

// AddFeatureFlags to the tags. The tags map is expected to be a serf.Config.Tags.
// The feature flags are encoded in the tags so that IsConsulServer can decode them
// and populate the Server.FeatureFlags map.
func AddFeatureFlags(tags map[string]string, flags ...string) {
	for _, flag := range flags {
		tags[featureFlagPrefix+flag] = "1"
	}
}
