package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func (s *HTTPHandlers) ReachabilityProbe(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ReachabilityRequest

	s.parseDC(req, &args.Datacenter)
	s.parseTokenWithDefault(req, &args.Token)

	// Check if the WAN is being queried
	wan := false
	if other := req.URL.Query().Get("wan"); other != "" {
		args.WAN = true
	}

	args.Segment = req.URL.Query().Get("segment")
	if wan {
		switch args.Segment {
		case "", api.AllSegments:
			// The zero value and the special "give me all members"
			// key are ok, otherwise the argument doesn't apply to
			// the WAN.
		default:
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(resp, "Cannot provide a segment with wan=true")
			return nil, nil
		}
	}

	var out structs.ReachabilityResponses
	defer setMeta(resp, &out.QueryMeta)

	if err := s.agent.RPC("Internal.ReachabilityProbe", &args, &out); err != nil {
		return nil, err
	}

	return reachabilityResponses{Responses: out.Responses}, nil
}

// reachabilityResponses is the API variation of var out structs.ReachabilityResponses
type reachabilityResponses struct {
	Responses []*structs.ReachabilityResponse
}
