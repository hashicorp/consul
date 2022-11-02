//go:build !consulent
// +build !consulent

package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPHandlers) parseEntMeta(req *http.Request, entMeta *acl.EnterpriseMeta) error {
	if headerNS := req.Header.Get("X-Consul-Namespace"); headerNS != "" {
		return HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason:     "Invalid header: \"X-Consul-Namespace\" - Namespaces are a Consul Enterprise feature",
		}
	}
	if queryNS := req.URL.Query().Get("ns"); queryNS != "" {
		return HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason:     "Invalid query parameter: \"ns\" - Namespaces are a Consul Enterprise feature",
		}
	}

	return s.parseEntMetaPartition(req, entMeta)
}

func (s *HTTPHandlers) validateEnterpriseIntentionPartition(logName, partition string) error {
	if partition == "" {
		return nil
	} else if strings.ToLower(partition) == "default" {
		return nil
	}

	// No special handling for wildcard namespaces as they are pointless in OSS.

	return HTTPError{
		StatusCode: http.StatusBadRequest,
		Reason:     "Invalid " + logName + "(" + partition + ")" + ": Partitions is a Consul Enterprise feature",
	}
}

func (s *HTTPHandlers) validateEnterpriseIntentionNamespace(logName, ns string, _ bool) error {
	if ns == "" {
		return nil
	} else if strings.ToLower(ns) == structs.IntentionDefaultNamespace {
		return nil
	}

	// No special handling for wildcard namespaces as they are pointless in OSS.

	return HTTPError{
		StatusCode: http.StatusBadRequest,
		Reason:     "Invalid " + logName + "(" + ns + ")" + ": Namespaces is a Consul Enterprise feature",
	}
}

func (s *HTTPHandlers) parseEntMetaNoWildcard(req *http.Request, _ *acl.EnterpriseMeta) error {
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
		return HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason:     "Invalid query parameter: \"authmethod-ns\" - Namespaces are a Consul Enterprise feature",
		}
	}

	return nil
}

// enterpriseHandler is a noop for the enterprise implementation. we pass the original back
func (s *HTTPHandlers) enterpriseHandler(next http.Handler) http.Handler {
	return next
}

// uiTemplateDataTransform returns an optional uiserver.UIDataTransform to allow
// altering UI data in enterprise.
func (s *HTTPHandlers) uiTemplateDataTransform(data map[string]interface{}) error {
	return nil
}

func (s *HTTPHandlers) parseEntMetaPartition(req *http.Request, meta *acl.EnterpriseMeta) error {
	if headerAP := req.Header.Get("X-Consul-Partition"); headerAP != "" {
		return HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason:     "Invalid header: \"X-Consul-Partition\" - Partitions are a Consul Enterprise feature",
		}
	}
	if queryAP := req.URL.Query().Get("partition"); queryAP != "" {
		return HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason:     "Invalid query parameter: \"partition\" - Partitions are a Consul Enterprise feature",
		}
	}

	return nil
}
