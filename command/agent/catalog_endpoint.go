package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

func (s *HTTPServer) CatalogRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.RegisterRequest
	if err := decodeBody(req, &args); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Register", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *HTTPServer) CatalogDeregister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DeregisterRequest
	if err := decodeBody(req, &args); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Deregister", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *HTTPServer) CatalogDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var out []string
	if err := s.agent.RPC("Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPServer) CatalogNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	dc := s.agent.config.Datacenter

	// Check for other DC
	if other := req.URL.Query().Get("dc"); other != "" {
		dc = other
	}

	var out structs.Nodes
	if err := s.agent.RPC("Catalog.ListNodes", dc, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPServer) CatalogServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	dc := s.agent.config.Datacenter

	// Check for other DC
	if other := req.URL.Query().Get("dc"); other != "" {
		dc = other
	}

	var out structs.Services
	if err := s.agent.RPC("Catalog.ListServices", dc, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPServer) CatalogServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceNodesRequest{
		Datacenter: s.agent.config.Datacenter,
	}

	// Check for other DC
	params := req.URL.Query()
	if other := params.Get("dc"); other != "" {
		args.Datacenter = other
	}

	// Check for a tag
	if _, ok := params["tag"]; ok {
		args.ServiceTag = params.Get("tag")
		args.TagFilter = true
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/catalog/service/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
		return nil, nil
	}

	// Make the RPC request
	var out structs.ServiceNodes
	if err := s.agent.RPC("Catalog.ServiceNodes", &args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPServer) CatalogNodeServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default Datacenter
	args := structs.NodeServicesRequest{
		Datacenter: s.agent.config.Datacenter,
	}

	// Check for other DC
	params := req.URL.Query()
	if other := params.Get("dc"); other != "" {
		args.Datacenter = other
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node/")
	if args.Node == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing node name"))
		return nil, nil
	}

	// Make the RPC request
	out := new(structs.NodeServices)
	if err := s.agent.RPC("Catalog.NodeServices", &args, out); err != nil {
		return nil, err
	}
	return out, nil
}
