package agent

import (
	"fmt"
	"net/http"
	"strings"

	cachetype "github.com/hashicorp/consul/agent/cache-types"
	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
)

// /v1/connect/intentions
func (s *HTTPHandlers) IntentionEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
func (s *HTTPHandlers) IntentionList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	var args structs.IntentionListRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	var reply structs.IndexedIntentions
	defer setMeta(resp, &reply.QueryMeta)
	if err := s.agent.RPC("Intention.List", &args, &reply); err != nil {
		return nil, err
	}

	return reply.Intentions, nil
}

// IntentionCreate is used to create legacy intentions.
// Deprecated: use IntentionPutExact.
func (s *HTTPHandlers) IntentionCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}
	if entMeta.PartitionOrDefault() != structs.PartitionOrDefault("") {
		return nil, BadRequestError{Reason: "Cannot use a partition with this endpoint"}
	}

	args := structs.IntentionRequest{
		Op: structs.IntentionOpCreate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req.Body, &args.Intention); err != nil {
		return nil, fmt.Errorf("Failed to decode request body: %s", err)
	}

	if args.Intention.DestinationPartition != "" && args.Intention.DestinationPartition != "default" {
		return nil, BadRequestError{Reason: "Cannot specify a destination partition with this endpoint"}
	}
	if args.Intention.SourcePartition != "" && args.Intention.SourcePartition != "default" {
		return nil, BadRequestError{Reason: "Cannot specify a source partition with this endpoint"}
	}

	args.Intention.FillPartitionAndNamespace(&entMeta, false)

	if err := s.validateEnterpriseIntention(args.Intention); err != nil {
		return nil, err
	}

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return intentionCreateResponse{reply}, nil
}

func (s *HTTPHandlers) validateEnterpriseIntention(ixn *structs.Intention) error {
	if err := s.validateEnterpriseIntentionPartition("SourcePartition", ixn.SourcePartition); err != nil {
		return err
	}
	if err := s.validateEnterpriseIntentionPartition("DestinationPartition", ixn.DestinationPartition); err != nil {
		return err
	}
	if err := s.validateEnterpriseIntentionNamespace("SourceNS", ixn.SourceNS, true); err != nil {
		return err
	}
	if err := s.validateEnterpriseIntentionNamespace("DestinationNS", ixn.DestinationNS, true); err != nil {
		return err
	}

	return nil
}

// GET /v1/connect/intentions/match
func (s *HTTPHandlers) IntentionMatch(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Prepare args
	args := &structs.IntentionQueryRequest{Match: &structs.IntentionQueryMatch{}}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
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
		ap, ns, name, err := parseIntentionStringComponent(n, &entMeta)
		if err != nil {
			return nil, fmt.Errorf("name %q is invalid: %s", n, err)
		}

		args.Match.Entries[i] = structs.IntentionMatchEntry{
			Partition: ap,
			Namespace: ns,
			Name:      name,
		}
	}

	// Make the RPC request
	var out structs.IndexedIntentionMatches
	defer setMeta(resp, &out.QueryMeta)

	if s.agent.config.HTTPUseCache && args.QueryOptions.UseCache {
		raw, m, err := s.agent.cache.Get(req.Context(), cachetype.IntentionMatchName, args)
		if err != nil {
			return nil, err
		}
		defer setCacheMeta(resp, &m)

		reply, ok := raw.(*structs.IndexedIntentionMatches)
		if !ok {
			// This should never happen, but we want to protect against panics
			return nil, fmt.Errorf("internal error: response type not correct")
		}
		out = *reply
	} else {
	RETRY_ONCE:
		if err := s.agent.RPC("Intention.Match", args, &out); err != nil {
			return nil, err
		}
		if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < out.LastContact {
			args.AllowStale = false
			args.MaxStaleDuration = 0
			goto RETRY_ONCE
		}
	}
	out.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// We must have an identical count of matches
	if len(out.Matches) != len(names) {
		return nil, fmt.Errorf("internal error: match response count didn't match input count")
	}

	// Use empty list instead of nil.
	response := make(map[string]structs.Intentions)
	for i, ixns := range out.Matches {
		response[names[i]] = ixns
	}

	return response, nil
}

