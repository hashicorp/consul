package agent

import (
	"fmt"
	"net/http"
	"strings"

	metrics "github.com/armon/go-metrics"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

var durations = NewDurationFixer("interval", "timeout", "deregistercriticalserviceafter")

func (s *HTTPServer) CatalogRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_register"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	var args structs.RegisterRequest
	if err := decodeBody(req, &args, durations.FixupDurations); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
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
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_register"}, 1,
			[]metrics.Label{{Name: "node", Value: s.nodeName()}})
		return nil, err
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_register"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return true, nil
}

func (s *HTTPServer) CatalogDeregister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_deregister"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	var args structs.DeregisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
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
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_deregister"}, 1,
			[]metrics.Label{{Name: "node", Value: s.nodeName()}})
		return nil, err
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_deregister"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return true, nil
}

func (s *HTTPServer) CatalogDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_datacenters"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	args := structs.DatacentersRequest{}
	s.parseConsistency(resp, req, &args.QueryOptions)
	parseCacheControl(resp, req, &args.QueryOptions)
	var out []string

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(cachetype.CatalogDatacentersName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_datacenters"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
		reply, ok := raw.(*[]string)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		defer setCacheMeta(resp, &m)
		out = *reply
	} else {
		if err := s.agent.RPC("Catalog.ListDatacenters", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_datacenters"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
	}

	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_datacenters"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return out, nil
}

func (s *HTTPServer) CatalogNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_nodes"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	// Setup the request
	args := structs.DCSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_nodes"}, 1,
			[]metrics.Label{{Name: "node", Value: s.nodeName()}})
		return nil, nil
	}

	var out structs.IndexedNodes
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("Catalog.ListNodes", &args, &out); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	s.agent.TranslateAddresses(args.Datacenter, out.Nodes)

	// Use empty list instead of nil
	if out.Nodes == nil {
		out.Nodes = make(structs.Nodes, 0)
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_nodes"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return out.Nodes, nil
}

func (s *HTTPServer) CatalogServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_services"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	// Set default DC
	args := structs.DCSpecificRequest{}
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	var out structs.IndexedServices
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(cachetype.CatalogListServicesName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_services"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
		reply, ok := raw.(*structs.IndexedServices)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		defer setCacheMeta(resp, &m)
		out = *reply
	} else {
	RETRY_ONCE:
		if err := s.agent.RPC("Catalog.ListServices", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_services"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}

	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty map instead of nil
	if out.Services == nil {
		out.Services = make(structs.Services)
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_services"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return out.Services, nil
}

func (s *HTTPServer) CatalogConnectServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.catalogServiceNodes(resp, req, true)
}

func (s *HTTPServer) CatalogServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.catalogServiceNodes(resp, req, false)
}

func (s *HTTPServer) catalogServiceNodes(resp http.ResponseWriter, req *http.Request, connect bool) (interface{}, error) {
	metricsKey := "catalog_service_nodes"
	pathPrefix := "/v1/catalog/service/"
	if connect {
		metricsKey = "catalog_connect_service_nodes"
		pathPrefix = "/v1/catalog/connect/"
	}

	metrics.IncrCounterWithLabels([]string{"client", "api", metricsKey}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	// Set default DC
	args := structs.ServiceSpecificRequest{Connect: connect}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Check for a tag
	params := req.URL.Query()
	if _, ok := params["tag"]; ok {
		args.ServiceTags = params["tag"]
		args.TagFilter = true
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, pathPrefix)
	if args.ServiceName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedServiceNodes
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(cachetype.CatalogServicesName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_service_nodes"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
		defer setCacheMeta(resp, &m)
		reply, ok := raw.(*structs.IndexedServiceNodes)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
	RETRY_ONCE:
		if err := s.agent.RPC("Catalog.ServiceNodes", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_service_nodes"}, 1,
				[]metrics.Label{{Name: "node", Value: s.nodeName()}})
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}

	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	s.agent.TranslateAddresses(args.Datacenter, out.ServiceNodes)

	// Use empty list instead of nil
	if out.ServiceNodes == nil {
		out.ServiceNodes = make(structs.ServiceNodes, 0)
	}
	for i, s := range out.ServiceNodes {
		if s.ServiceTags == nil {
			clone := *s
			clone.ServiceTags = make([]string, 0)
			out.ServiceNodes[i] = &clone
		}
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_service_nodes"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return out.ServiceNodes, nil
}

func (s *HTTPServer) CatalogNodeServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_node_services"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})

	// Set default Datacenter
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node/")
	if args.Node == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing node name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedNodeServices
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_node_services"}, 1,
			[]metrics.Label{{Name: "node", Value: s.nodeName()}})
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	if out.NodeServices != nil {
		s.agent.TranslateAddresses(args.Datacenter, out.NodeServices)
	}

	// TODO: The NodeServices object in IndexedNodeServices is a pointer to
	// something that's created for each request by the state store way down
	// in https://github.com/hashicorp/consul/blob/v1.0.4/agent/consul/state/catalog.go#L953-L963.
	// Since this isn't a pointer to a real state store object, it's safe to
	// modify out.NodeServices.Services in the loop below without making a
	// copy here. Same for the Tags in each service entry, since that was
	// created by .ToNodeService() which made a copy. This is safe as-is but
	// this whole business is tricky and subtle. See #3867 for more context.

	// Use empty list instead of nil
	if out.NodeServices != nil {
		for _, s := range out.NodeServices.Services {
			if s.Tags == nil {
				s.Tags = make([]string, 0)
			}
		}
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_node_services"}, 1,
		[]metrics.Label{{Name: "node", Value: s.nodeName()}})
	return out.NodeServices, nil
}
