// +build !ent

package consul

import (
	"net"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/serf/serf"
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
