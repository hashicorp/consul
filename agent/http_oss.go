// +build !consulent

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/uiserver"
)

func (s *HTTPHandlers) parseEntMeta(req *http.Request, entMeta *structs.EnterpriseMeta) error {
	if headerNS := req.Header.Get("X-Consul-Namespace"); headerNS != "" {
		return BadRequestError{Reason: "Invalid header: \"X-Consul-Namespace\" - Namespaces are a Consul Enterprise feature"}
	}
	if queryNS := req.URL.Query().Get("ns"); queryNS != "" {
		return BadRequestError{Reason: "Invalid query parameter: \"ns\" - Namespaces are a Consul Enterprise feature"}
	}
	return nil
}

func (s *HTTPHandlers) validateEnterpriseIntentionNamespace(logName, ns string, _ bool) error {
	if ns == "" {
		return nil
	} else if strings.ToLower(ns) == structs.IntentionDefaultNamespace {
		return nil
	}

	// No special handling for wildcard namespaces as they are pointless in OSS.

	return BadRequestError{Reason: "Invalid " + logName + "(" + ns + ")" + ": Namespaces is a Consul Enterprise feature"}
}

func (s *HTTPHandlers) parseEntMetaNoWildcard(req *http.Request, _ *structs.EnterpriseMeta) error {
	return s.parseEntMeta(req, nil)
}

func (s *HTTPHandlers) rewordUnknownEnterpriseFieldError(err error) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	if strings.Contains(msg, "json: unknown field ") {
		quotedField := strings.TrimPrefix(msg, "json: unknown field ")

		switch quotedField {
		case `"Namespace"`:
			return fmt.Errorf("%v - Namespaces are a Consul Enterprise feature", err)
		}
	}

	return err
}

func parseACLAuthMethodEnterpriseMeta(req *http.Request, _ *structs.ACLAuthMethodEnterpriseMeta) error {
	if methodNS := req.URL.Query().Get("authmethod-ns"); methodNS != "" {
		return BadRequestError{Reason: "Invalid query parameter: \"authmethod-ns\" - Namespaces are a Consul Enterprise feature"}
	}

	return nil
}

// enterpriseHandler is a noop for the enterprise implementation. we pass the original back
func (s *HTTPHandlers) enterpriseHandler(next http.Handler) http.Handler {
	return next
}

// uiTemplateDataTransform returns an optional uiserver.UIDataTransform to allow
// altering UI data in enterprise.
func (s *HTTPHandlers) uiTemplateDataTransform() uiserver.UIDataTransform {
	return nil
}
