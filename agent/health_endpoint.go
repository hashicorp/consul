package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/consul/structs"
	"github.com/hashicorp/consul/api"
)

func (s *HTTPServer) HealthChecksInState(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ChecksInStateRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.State = strings.TrimPrefix(req.URL.Path, "/v1/health/state/")
	if args.State == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing check state")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Health.ChecksInState", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for _, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			c.ServiceTags = make([]string, 0)
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthNodeChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/health/node/")
	if args.Node == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing node name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Health.NodeChecks", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for _, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			c.ServiceTags = make([]string, 0)
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
	args.NodeMetaFilters = s.parseMetaFilter(req)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/checks/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedHealthChecks
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Health.ServiceChecks", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.HealthChecks == nil {
		out.HealthChecks = make(structs.HealthChecks, 0)
	}
	for _, c := range out.HealthChecks {
		if c.ServiceTags == nil {
			c.ServiceTags = make([]string, 0)
		}
	}
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/service/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		fmt.Fprint(resp, "Missing service name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedCheckServiceNodes
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
		return nil, err
	}

	// Filter to only passing if specified
	if _, ok := params[api.HealthPassing]; ok {
		val := params.Get(api.HealthPassing)
		// Backwards-compat to allow users to specify ?passing without a value. This
		// should be removed in Consul 0.10.
		var filter bool
		if val == "" {
			filter = true
		} else {
			var err error
			filter, err = strconv.ParseBool(val)
			if err != nil {
				resp.WriteHeader(400)
				fmt.Fprint(resp, "Invalid value for ?passing")
				return nil, nil
			}
		}

		if filter {
			out.Nodes = filterNonPassing(out.Nodes)
		}
	}

	// Translate addresses after filtering so we don't waste effort.
	translateAddresses(s.agent.config, args.Datacenter, out.Nodes)

	// Use empty list instead of nil
	if out.Nodes == nil {
		out.Nodes = make(structs.CheckServiceNodes, 0)
	}
	for i := range out.Nodes {
		// TODO (slackpad) It's lame that this isn't a slice of pointers
		// but it's not a well-scoped change to fix this. We should
		// change this at the next opportunity.
		if out.Nodes[i].Checks == nil {
			out.Nodes[i].Checks = make(structs.HealthChecks, 0)
		}
		for _, c := range out.Nodes[i].Checks {
			if c.ServiceTags == nil {
				c.ServiceTags = make([]string, 0)
			}
		}
		if out.Nodes[i].Service != nil && out.Nodes[i].Service.Tags == nil {
			out.Nodes[i].Service.Tags = make([]string, 0)
		}
	}
	return out.Nodes, nil
}

// filterNonPassing is used to filter out any nodes that have check that are not passing
func filterNonPassing(nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	n := len(nodes)
OUTER:
	for i := 0; i < n; i++ {
		node := nodes[i]
		for _, check := range node.Checks {
			if check.Status != api.HealthPassing {
				nodes[i], nodes[n-1] = nodes[n-1], structs.CheckServiceNode{}
				n--
				i--
				continue OUTER
			}
		}
	}
	return nodes[:n]
}
