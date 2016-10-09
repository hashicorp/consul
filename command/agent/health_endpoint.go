package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/consul/consul/structs"
)

func (s *HTTPServer) HealthChecksInState(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ChecksInStateRequest{}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.State = strings.TrimPrefix(req.URL.Path, "/v1/health/state/")
	if args.State == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing check state"))
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
		resp.Write([]byte("Missing node name"))
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
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceChecks(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the service name
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/checks/")
	if args.ServiceName == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
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
	return out.HealthChecks, nil
}

func (s *HTTPServer) HealthServiceNodes(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
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
		resp.Write([]byte("Missing service name"))
		return nil, nil
	}

	// Make the RPC request
	var out structs.IndexedCheckServiceNodes
	defer func() {
		setMeta(resp, &out.QueryMeta)
	}()

	if err := s.agent.RPC("Health.ServiceNodes", &args, &out); err != nil {
		return nil, err
	}

	// Filter to only passing if specified
	if _, ok := params[structs.HealthPassing]; ok {
		out.Nodes = filterNonPassing(out.Nodes)
	}

	// Translate addresses after filtering so we don't waste effort.
	translateAddresses(s.agent.config, args.Datacenter, out.Nodes)

	// Use empty list instead of nil
	for i, _ := range out.Nodes {
		// TODO (slackpad) It's lame that this isn't a slice of pointers
		// but it's not a well-scoped change to fix this. We should
		// change this at the next opportunity.
		if out.Nodes[i].Checks == nil {
			out.Nodes[i].Checks = make(structs.HealthChecks, 0)
		}
	}
	if out.Nodes == nil {
		out.Nodes = make(structs.CheckServiceNodes, 0)
	}

	return out.Nodes, nil
}

func (s *HTTPServer) HealthServiceNodesBatch(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC  
	args := structs.ServiceSpecificRequest{}
	s.parseSource(req, &args.Source)
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
	args.ServiceName = strings.TrimPrefix(req.URL.Path, "/v1/health/services/")

	if args.ServiceName == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing service name"))
		return nil, nil
	}

	// Make the RPC request

	services := strings.Split(args.ServiceName, ",")

	out := structs.IndexedCheckServiceNodesMap{Nodes: make(map[string]structs.IndexedCheckServiceNodes)}

	for _, currService := range services {
		out.Nodes[currService] = structs.IndexedCheckServiceNodes{}
	}

	defer func() {
		for _, value := range out.Nodes {
			setMeta(resp, &value.QueryMeta)
		}
	}()

	err := s.agent.RPC("Health.ServiceNodesBatch", &args, &out)

	if err != nil {
		return nil, err
	}

	for key, value := range out.Nodes {
		// Filter to only passing if specified

		if _, ok := params[structs.HealthPassing]; ok {
			value.Nodes = filterNonPassing(value.Nodes)
		}

		if len(value.Nodes) == 0 {
			delete(out.Nodes, key)
		}

		// Use empty list instead of nil
		for i, _ := range value.Nodes {
			// TODO (slackpad) It's lame that this isn't a slice of pointers
			// but it's not a well-scoped change to fix this. We should
			// change this at the next opportunity.
			if value.Nodes[i].Checks == nil {
				value.Nodes[i].Checks = make(structs.HealthChecks, 0)
			}
		}
		if value.Nodes == nil {
			value.Nodes = make(structs.CheckServiceNodes, 0)
		}
	}

	return out, nil
}

// filterNonPassing is used to filter out any nodes that have check that are not passing
func filterNonPassing(nodes structs.CheckServiceNodes) structs.CheckServiceNodes {
	n := len(nodes)
OUTER:
	for i := 0; i < n; i++ {
		node := nodes[i]
		for _, check := range node.Checks {
			if check.Status != structs.HealthPassing {
				nodes[i], nodes[n-1] = nodes[n-1], structs.CheckServiceNode{}
				n--
				i--
				continue OUTER
			}
		}
	}
	return nodes[:n]
}
