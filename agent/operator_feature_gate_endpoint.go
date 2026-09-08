// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func (s *HTTPHandlers) OperatorFeatureGateList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.FeatureGateQueryRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	var reply structs.FeatureGateQueryResponse
	if err := s.agent.RPC(req.Context(), "Operator.FeatureGateGet", &args, &reply); err != nil {
		return nil, err
	}
	defer setMeta(resp, &reply.QueryMeta)
	if reply.Uninitialized {
		resp.Header().Set("X-Consul-Feature-Gates-Uninitialized", "true")
	}

	features := make([]api.FeatureGate, 0, len(reply.Features))
	for _, feature := range reply.Features {
		features = append(features, featureGateToAPI(feature))
	}
	return features, nil
}

func (s *HTTPHandlers) OperatorFeatureGate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/operator/feature/")
	if name == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "feature gate name is required"}
	}

	switch req.Method {
	case http.MethodGet:
		args := structs.FeatureGateQueryRequest{Name: name}
		if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
			return nil, nil
		}
		var reply structs.FeatureGateQueryResponse
		if err := s.agent.RPC(req.Context(), "Operator.FeatureGateGet", &args, &reply); err != nil {
			return nil, err
		}
		defer setMeta(resp, &reply.QueryMeta)
		if reply.Uninitialized {
			return nil, HTTPError{StatusCode: http.StatusServiceUnavailable, Reason: "feature-gate policy is not yet initialized; retry after the leader has committed the first policy generation"}
		}
		if len(reply.Features) != 1 {
			return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: fmt.Sprintf("feature gate %q not found", name)}
		}
		return featureGateToAPI(reply.Features[0]), nil

	case http.MethodPut:
		var body api.FeatureGateSetRequest
		if err := decodeBody(req.Body, &body); err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("error parsing feature gate setting: %v", err)}
		}
		args := structs.FeatureGateSetRequest{Name: name, Enabled: body.Enabled}
		s.parseDC(req, &args.Datacenter)
		s.parseToken(req, &args.Token)
		if rawCAS := req.URL.Query().Get("cas"); rawCAS != "" {
			cas, err := strconv.ParseUint(rawCAS, 10, 64)
			if err != nil {
				return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("error parsing cas value: %v", err)}
			}
			args.ExpectedPolicyIndex = cas
		}

		var reply structs.FeatureGateSetResponse
		if err := s.agent.RPC(req.Context(), "Operator.FeatureGateSet", &args, &reply); err != nil {
			return nil, err
		}
		return api.FeatureGateSetResponse{Applied: reply.Applied, Feature: featureGateToAPI(reply.Feature)}, nil

	default:
		return nil, MethodNotAllowedError{req.Method, []string{http.MethodGet, http.MethodPut}}
	}
}

func featureGateToAPI(feature structs.FeatureGateInfo) api.FeatureGate {
	return api.FeatureGate{
		Name:                 feature.Name,
		Stage:                feature.Stage,
		MinVersion:           feature.MinVersion,
		DefaultEnabled:       feature.DefaultEnabled,
		BeforeMinimumVersion: feature.BeforeMinimumVersion,
		Description:          feature.Description,
		Owner:                feature.Owner,
		DesiredEnabled:       feature.DesiredEnabled,
		EffectiveEnabled:     feature.EffectiveEnabled,
		Eligible:             feature.Eligible,
		Source:               feature.Source,
		Reason:               string(feature.Reason),
		PolicyIndex:          feature.PolicyIndex,
		StatusIndex:          feature.StatusIndex,
	}
}
