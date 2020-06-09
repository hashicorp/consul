package agent

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func (s *HTTPServer) HealthChecksInState(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ChecksInStateRequest{}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.State = strings.TrimPrefix(req.URL.Path, "/v1/health/state/")
	if args.State == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing check state")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("Health.ChecksInState", &args, &out); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for i, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			clone := *c
			clone.ServiceTags = make([]string, 0)
			out.HealthChecks[i] = &clone
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthNodeChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.NodeSpecificRequest{}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/health/node/")
	if args.Node == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing node name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("Health.NodeChecks", &args, &out); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for i, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			clone := *c
			clone.ServiceTags = make([]string, 0)
			out.HealthChecks[i] = &clone
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/checks/")
	if args.ServiceName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("Health.ServiceChecks", &args, &out); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for i, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			clone := *c
			clone.ServiceTags = make([]string, 0)
			out.HealthChecks[i] = &clone
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthConnectServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.healthServiceNodes(resp, req, true)
}

func (s *HTTPServer) HealthServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.healthServiceNodes(resp, req, false)
}

func (s *HTTPServer) healthServiceNodes(resp http.ResponseWriter, req *http.Request, connect bool) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Check for tags
	params := req.URL.Query()
	if _, ok := params["tag"]; ok {
		args.ServiceTags = params["tag"]
		args.TagFilter = true
	}

	// Determine the prefix
	prefix := "/v1/health/service/"
	if connect {
		prefix = "/v1/health/connect/"

		// Check for ingress request only when requesting connect services
		ingress, err := getBoolQueryParam(params, "ingress")
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Invalid value for ?ingress")
			return nil, nil
		}

		if ingress {
			args.Ingress = true
		} else {
			args.Connect = true
		}
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, prefix)
	if args.ServiceName == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedCheckServiceNodes
	defer setMeta(resp, &out.QueryMeta)

	if s.agent.config.HTTPUseCache && args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(cachetype.HealthServicesName, &args)
		if err != nil {
			return nil, err
		}
		defer setCacheMeta(resp, &m)
		reply, ok := raw.(*structs.IndexedCheckServiceNodes)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
	RETRY_ONCE:
		if err := s.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Filter to only passing if specified
	filter, err := getBoolQueryParam(params, api.HealthPassing)
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Invalid value for ?passing")
		return nil, nil
	}

	if filter {
		out.Nodes = filterNonPassing(out.Nodes)
	}

	// Translate addresses after filtering so we don't waste effort.
	s.agent.TranslateAddresses(args.Datacenter, out.Nodes, TranslateAddressAcceptAny)

	// Use empty list instead of nil
	if out.Nodes == nil {
		out.Nodes = make(structs.CheckServiceNodes, 0)
	}
	for i := range out.Nodes {
		if out.Nodes[i].Checks == nil {
			out.Nodes[i].Checks = make(structs.HealthChecks, 0)
		}
		for j, c := range out.Nodes[i].Checks {
			if c.ServiceTags == nil {
				clone := *c
				clone.ServiceTags = make([]string, 0)
				out.Nodes[i].Checks[j] = &clone
			}
		}
		if out.Nodes[i].Service != nil && out.Nodes[i].Service.Tags == nil {
			clone := *out.Nodes[i].Service
			clone.Tags = make([]string, 0)
			out.Nodes[i].Service = &clone
		}
	}
	return out.Nodes, nil
}

func getBoolQueryParam(params url.Values, key string) (bool, error) {
	var param bool
	if _, ok := params[key]; ok {
		val := params.Get(key)
		// Orginally a comment declared this check should be removed after Consul
		// 0.10, to no longer support using ?passing without a value. However, I
		// think this is a reasonable experience for a user and so am keeping it
		// here.
		if val == "" {
			param = true
		} else {
			var err error
			param, err = strconv.ParseBool(val)
			if err != nil {
				return false, err
			}
		}
	}
	return param, nil
}

// filterNonPassing is used to filter out any nodes that have check that are not passing
func filterNonPassing(nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	n := len(nodes)

	// Make a copy of the cached nodes rather than operating on the cache directly
	out := append(nodes[:0:0], nodes...)

OUTER:
	for i := 0; i < n; i++ {
		node := out[i]
		for _, check := range node.Checks {
			if check.Status != api.HealthPassing {
				out[i], out[n-1] = out[n-1], structs.CheckServiceNode{}
				n--
				i--
				continue OUTER
			}
		}
	}
	return out[:n]
}
