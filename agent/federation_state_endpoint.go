package agent

import (
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// GET /v1/internal/federation-state/<datacenter>
func (s *HTTPHandlers) FederationStateGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	datacenterName := strings.TrimPrefix(req.URL.Path, "/v1/internal/federation-state/")
	if datacenterName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing datacenter name"}
	}

	args := structs.FederationStateQuery{
		Datacenter: datacenterName,
	}
	if done := s.parse(resp, req, &args.TargetDatacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.FederationStateResponse
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("FederationState.Get", &args, &out); err != nil {
		return nil, err
	}

	if out.State == nil {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	return out, nil
}

// GET /v1/internal/federation-states
func (s *HTTPHandlers) FederationStateList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.IndexedFederationStates
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("FederationState.List", &args, &out); err != nil {
		return nil, err
	}

	// make sure we return an array and not nil
	if out.States == nil {
		out.States = make(structs.FederationStates, 0)
	}

	return out.States, nil
}

// GET /v1/internal/federation-states/mesh-gateways
func (s *HTTPHandlers) FederationStateListMeshGateways(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if args.Datacenter == "" {
		args.Datacenter = s.agent.config.Datacenter
	}

	var out structs.DatacenterIndexedCheckServiceNodes
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("FederationState.ListMeshGateways", &args, &out); err != nil {
		return nil, err
	}

	// make sure we return a arrays and not nils
	if out.DatacenterNodes == nil {
		out.DatacenterNodes = make(map[string]structs.CheckServiceNodes)
	}
	for dc, nodes := range out.DatacenterNodes {
		if nodes == nil {
			out.DatacenterNodes[dc] = make(structs.CheckServiceNodes, 0)
		}
	}

	return out.DatacenterNodes, nil
}
