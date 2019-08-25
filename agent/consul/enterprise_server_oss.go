// +build !consulent

package consul

import (
	"net"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"
)

var (
	// minMultiDCConnectVersion is the minimum version in order to support multi-DC Connect
	// features.
	minMultiDCConnectVersion = version.Must(version.NewVersion("1.6.0"))
)

type EnterpriseServer struct{}

func (s *Server) initEnterprise() error {
	return nil
}

func (s *Server) startEnterprise() error {
	return nil
}

func (s *Server) handleEnterpriseUserEvents(event serf.UserEvent) bool {
	return false
}

func (s *Server) handleEnterpriseRPCConn(rtype pool.RPCType, conn net.Conn, isTLS bool) bool {
	return false
}

func (s *Server) enterpriseStats() map[string]map[string]string {
	return nil
}
