package agent

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	metrics "github.com/armon/go-metrics"
	"github.com/hashicorp/consul/agent/structs"
)

var durations = NewDurationFixer("interval", "timeout", "deregistercriticalserviceafter")

// findSourceIP extract the source IP of request
func findSourceIP(req *http.Request) string {
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return "unknown"
	}
	return host
}

func (s *HTTPServer) CatalogRegister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_register"}, 1, labels)
	var args structs.RegisterRequest
	if err := decodeBody(req, &args, durations.FixupDurations); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
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
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_register"}, 1, labels)
		return nil, err
	}
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_register"}, 1, labels)
	return true, nil
}

func (s *HTTPServer) CatalogDeregister(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_deregister"}, 1, labels)

	var args structs.DeregisterRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
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
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_deregister"}, 1, labels)
		return nil, err
	}
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_deregister"}, 1, labels)
	return true, nil
}

func (s *HTTPServer) CatalogDatacenters(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_datacenters"}, 1, labels)

	var out []string
	if err := s.agent.RPC("Catalog.ListDatacenters", struct{}{}, &out); err != nil {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_datacenters"}, 1, labels)
		return nil, err
	}
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_datacenters"}, 1, labels)
	return out, nil
}

func (s *HTTPServer) CatalogNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_nodes"}, 1, labels)

	// Setup the request
	args := structs.DCSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_nodes"}, 1, labels)
		return nil, nil
	}

	var out structs.IndexedNodes
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	labels = append(labels, metrics.Label{Name: "consistency", Value: args.QueryOptions.ConsistencyLevel()})
	if err := s.agent.RPC("Catalog.ListNodes", &args, &out); err != nil {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
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
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_nodes"}, 1, labels)
	return out.Nodes, nil
}

func (s *HTTPServer) CatalogServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_services"}, 1, labels)

	// Set default DC
	args := structs.DCSpecificRequest{}
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedServices
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	labels = append(labels, metrics.Label{Name: "consistency", Value: args.QueryOptions.ConsistencyLevel()})
	if err := s.agent.RPC("Catalog.ListServices", &args, &out); err != nil {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_services"}, 1, labels)
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty map instead of nil
	if out.Services == nil {
		out.Services = make(structs.Services, 0)
	}
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_services"}, 1, labels)
	return out.Services, nil
}

func (s *HTTPServer) CatalogServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_service_nodes"}, 1, labels)

	// Set default DC
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
		return nil, nil
	}

	// Check for a tag
	params := req.URL.Query()
	if _, ok := params["tag"]; ok {
		args.ServiceTag = params.Get("tag")
		args.TagFilter = true
		labels = append(labels, metrics.Label{Name: "tag", Value: args.ServiceTag})
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/catalog/service/")
	if args.ServiceName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service name")
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedServiceNodes
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	labels = append(labels, metrics.Label{Name: "consistency", Value: args.QueryOptions.ConsistencyLevel()})
	if err := s.agent.RPC("Catalog.ServiceNodes", &args, &out); err != nil {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_service_nodes"}, 1, labels)
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
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
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_service_nodes"}, 1, labels)
	return out.ServiceNodes, nil
}

func (s *HTTPServer) CatalogNodeServices(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	labels := []metrics.Label{{Name: "agent", Value: s.nodeName()}, {Name: "client", Value: findSourceIP(req)}}
	defer metrics.IncrCounterWithLabels([]string{"client", "api", "catalog_node_services"}, 1, labels)

	// Set default Datacenter
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/catalog/node/")
	if args.Node == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing node name")
		labels = append(labels, metrics.Label{Name: "code", Value: "400"})
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedNodeServices
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	labels = append(labels, metrics.Label{Name: "consistency", Value: args.QueryOptions.ConsistencyLevel()})
	if err := s.agent.RPC("Catalog.NodeServices", &args, &out); err != nil {
		labels = append(labels, metrics.Label{Name: "code", Value: "500"})
		metrics.IncrCounterWithLabels([]string{"client", "rpc", "error", "catalog_node_services"}, 1, labels)
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	if out.NodeServices != nil && out.NodeServices.Node != nil {
		s.agent.TranslateAddresses(args.Datacenter, out.NodeServices.Node)
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
	labels = append(labels, metrics.Label{Name: "code", Value: "200"})
	metrics.IncrCounterWithLabels([]string{"client", "api", "success", "catalog_node_services"}, 1, labels)
	return out.NodeServices, nil
}
