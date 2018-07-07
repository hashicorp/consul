package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
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
		return nil, fmt.Errorf("Failed to decode request body: %s", err)
	}

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return intentionCreateResponse{reply}, nil
}

// GET /v1/connect/intentions/match
func (s *HTTPServer) IntentionMatch(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Prepare args
	args := &structs.IntentionQueryRequest{Match: &structs.IntentionQueryMatch{}}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	q := req.URL.Query()

	// Extract the "by" query parameter
	if by, ok := q["by"]; !ok || len(by) != 1 {
		return nil, fmt.Errorf("required query parameter 'by' not set")
	} else {
		switch v := structs.IntentionMatchType(by[0]); v {
		case structs.IntentionMatchSource, structs.IntentionMatchDestination:
			args.Match.Type = v
		default:
			return nil, fmt.Errorf("'by' parameter must be one of 'source' or 'destination'")
		}
	}

	// Extract all the match names
	names, ok := q["name"]
	if !ok || len(names) == 0 {
		return nil, fmt.Errorf("required query parameter 'name' not set")
	}

	// Build the entries in order. The order matters since that is the
	// order of the returned responses.
	args.Match.Entries = make([]structs.IntentionMatchEntry, len(names))
	for i, n := range names {
		entry, err := parseIntentionMatchEntry(n)
		if err != nil {
			return nil, fmt.Errorf("name %q is invalid: %s", n, err)
		}

		args.Match.Entries[i] = entry
	}

	var reply structs.IndexedIntentionMatches
	if err := s.agent.RPC("Intention.Match", args, &reply); err != nil {
		return nil, err
	}

	// We must have an identical count of matches
	if len(reply.Matches) != len(names) {
		return nil, fmt.Errorf("internal error: match response count didn't match input count")
	}

	// Use empty list instead of nil.
	response := make(map[string]structs.Intentions)
	for i, ixns := range reply.Matches {
		response[names[i]] = ixns
	}

	return response, nil
}

// GET /v1/connect/intentions/check
func (s *HTTPServer) IntentionCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Prepare args
	args := &structs.IntentionQueryRequest{Check: &structs.IntentionQueryCheck{}}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	q := req.URL.Query()

	// Set the source type if set
	args.Check.SourceType = structs.IntentionSourceConsul
	if sourceType, ok := q["source-type"]; ok && len(sourceType) > 0 {
		args.Check.SourceType = structs.IntentionSourceType(sourceType[0])
	}

	// Extract the source/destination
	source, ok := q["source"]
	if !ok || len(source) != 1 {
		return nil, fmt.Errorf("required query parameter 'source' not set")
	}
	destination, ok := q["destination"]
	if !ok || len(destination) != 1 {
		return nil, fmt.Errorf("required query parameter 'destination' not set")
	}

	// We parse them the same way as matches to extract namespace/name
	args.Check.SourceName = source[0]
	if args.Check.SourceType == structs.IntentionSourceConsul {
		entry, err := parseIntentionMatchEntry(source[0])
		if err != nil {
			return nil, fmt.Errorf("source %q is invalid: %s", source[0], err)
		}
		args.Check.SourceNS = entry.Namespace
		args.Check.SourceName = entry.Name
	}

	// The destination is always in the Consul format
	entry, err := parseIntentionMatchEntry(destination[0])
	if err != nil {
		return nil, fmt.Errorf("destination %q is invalid: %s", destination[0], err)
	}
	args.Check.DestinationNS = entry.Namespace
	args.Check.DestinationName = entry.Name

	var reply structs.IntentionQueryCheckResponse
	if err := s.agent.RPC("Intention.Check", args, &reply); err != nil {
		return nil, err
	}

	return &reply, nil
}

// IntentionSpecific handles the endpoint for /v1/connection/intentions/:id
func (s *HTTPServer) IntentionSpecific(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	id := strings.TrimPrefix(req.URL.Path, "/v1/connect/intentions/")

	switch req.Method {
	case "GET":
		return s.IntentionSpecificGet(id, resp, req)

	case "PUT":
		return s.IntentionSpecificUpdate(id, resp, req)

	case "DELETE":
		return s.IntentionSpecificDelete(id, resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
	}
}

// GET /v1/connect/intentions/:id
func (s *HTTPServer) IntentionSpecificGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	args := structs.IntentionQueryRequest{
		IntentionID: id,
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedIntentions
	if err := s.agent.RPC("Intention.Get", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds the error type
		if err.Error() == consul.ErrIntentionNotFound.Error() {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}

		// Not ideal, but there are a number of error scenarios that are not
		// user error (400). We look for a specific case of invalid UUID
		// to detect a parameter error and return a 400 response. The error
		// is not a constant type or message, so we have to use strings.Contains
		if strings.Contains(err.Error(), "UUID") {
			return nil, BadRequestError{Reason: err.Error()}
		}

		return nil, err
	}

	// This shouldn't happen since the called API documents it shouldn't,
	// but we check since the alternative if it happens is a panic.
	if len(reply.Intentions) == 0 {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	return reply.Intentions[0], nil
}

// PUT /v1/connect/intentions/:id
func (s *HTTPServer) IntentionSpecificUpdate(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	args := structs.IntentionRequest{
		Op: structs.IntentionOpUpdate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req, &args.Intention, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	// Use the ID from the URL
	args.Intention.ID = id

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	// Update uses the same create response
	return intentionCreateResponse{reply}, nil

}

// DELETE /v1/connect/intentions/:id
func (s *HTTPServer) IntentionSpecificDelete(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	args := structs.IntentionRequest{
		Op:        structs.IntentionOpDelete,
		Intention: &structs.Intention{ID: id},
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return true, nil
}

// intentionCreateResponse is the response structure for creating an intention.
type intentionCreateResponse struct{ ID string }

// parseIntentionMatchEntry parses the query parameter for an intention
// match query entry.
func parseIntentionMatchEntry(input string) (structs.IntentionMatchEntry, error) {
	var result structs.IntentionMatchEntry
	result.Namespace = structs.IntentionDefaultNamespace

	// TODO(mitchellh): when namespaces are introduced, set the default
	// namespace to be the namespace of the requestor.

	// Get the index to the '/'. If it doesn't exist, we have just a name
	// so just set that and return.
	idx := strings.IndexByte(input, '/')
	if idx == -1 {
		result.Name = input
		return result, nil
	}

	result.Namespace = input[:idx]
	result.Name = input[idx+1:]
	if strings.IndexByte(result.Name, '/') != -1 {
		return result, fmt.Errorf("input can contain at most one '/'")
	}

	return result, nil
}
