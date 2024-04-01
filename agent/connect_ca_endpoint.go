// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
)

// GET /v1/connect/ca/roots
func (s *HTTPHandlers) ConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	pemResponse := false
	if pemParam := req.URL.Query().Get("pem"); pemParam != "" {
		val, err := strconv.ParseBool(pemParam)
		if err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The 'pem' query parameter must be a boolean value"}
		}
		pemResponse = val
	}

	var reply structs.IndexedCARoots
	defer setMeta(resp, &reply.QueryMeta)
	if err := s.agent.RPC(req.Context(), "ConnectCA.Roots", &args, &reply); err != nil {
		return nil, err
	}

	if !pemResponse {
		return reply, nil
	}

	// defined in RFC 8555 and registered with the IANA
	resp.Header().Set("Content-Type", "application/pem-certificate-chain")

	for _, root := range reply.Roots {
		if _, err := resp.Write([]byte(root.RootCert)); err != nil {
			return nil, err
		}
		for _, intermediate := range root.IntermediateCerts {
			if _, err := resp.Write([]byte(intermediate)); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

// /v1/connect/ca/configuration
func (s *HTTPHandlers) ConnectCAConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.ConnectCAConfigurationGet(resp, req)

	case "PUT":
		return s.ConnectCAConfigurationSet(req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// GET /v1/connect/ca/configuration
func (s *HTTPHandlers) ConnectCAConfigurationGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.CAConfiguration
	err := s.agent.RPC(req.Context(), "ConnectCA.ConfigurationGet", &args, &reply)
	if err != nil {
		return nil, err
	}

	return reply, nil
}

// PUT /v1/connect/ca/configuration
func (s *HTTPHandlers) ConnectCAConfigurationSet(req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration

	var args structs.CARequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req.Body, &args.Config); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	var reply interface{}
	err := s.agent.RPC(req.Context(), "ConnectCA.ConfigurationSet", &args, &reply)
	if err != nil && err.Error() == consul.ErrStateReadOnly.Error() {
		return nil, HTTPError{
			StatusCode: http.StatusBadRequest,
			Reason: "Provider State is read-only. It must be omitted" +
				" or identical to the current value",
		}
	}
	return nil, err
}
