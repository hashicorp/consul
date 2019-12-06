// +build !consulent

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPServer) parseEntMeta(req *http.Request, entMeta *structs.EnterpriseMeta) error {
	if headerNS := req.Header.Get("X-Consul-Namespace"); headerNS != "" {
		return BadRequestError{Reason: "Invalid header: \"X-Consul-Namespace\" - Namespaces is a Consul Enterprise feature"}
	}
	if queryNS := req.URL.Query().Get("ns"); queryNS != "" {
		return BadRequestError{Reason: "Invalid query parameter: \"ns\" - Namespaces is a Consul Enterprise feature"}
	}
	return nil
}

func (s *HTTPServer) rewordUnknownEnterpriseFieldError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	if strings.Contains(msg, "json: unknown field ") {
		quotedField := strings.TrimPrefix(msg, "json: unknown field ")

		switch quotedField {
		case `"Namespace"`:
			return fmt.Errorf("%v - Namespaces is a Consul Enterprise feature", err)
		}
	}

	return err
}
