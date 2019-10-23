// +build !consulent

package agent

import (
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPServer) parseEntMeta(req *http.Request, entMeta *structs.EnterpriseMeta) {
}
