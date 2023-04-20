package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/armon/go-metrics"
	"github.com/armon/go-metrics/prometheus"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
)

var CatalogCounters = []prometheus.CounterDefinition{
	{
		Name: []string{"client", "api", "catalog_register"},
		Help: "Increments whenever a Consul agent receives a catalog register request.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_register"},
		Help: "Increments whenever a Consul agent receives an RPC error for a catalog register request.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_register"},
		Help: "Increments whenever a Consul agent successfully responds to a catalog register request.",
	},
	{
		Name: []string{"client", "api", "catalog_deregister"},
		Help: "Increments whenever a Consul agent receives a catalog deregister request.",
	},
	{
		Name: []string{"client", "api", "catalog_datacenters"},
		Help: "Increments whenever a Consul agent receives a request to list datacenters in the catalog.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_deregister"},
		Help: "Increments whenever a Consul agent receives an RPC error for a catalog deregister request.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_nodes"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list nodes.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_nodes"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list nodes.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_deregister"},
		Help: "Increments whenever a Consul agent successfully responds to a catalog deregister request.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_datacenters"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list datacenters.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_datacenters"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list datacenters.",
	},
	{
		Name: []string{"client", "api", "catalog_nodes"},
		Help: "Increments whenever a Consul agent receives a request to list nodes from the catalog.",
	},
	{
		Name: []string{"client", "api", "catalog_services"},
		Help: "Increments whenever a Consul agent receives a request to list services from the catalog.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_services"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list services.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_services"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list services.",
	},
	{
		Name: []string{"client", "api", "catalog_service_nodes"},
		Help: "Increments whenever a Consul agent receives a request to list nodes offering a service.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_service_nodes"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list nodes offering a service.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_service_nodes"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list nodes offering a service.",
	},
	{
		Name: []string{"client", "api", "error", "catalog_service_nodes"},
		Help: "Increments whenever a Consul agent receives an RPC error for request to list nodes offering a service.",
	},
	{
		Name: []string{"client", "api", "catalog_node_services"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list nodes offering a service.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_node_services"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list services in a node.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_node_services"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list services in a node.",
	},
	{
		Name: []string{"client", "api", "catalog_node_service_list"},
		Help: "Increments whenever a Consul agent receives a request to list a node's registered services.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_node_service_list"},
		Help: "Increments whenever a Consul agent receives an RPC error for request to list a node's registered services.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_node_service_list"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list a node's registered services.",
	},
	{
		Name: []string{"client", "api", "catalog_gateway_services"},
		Help: "Increments whenever a Consul agent receives a request to list services associated with a gateway.",
	},
	{
		Name: []string{"client", "rpc", "error", "catalog_gateway_services"},
		Help: "Increments whenever a Consul agent receives an RPC error for a request to list services associated with a gateway.",
	},
	{
		Name: []string{"client", "api", "success", "catalog_gateway_services"},
		Help: "Increments whenever a Consul agent successfully responds to a request to list services associated with a gateway.",
	},
}

func (s *HTTPHandlers) CatalogRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_register"}, 1,
		s.nodeMetricsLabels())

	var args structs.RegisterRequest
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if err := s.rewordUnknownEnterpriseFieldError(decodeBody(req.Body, &args)); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}
	s.parseToken(req, &args.Token)

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC(req.Context(), "Catalog.Register", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_register"}, 1,
			s.nodeMetricsLabels())
		return nil, err
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_register"}, 1,
		s.nodeMetricsLabels())
	return true, nil
}

func (s *HTTPHandlers) CatalogDeregister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_deregister"}, 1,
		s.nodeMetricsLabels())

	var args structs.DeregisterRequest
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if err := s.rewordUnknownEnterpriseFieldError(decodeBody(req.Body, &args)); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	// Setup the default DC if not provided
	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}
	s.parseToken(req, &args.Token)

	// Forward to the servers
	var out struct{}
	if err := s.agent.RPC(req.Context(), "Catalog.Deregister", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_deregister"}, 1,
			s.nodeMetricsLabels())
		return nil, err
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_deregister"}, 1,
		s.nodeMetricsLabels())
	return true, nil
}

func (s *HTTPHandlers) CatalogDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_datacenters"}, 1,
		s.nodeMetricsLabels())

	args := structs.DatacentersRequest{}
	s.parseConsistency(resp, req, &args.QueryOptions)
	parseCacheControl(resp, req, &args.QueryOptions)
	var out []string

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(req.Context(), cachetype.CatalogDatacentersName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_datacenters"}, 1,
				s.nodeMetricsLabels())
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
		if err := s.agent.RPC(req.Context(), "Catalog.ListDatacenters", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_datacenters"}, 1,
				s.nodeMetricsLabels())
			return nil, err
		}
	}

	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_datacenters"}, 1,
		s.nodeMetricsLabels())
	return out, nil
}

