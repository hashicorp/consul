package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

// GET /v1/connect/ca/roots
func (s *HTTPServer) ConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedCARoots
	defer setMeta(resp, &reply.QueryMeta)
	if err := s.agent.RPC("ConnectCA.Roots", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.ConnectCAConfigurationGet(resp, req)

	case "PUT":
		return s.ConnectCAConfigurationSet(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// GEt /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfigurationGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.CAConfiguration
	err := s.agent.RPC("ConnectCA.ConfigurationGet", &args, &reply)
	return reply, err
}

// PUT /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfigurationSet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration

	var args structs.CARequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req, &args.Config, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	var reply interface{}
	err := s.agent.RPC("ConnectCA.ConfigurationSet", &args, &reply)
	return nil, err
}
