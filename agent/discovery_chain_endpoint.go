package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPServer) ConnectDiscoveryChainGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DiscoveryChainRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.Name = strings.TrimPrefix(req.URL.Path, "/v1/connect/discovery-chain/")
	if args.Name == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing chain name")
		return nil, nil
	}

	args.EvaluateInDatacenter = req.URL.Query().Get("eval_dc")
	// TODO(namespaces): args.EvaluateInNamespace = req.URL.Query().Get("eval_namespace")

	overrideMeshGatewayMode := req.URL.Query().Get("override_mesh_gateway_mode")
	if overrideMeshGatewayMode != "" {
		mode, err := structs.ValidateMeshGatewayMode(overrideMeshGatewayMode)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Invalid override_mesh_gateway_mode parameter")
			return nil, nil
		}
		args.OverrideMeshGateway.Mode = mode
	}

	args.OverrideProtocol = req.URL.Query().Get("override_protocol")
	overrideTimeoutString := req.URL.Query().Get("override_connect_timeout")
	if overrideTimeoutString != "" {
		d, err := time.ParseDuration(overrideTimeoutString)
		if err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Invalid override_connect_timeout parameter")
			return nil, nil
		}
		args.OverrideConnectTimeout = d
	}

	// Make the RPC request
	var out structs.DiscoveryChainResponse
	defer setMeta(resp, &out.QueryMeta)

	if err := s.agent.RPC("DiscoveryChain.Get", &args, &out); err != nil {
		return nil, err
	}

	apiOut := apiDiscoveryChainResponse{
		Chain:   out.Chain,
		Entries: out.Entries,
	}

	return apiOut, nil
}

type apiDiscoveryChainResponse struct {
	Chain   *structs.CompiledDiscoveryChain
	Entries []structs.ConfigEntry
}
