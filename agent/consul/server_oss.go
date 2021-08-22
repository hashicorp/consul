// +build !consulent

package consul

import (
	"github.com/hashicorp/serf/serf"
	"google.golang.org/grpc"
)

func (s *Server) removeFailedNodeEnterprise(remove func(*serf.Serf, string) error, node, wanNode string) error {
	// nothing to do for oss
	return nil
}

func (s *Server) registerEnterpriseGRPCServices(deps Deps, srv *grpc.Server) {}
