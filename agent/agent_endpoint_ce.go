//go:build !consulent
// +build !consulent

package agent

import (
	"net/http"

	"github.com/hashicorp/consul/acl"
)

func (s *HTTPHandlers) validateRequestPartition(_ http.ResponseWriter, _ *acl.EnterpriseMeta) bool {
	return true
}

func (s *HTTPHandlers) defaultMetaPartitionToAgent(entMeta *acl.EnterpriseMeta) {
}
