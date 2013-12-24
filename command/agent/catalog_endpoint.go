package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
)

/*
* /v1/catalog/register : Registers a new service
* /v1/catalog/deregister : Deregisters a service or node
* /v1/catalog/datacenters : Lists known datacenters
* /v1/catalog/nodes : Lists nodes in a given DC
* /v1/catalog/services : Lists services in a given DC
* /v1/catalog/service/<service>/ : Lists the nodes in a given service
* /v1/catalog/node/<node>/ : Lists the services provided by a node
 */

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
	s.logger.Printf("[DEBUG] ARGS: %#v %v %#v", args, args.Datacenter == "", s.agent.config)

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC("Catalog.Register", &args, &out); err != nil {
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
