package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPServer) CatalogRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.RegisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}
	s.parseToken(req, &args.Token)

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Register", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *HTTPServer) CatalogDeregister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DeregisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}
	s.parseToken(req, &args.Token)

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
	// Setup the request
	args := structs.DCSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedNodes
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Catalog.ListNodes", &args, &out); err != nil {
		return nil, err
	}
	s.agent.TranslateAddresses(args.Datacenter, out.Nodes)

	// Use empty list instead of nil
	if out.Nodes == nil {
		out.Nodes = make(structs.Nodes, 0)
	}
	return out.Nodes, nil
}

func (s *HTTPServer) CatalogServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.DCSpecificRequest{}
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedServices
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Catalog.ListServices", &args, &out); err != nil {
		return nil, err
	}

	// Use empty map instead of nil
	if out.Services == nil {
		out.Services = make(structs.Services, 0)
	}
	return out.Services, nil
}

func (s *HTTPServer) CatalogServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Check for a tag
	params := req.URL.Query()
	if _, ok := params["tag"]; ok {
		args.ServiceTag = params.Get("tag")
		args.TagFilter = true
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/catalog/service/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedServiceNodes
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Catalog.ServiceNodes", &args, &out); err != nil {
		return nil, err
	}
	s.agent.TranslateAddresses(args.Datacenter, out.ServiceNodes)

	// Use empty list instead of nil
	if out.ServiceNodes == nil {
		out.ServiceNodes = make(structs.ServiceNodes, 0)
	}
	for _, s := range out.ServiceNodes {
		if s.ServiceTags == nil {
			s.ServiceTags = make([]string, 0)
		}
	}
	return out.ServiceNodes, nil
}

func (s *HTTPServer) CatalogNodeServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default Datacenter
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node/")
	if args.Node == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing node name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedNodeServices
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		return nil, err
	}
	if out.NodeServices != nil && out.NodeServices.Node != nil {
		s.agent.TranslateAddresses(args.Datacenter, out.NodeServices.Node)
	}

	// Use empty list instead of nil
	if out.NodeServices != nil {
		for _, s := range out.NodeServices.Services {
			if s.Tags == nil {
				s.Tags = make([]string, 0)
			}
		}
	}
	return out.NodeServices, nil
}
