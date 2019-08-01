package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPServer) DiscoveryChainRead(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET", "POST":
	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}

	var args structs.DiscoveryChainRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.Name = strings.TrimPrefix(req.URL.Path, "/v1/discovery-chain/")
	if args.Name == "" {
		return nil, BadRequestError{Reason: "Missing chain name"}
	}

	args.EvaluateInDatacenter = req.URL.Query().Get("compile-dc")
	// TODO(namespaces): args.EvaluateInNamespace = req.URL.Query().Get("compile-namespace")

	if req.Method == "POST" {
		var apiReq discoveryChainReadRequest
		if err := decodeBody(req, &apiReq, nil); err != nil {
			return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
		}

		args.OverrideProtocol = apiReq.OverrideProtocol
		args.OverrideConnectTimeout = apiReq.OverrideConnectTimeout

		if apiReq.OverrideMeshGateway.Mode != "" {
			_, err := structs.ValidateMeshGatewayMode(string(apiReq.OverrideMeshGateway.Mode))
			if err != nil {
				resp.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(resp, "Invalid OverrideMeshGateway.Mode parameter")
				return nil, nil
			}
			args.OverrideMeshGateway = apiReq.OverrideMeshGateway
		}
	}

	// Make the RPC request
	var out structs.DiscoveryChainResponse
	defer setMeta(resp, &out.QueryMeta)

	if err := s.agent.RPC("DiscoveryChain.Get", &args, &out); err != nil {
		return nil, err
	}

	return discoveryChainReadResponse{Chain: out.Chain}, nil
}

// discoveryChainReadRequest is the API variation of structs.DiscoveryChainRequest
type discoveryChainReadRequest struct {
	OverrideMeshGateway    structs.MeshGatewayConfig
	OverrideProtocol       string
	OverrideConnectTimeout time.Duration
}

// discoveryChainReadResponse is the API variation of structs.DiscoveryChainResponse
type discoveryChainReadResponse struct {
	Chain *structs.CompiledDiscoveryChain
}