// GET /v1/connect/intentions/check
func (s *HTTPHandlers) IntentionCheck(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Prepare args
	args := &structs.IntentionQueryRequest{Check: &structs.IntentionQueryCheck{}}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
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

	// We parse them the same way as matches to extract partition/namespace/name
	args.Check.SourceName = source[0]
	if args.Check.SourceType == structs.IntentionSourceConsul {
		ap, ns, name, err := parseIntentionStringComponent(source[0], &entMeta)
		if err != nil {
			return nil, fmt.Errorf("source %q is invalid: %s", source[0], err)
		}
		args.Check.SourcePartition = ap
		args.Check.SourceNS = ns
		args.Check.SourceName = name
	}

	// The destination is always in the Consul format
	ap, ns, name, err := parseIntentionStringComponent(destination[0], &entMeta)
	if err != nil {
		return nil, fmt.Errorf("destination %q is invalid: %s", destination[0], err)
	}
	args.Check.DestinationPartition = ap
	args.Check.DestinationNS = ns
	args.Check.DestinationName = name

	var reply structs.IntentionQueryCheckResponse
	if err := s.agent.RPC("Intention.Check", args, &reply); err != nil {
		return nil, err
	}

	return &reply, nil
}

// IntentionExact handles the endpoint for /v1/connect/intentions/exact
func (s *HTTPHandlers) IntentionExact(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.IntentionGetExact(resp, req)
	case "PUT":
		return s.IntentionPutExact(resp, req)
	case "DELETE":
		return s.IntentionDeleteExact(resp, req)
	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
	}
}

