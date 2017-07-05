package consul

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/serf/serf"
)

func waitForLeader(servers ...*Server) error {
	if len(servers) == 0 {
		return errors.New("no servers")
	}
	dc := servers[0].config.Datacenter
	for _, s := range servers {
		if s.config.Datacenter != dc {
			return fmt.Errorf("servers are in different datacenters %s and %s", s.config.Datacenter, dc)
		}
	}
	for _, s := range servers {
		if s.IsLeader() {
			return nil
		}
	}
	return errors.New("no leader")
}

// wantPeers determines whether the server has the given
// number of raft peers.
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
	LANMembers() []serf.Member
}

// joinLAN is a convenience function for
//
//   member.JoinLAN("127.0.0.1:"+leader.config.SerfLANConfig.MemberlistConfig.BindPort)
func joinLAN(t *testing.T, member clientOrServer, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	var memberAddr string
	switch x := member.(type) {
	case *Server:
		memberAddr = joinAddrLAN(x)
	case *Client:
		memberAddr = fmt.Sprintf("127.0.0.1:%d", x.config.SerfLANConfig.MemberlistConfig.BindPort)
	}
	leaderAddr := joinAddrLAN(leader)
	if _, err := member.JoinLAN([]string{leaderAddr}); err != nil {
		t.Fatal(err)
	}
	retry.Run(t, func(r *retry.R) {
		if !seeEachOther(leader.LANMembers(), member.LANMembers(), leaderAddr, memberAddr) {
			r.Fatalf("leader and member cannot see each other on LAN")
		}
	})
	if !seeEachOther(leader.LANMembers(), member.LANMembers(), leaderAddr, memberAddr) {
		t.Fatalf("leader and member cannot see each other on LAN")
	}
}

// joinWAN is a convenience function for
//
//   member.JoinWAN("127.0.0.1:"+leader.config.SerfWANConfig.MemberlistConfig.BindPort)
func joinWAN(t *testing.T, member, leader *Server) {
	if member == nil || leader == nil {
		panic("no server")
	}
	leaderAddr, memberAddr := joinAddrWAN(leader), joinAddrWAN(member)
	if _, err := member.JoinWAN([]string{leaderAddr}); err != nil {
		t.Fatal(err)
	}
	retry.Run(t, func(r *retry.R) {
		if !seeEachOther(leader.WANMembers(), member.WANMembers(), leaderAddr, memberAddr) {
			r.Fatalf("leader and member cannot see each other on WAN")
		}
	})
	if !seeEachOther(leader.WANMembers(), member.WANMembers(), leaderAddr, memberAddr) {
		t.Fatalf("leader and member cannot see each other on WAN")
	}
}

func seeEachOther(a, b []serf.Member, addra, addrb string) bool {
	return serfMembersContains(a, addrb) && serfMembersContains(b, addra)
}

func serfMembersContains(members []serf.Member, addr string) bool {
	// There are tests that manipulate the advertise address, so we just
	// compare port numbers here, since that uniquely identifies a member
	// as we use the loopback interface for everything.
	_, want, err := net.SplitHostPort(addr)
	if err != nil {
		panic(err)
	}
	for _, m := range members {
		if got := fmt.Sprintf("%d", m.Port); got == want {
			return true
		}
	}
	return false
}
