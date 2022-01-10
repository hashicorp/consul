//go:build !consulent
// +build !consulent

package agent

import (
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPHandlers) validateRequestPartition(_ http.ResponseWriter, _ *structs.EnterpriseMeta) bool {
	return true
}
