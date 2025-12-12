// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/version"
)

func (s *HTTPHandlers) OperatorUtilizationEndpoint(resp http.ResponseWriter, req *http.Request) (any, error) {
	if !version.IsEnterprise() {
		return nil, HTTPError{
			StatusCode: http.StatusNotFound,
			Reason:     "operator utilization requires Consul Enterprise",
		}
	}

	if req.Method != http.MethodGet {
		return nil, MethodNotAllowedError{Method: req.Method, Allow: []string{"GET"}}
	}

	var args structs.UtilizationBundleRequest

	// Parse namespace/partition context first to ensure request is well scoped.
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Parse the standard read query options and ACL token information.
	var dc string
	if done := s.parse(resp, req, &dc, &args.QueryOptions); done {
		return nil, nil
	}
	args.Datacenter = dc

	query := req.URL.Query()
	args.Message = query.Get("message")

	if raw := query.Get("today_only"); raw != "" {
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("invalid value for today_only: %q", raw)}
		}
		args.TodayOnly = val
	}

	if raw := query.Get("send_report"); raw != "" {
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("invalid value for send_report: %q", raw)}
		}
		args.SendReport = val
	}

	var reply structs.UtilizationBundleResponse
	if err := s.agent.RPC(req.Context(), "Operator.UtilizationBundle", &args, &reply); err != nil {
		return nil, err
	}

	if err := setMeta(resp, &reply.QueryMeta); err != nil {
		return nil, err
	}

	if len(reply.Bundle) == 0 {
		return map[string]string{"message": "no utilization data available"}, nil
	}

	resp.Header().Set(contentTypeHeader, "application/json")
	resp.Header().Set("Content-Disposition", "attachment; filename=\"consul-utilization-bundle.json\"")
	resp.WriteHeader(http.StatusOK)
	_, err := resp.Write(reply.Bundle)
	return nil, err
}