func (s *HTTPHandlers) CatalogNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_nodes"}, 1,
		s.nodeMetricsLabels())

	// Setup the request
	args := structs.DCSpecificRequest{}
	s.parseSource(req, &args.Source)
	if err := s.parseEntMetaPartition(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_nodes"}, 1,
			s.nodeMetricsLabels())
		return nil, nil
	}

	var out structs.IndexedNodes
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC(req.Context(), "Catalog.ListNodes", &args, &out); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	s.agent.TranslateAddresses(args.Datacenter, out.Nodes, TranslateAddressAcceptAny)

	// Use empty list instead of nil
	if out.Nodes == nil {
		out.Nodes = make(structs.Nodes, 0)
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_nodes"}, 1,
		s.nodeMetricsLabels())
	return out.Nodes, nil
}

func (s *HTTPHandlers) CatalogServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_services"}, 1,
		s.nodeMetricsLabels())

	args := structs.DCSpecificRequest{}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	var out structs.IndexedServices
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(req.Context(), cachetype.CatalogListServicesName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_services"}, 1,
				s.nodeMetricsLabels())
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
		if err := s.agent.RPC(req.Context(), "Catalog.ListServices", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_services"}, 1,
				s.nodeMetricsLabels())
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
		s.nodeMetricsLabels())
	return out.Services, nil
}

func (s *HTTPHandlers) CatalogConnectServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.catalogServiceNodes(resp, req, true)
}

func (s *HTTPHandlers) CatalogServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.catalogServiceNodes(resp, req, false)
}

func (s *HTTPHandlers) catalogServiceNodes(resp http.ResponseWriter, req *http.Request, connect bool) (interface{}, error) {
	metricsKey := "catalog_service_nodes"
	pathPrefix := "/v1/catalog/service/"
	if connect {
		metricsKey = "catalog_connect_service_nodes"
		pathPrefix = "/v1/catalog/connect/"
	}

	metrics.IncrCounterWithLabels([]string{"client", "api", metricsKey}, 1,
		s.nodeMetricsLabels())

	args := structs.ServiceSpecificRequest{Connect: connect}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

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

	if _, ok := params["merge-central-config"]; ok {
		args.MergeCentralConfig = true
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, pathPrefix)
	if args.ServiceName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing service name"}
	}

	// Make the RPC request
	var out structs.IndexedServiceNodes
	defer setMeta(resp, &out.QueryMeta)

	if args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(req.Context(), cachetype.CatalogServicesName, &args)
		if err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_service_nodes"}, 1,
				s.nodeMetricsLabels())
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
		if err := s.agent.RPC(req.Context(), "Catalog.ServiceNodes", &args, &out); err != nil {
			metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_service_nodes"}, 1,
				s.nodeMetricsLabels())
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}

	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	s.agent.TranslateAddresses(args.Datacenter, out.ServiceNodes, TranslateAddressAcceptAny)

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
		s.nodeMetricsLabels())
	return out.ServiceNodes, nil
}

func (s *HTTPHandlers) CatalogNodeServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_node_services"}, 1,
		s.nodeMetricsLabels())

	// Set default Datacenter
	args := structs.NodeSpecificRequest{}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node/")
	if args.Node == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing node name"}
	}

	// Make the RPC request
	var out structs.IndexedNodeServices
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC(req.Context(), "Catalog.NodeServices", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_node_services"}, 1,
			s.nodeMetricsLabels())
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	if out.NodeServices != nil {
		s.agent.TranslateAddresses(args.Datacenter, out.NodeServices, TranslateAddressAcceptAny)
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
		s.nodeMetricsLabels())
	return out.NodeServices, nil
}

func (s *HTTPHandlers) CatalogNodeServiceList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_node_service_list"}, 1,
		s.nodeMetricsLabels())

	// Set default Datacenter
	args := structs.NodeSpecificRequest{}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node-services/")
	if args.Node == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing node name"}
	}

	if _, ok := req.URL.Query()["merge-central-config"]; ok {
		args.MergeCentralConfig = true
	}

	// Make the RPC request
	var out structs.IndexedNodeServiceList
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC(req.Context(), "Catalog.NodeServiceList", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_node_service_list"}, 1,
			s.nodeMetricsLabels())
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	s.agent.TranslateAddresses(args.Datacenter, &out.NodeServices, TranslateAddressAcceptAny)

	// Use empty list instead of nil
	for _, s := range out.NodeServices.Services {
		if s.Tags == nil {
			s.Tags = make([]string, 0)
		}
	}
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_node_service_list"}, 1,
		s.nodeMetricsLabels())
	return &out.NodeServices, nil
}

func (s *HTTPHandlers) CatalogGatewayServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_gateway_services"}, 1,
		s.nodeMetricsLabels())

	var args structs.ServiceSpecificRequest

	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the gateway's service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/catalog/gateway-services/")
	if args.ServiceName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing gateway name"}
	}

	// Make the RPC request
	var out structs.IndexedGatewayServices
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC(req.Context(), "Catalog.GatewayServices", &args, &out); err != nil {
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_gateway_services"}, 1,
			s.nodeMetricsLabels())
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_gateway_services"}, 1,
		s.nodeMetricsLabels())
	return out.Services, nil
}
