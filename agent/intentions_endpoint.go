package agent

import (
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

// /v1/connect/intentions
func (s *HTTPServer) IntentionList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
	}

	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedIntentions
	if err := s.agent.RPC("Intention.List", &args, &reply); err != nil {
		return nil, err
	}

	// Use empty list instead of nil.
	if reply.Intentions == nil {
		reply.Intentions = make(structs.Intentions, 0)
	}
	return reply.Intentions, nil
}
