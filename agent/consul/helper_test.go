package consul

import (
	"fmt"
	"testing"
)

// wantPeers determines whether the server has the given
// number of peers.
func wantPeers(s *Server, peers int) error {
	n, err := s.numPeers()
	if err != nil {
		return err
	}
	if got, want := n, peers; got != want {
		return fmt.Errorf("got %d peers want %d", got, want)
	}
	return nil
}

// joinAddrLAN returns the address other servers can
// use to join the cluster on the LAN interface.
func joinAddrLAN(s *Server) string {
	if s == nil {
		panic("no server")
	}
	port := s.config.SerfLANConfig.MemberlistConfig.BindPort
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// joinAddrWAN returns the address other servers can
// use to join the cluster on the WAN interface.
func joinAddrWAN(s *Server) string {
	if s == nil {
		panic("no server")
	}
	port := s.config.SerfWANConfig.MemberlistConfig.BindPort
	return fmt.Sprintf("127.0.0.1:%d", port)
}

type clientOrServer interface {
	JoinLAN(addrs []string) (int, error)
}

// joinLAN is a convenience function for
//
//   member.JoinLAN("127.0.0.1:"+leader.config.SerfLANConfig.MemberlistConfig.BindPort)
func joinLAN(t *testing.T, member clientOrServer, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	addr := []string{joinAddrLAN(leader)}
	if _, err := member.JoinLAN(addr); err != nil {
		t.Fatal(err)
	}
}

// joinWAN is a convenience function for
//
//   member.JoinWAN("127.0.0.1:"+leader.config.SerfWANConfig.MemberlistConfig.BindPort)
func joinWAN(t *testing.T, member, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	addr := []string{joinAddrWAN(leader)}
	if _, err := member.JoinWAN(addr); err != nil {
		t.Fatal(err)
	}
}
