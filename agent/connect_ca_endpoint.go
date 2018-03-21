package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

// GET /v1/connect/ca/roots
func (s *HTTPServer) ConnectCARoots(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Test the method
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

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
	case "PUT":
		return s.ConnectCAConfigurationSet(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// PUT /v1/connect/ca/configuration
func (s *HTTPServer) ConnectCAConfigurationSet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in ConnectCAConfiguration

	var args structs.CAConfiguration
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	var reply interface{}
	err := s.agent.RPC("ConnectCA.ConfigurationSet", &args, &reply)
	return nil, err
}