// GET /v1/connect/intentions/exact
func (s *HTTPHandlers) IntentionGetExact(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	args := structs.IntentionQueryRequest{
		Exact: &structs.IntentionQueryExact{},
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	q := req.URL.Query()

	// Extract the source/destination
	source, ok := q["source"]
	if !ok || len(source) != 1 {
		return nil, fmt.Errorf("required query parameter 'source' not set")
	}
	destination, ok := q["destination"]
	if !ok || len(destination) != 1 {
		return nil, fmt.Errorf("required query parameter 'destination' not set")
	}

	{
		ap, ns, name, err := parseIntentionStringComponent(source[0], &entMeta)
		if err != nil {
			return nil, fmt.Errorf("source %q is invalid: %s", source[0], err)
		}
		args.Exact.SourcePartition = ap
		args.Exact.SourceNS = ns
		args.Exact.SourceName = name
	}

	{
		ap, ns, name, err := parseIntentionStringComponent(destination[0], &entMeta)
		if err != nil {
			return nil, fmt.Errorf("destination %q is invalid: %s", destination[0], err)
		}
		args.Exact.DestinationPartition = ap
		args.Exact.DestinationNS = ns
		args.Exact.DestinationName = name
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

// PUT /v1/connect/intentions/exact
func (s *HTTPHandlers) IntentionPutExact(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	exact, err := parseIntentionQueryExact(req, &entMeta)
	if err != nil {
		return nil, err
	}

	args := structs.IntentionRequest{
		Op: structs.IntentionOpUpsert,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req.Body, &args.Intention); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	// Explicitly CLEAR the old legacy ID field
	args.Intention.ID = ""

	// Use the intention identity from the URL.
	args.Intention.SourcePartition = exact.SourcePartition
	args.Intention.SourceNS = exact.SourceNS
	args.Intention.SourceName = exact.SourceName
	args.Intention.DestinationPartition = exact.DestinationPartition
	args.Intention.DestinationNS = exact.DestinationNS
	args.Intention.DestinationName = exact.DestinationName

	args.Intention.FillPartitionAndNamespace(&entMeta, false)

	var ignored string
	if err := s.agent.RPC("Intention.Apply", &args, &ignored); err != nil {
		return nil, err
	}

	return true, nil
}

// DELETE /v1/connect/intentions/exact
func (s *HTTPHandlers) IntentionDeleteExact(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}

	exact, err := parseIntentionQueryExact(req, &entMeta)
	if err != nil {
		return nil, err
	}

	args := structs.IntentionRequest{
		Op: structs.IntentionOpDelete,
		Intention: &structs.Intention{
			// NOTE: ID is explicitly empty here
			SourcePartition:      exact.SourcePartition,
			SourceNS:             exact.SourceNS,
			SourceName:           exact.SourceName,
			DestinationPartition: exact.DestinationPartition,
			DestinationNS:        exact.DestinationNS,
			DestinationName:      exact.DestinationName,
		},
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	var ignored string
	if err := s.agent.RPC("Intention.Apply", &args, &ignored); err != nil {
		return nil, err
	}

	return true, nil
}

// intentionCreateResponse is the response structure for creating an intention.
type intentionCreateResponse struct{ ID string }

func parseIntentionQueryExact(req *http.Request, entMeta *structs.EnterpriseMeta) (*structs.IntentionQueryExact, error) {
	q := req.URL.Query()

	// Extract the source/destination
	source, ok := q["source"]
	if !ok || len(source) != 1 || source[0] == "" {
		return nil, fmt.Errorf("required query parameter 'source' not set")
	}
	destination, ok := q["destination"]
	if !ok || len(destination) != 1 || destination[0] == "" {
		return nil, fmt.Errorf("required query parameter 'destination' not set")
	}

	var exact structs.IntentionQueryExact
	{
		ap, ns, name, err := parseIntentionStringComponent(source[0], entMeta)
		if err != nil {
			return nil, fmt.Errorf("source %q is invalid: %s", source[0], err)
		}
		exact.SourcePartition = ap
		exact.SourceNS = ns
		exact.SourceName = name
	}

	{
		ap, ns, name, err := parseIntentionStringComponent(destination[0], entMeta)
		if err != nil {
			return nil, fmt.Errorf("destination %q is invalid: %s", destination[0], err)
		}
		exact.DestinationPartition = ap
		exact.DestinationNS = ns
		exact.DestinationName = name
	}

	return &exact, nil
}

func parseIntentionStringComponent(input string, entMeta *structs.EnterpriseMeta) (string, string, string, error) {
	ss := strings.Split(input, "/")
	switch len(ss) {
	case 1: // Name only
		ns := entMeta.NamespaceOrEmpty()
		ap := entMeta.PartitionOrEmpty()
		return ap, ns, ss[0], nil
	case 2: // namespace/name
		ap := entMeta.PartitionOrEmpty()
		return ap, ss[0], ss[1], nil
	case 3: // partition/namespace/name
		return ss[0], ss[1], ss[2], nil
	default:
		return "", "", "", fmt.Errorf("input can contain at most two '/'")
	}
}

// IntentionSpecific handles the endpoint for /v1/connect/intentions/:id.
// Deprecated: use IntentionExact.
func (s *HTTPHandlers) IntentionSpecific(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

// Deprecated: use IntentionGetExact.
func (s *HTTPHandlers) IntentionSpecificGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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

// Deprecated: use IntentionPutExact.
func (s *HTTPHandlers) IntentionSpecificUpdate(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Method is tested in IntentionEndpoint

	var entMeta structs.EnterpriseMeta
	if err := s.parseEntMetaNoWildcard(req, &entMeta); err != nil {
		return nil, err
	}
	if entMeta.PartitionOrDefault() != structs.PartitionOrDefault("") {
		return nil, BadRequestError{Reason: "Cannot use a partition with this endpoint"}
	}

	args := structs.IntentionRequest{
		Op: structs.IntentionOpUpdate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req.Body, &args.Intention); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Request decode failed: %v", err)}
	}

	if args.Intention.DestinationPartition != "" && args.Intention.DestinationPartition != "default" {
		return nil, BadRequestError{Reason: "Cannot specify a destination partition with this endpoint"}
	}
	if args.Intention.SourcePartition != "" && args.Intention.SourcePartition != "default" {
		return nil, BadRequestError{Reason: "Cannot specify a source partition with this endpoint"}
	}

	args.Intention.FillPartitionAndNamespace(&entMeta, false)

	// Use the ID from the URL
	args.Intention.ID = id

	var reply string
	if err := s.agent.RPC("Intention.Apply", &args, &reply); err != nil {
		return nil, err
	}

	// Update uses the same create response
	return intentionCreateResponse{reply}, nil
}

// Deprecated: use IntentionDeleteExact.
func (s *HTTPHandlers) IntentionSpecificDelete(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
