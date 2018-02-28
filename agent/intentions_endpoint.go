package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

// /v1/connection/intentions
func (s *HTTPServer) IntentionEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.IntentionList(resp, req)

	case "POST":
		return s.IntentionCreate(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// GET /v1/connect/intentions
func (s *HTTPServer) IntentionList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

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

// POST /v1/connect/intentions
func (s *HTTPServer) IntentionCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	args := structs.IntentionRequest{
		Op: structs.IntentionOpCreate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req, &args.Intention, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return intentionCreateResponse{reply}, nil
}

// intentionCreateResponse is the response structure for creating an intention.
type intentionCreateResponse struct{ ID string }
