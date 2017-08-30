// +build !ent

package consul

import (
	"errors"

	"github.com/hashicorp/serf/serf"
)

const (
	errSegmentsNotSupported = "network segments are not supported in this version of Consul"
)

var (
	ErrSegmentsNotSupported = errors.New(errSegmentsNotSupported)
)

// LANSegmentMembers is used to return the members of the given LAN segment.
func (s *Server) LANSegmentMembers(segment string) ([]serf.Member, error) {
	if segment == "" {
		return s.LANMembers(), nil
	}

	return nil, ErrSegmentsNotSupported
}

// LANSegmentAddr is used to return the address used for the given LAN segment.
func (s *Server) LANSegmentAddr(name string) string {
	return ""
}

// setupSegments returns an error if any segments are defined since the OSS
// version of Consul doens't support them.
func (s *Server) setupSegments(config *Config, port int) error {
	if len(config.Segments) > 0 {
		return ErrSegmentsNotSupported
	}

	return nil
}

// floodSegments is a NOP in the OSS version of Consul.
func (s *Server) floodSegments(config *Config) {
}
