// +build !consulent

package consul

import (
	"github.com/hashicorp/serf/serf"
)

func (s *Server) removeFailedNodeEnterprise(remove func(*serf.Serf, string) error, node, wanNode string) error {
	// nothing to do for oss
	return nil
}
