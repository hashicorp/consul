//go:build !consulent
// +build !consulent

package consul

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/pool"
	"github.com/hashicorp/consul/agent/structs"
)

var (
	// minMultiDCConnectVersion is the minimum version in order to support multi-DC Connect
	// features.
	minMultiDCConnectVersion = version.Must(version.NewVersion("1.6.0"))
)

type EnterpriseServer struct{}

func (s *Server) initEnterprise(_ Deps) error {
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

func (s *Server) handleEnterpriseNativeTLSConn(alpnProto string, conn net.Conn) bool {
	return false
}

func (s *Server) handleEnterpriseLeave() {
	return
}

func (s *Server) establishEnterpriseLeadership(_ context.Context) error {
	return nil
}

func (s *Server) revokeEnterpriseLeadership() error {
	return nil
}

func (s *Server) validateEnterpriseRequest(entMeta *structs.EnterpriseMeta, write bool) error {
	return nil
}

func (s *Server) validateEnterpriseIntentionPartition(partition string) error {
	if partition == "" {
		return nil
	} else if strings.ToLower(partition) == "default" {
		return nil
	}

	// No special handling for wildcard partitions as they are pointless in OSS.

	return errors.New("Partitions is a Consul Enterprise feature")
}

func (s *Server) validateEnterpriseIntentionNamespace(ns string, _ bool) error {
	if ns == "" {
		return nil
	} else if strings.ToLower(ns) == structs.IntentionDefaultNamespace {
		return nil
	}

	// No special handling for wildcard namespaces as they are pointless in OSS.

	return errors.New("Namespaces is a Consul Enterprise feature")
}

func addEnterpriseSerfTags(_ map[string]string, _ *structs.EnterpriseMeta) {
	// do nothing
}
