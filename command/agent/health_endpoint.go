package agent

import (
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

func (s *HTTPServer) HealthChecksInState(resp http.ResponseWriter, req *http.Request) (uint64, interface{}, error) {
	// Set default DC
	args := structs.ChecksInStateRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.BlockingQuery); done {
		return 0, nil, nil
	}

	// Pull out the service name
	args.State = strings.TrimPrefix(req.URL.Path, "/v1/health/state/")
	if args.State == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing check state"))
		return 0, nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	if err := s.agent.RPC("Health.ChecksInState", &args, &out); err != nil {
		return 0, nil, err
	}
	return out.Index, out.HealthChecks, nil
}

func (s *HTTPServer) HealthNodeChecks(resp http.ResponseWriter, req *http.Request) (uint64, interface{}, error) {
	// Set default DC
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.BlockingQuery); done {
		return 0, nil, nil
	}

	// Pull out the service name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/health/node/")
	if args.Node == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing node name"))
		return 0, nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	if err := s.agent.RPC("Health.NodeChecks", &args, &out); err != nil {
		return 0, nil, err
	}
	return out.Index, out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceChecks(resp http.ResponseWriter, req *http.Request) (uint64, interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.BlockingQuery); done {
		return 0, nil, nil
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/checks/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
		return 0, nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	if err := s.agent.RPC("Health.ServiceChecks", &args, &out); err != nil {
		return 0, nil, err
	}
	return out.Index, out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceNodes(resp http.ResponseWriter, req *http.Request) (uint64, interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.BlockingQuery); done {
		return 0, nil, nil
	}

	// Check for a tag
	params := req.URL.Query()
	if _, ok := params["tag"]; ok {
		args.ServiceTag = params.Get("tag")
		args.TagFilter = true
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/service/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
		return 0, nil, nil
	}

	// Make the RPC request
	var out structs.IndexedCheckServiceNodes
	if err := s.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
		return 0, nil, err
	}
	return out.Index, out.Nodes, nil
}
