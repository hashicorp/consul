package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// InternalDiscoveryChain is helpful for debugging. Eventually we should expose
// this data officially somehow.
func (s *HTTPServer) InternalDiscoveryChain(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DiscoveryChainRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	args.Name = strings.TrimPrefix(req.URL.Path, "/v1/internal/discovery-chain/")
	if args.Name == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing chain name")
		return nil, nil
	}

	// Make the RPC request
	var out structs.DiscoveryChainResponse
	defer setMeta(resp, &out.QueryMeta)

	if err := s.agent.RPC("ConfigEntry.ReadDiscoveryChain", &args, &out); err != nil {
		return nil, err
	}

	if out.Chain == nil {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	// wipe before replying
	// out.Chain.GroupResolverNodes = nil
	// out.Chain.Resolvers = nil
	// out.Chain.Targets = nil

	return out.Chain, nil
}
