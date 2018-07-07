package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/structs"
)

// preparedQueryCreateResponse is used to wrap the query ID.
type preparedQueryCreateResponse struct {
	ID string
}

// preparedQueryCreate makes a new prepared query.
func (s *HTTPServer) preparedQueryCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQueryRequest{
		Op: structs.PreparedQueryCreate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := decodeBody(req, &args.Query, nil); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Request decode failed: %v", err)
		return nil, nil
	}

	var reply string
	if err := s.agent.RPC("PreparedQuery.Apply", &args, &reply); err != nil {
		return nil, err
	}
	return preparedQueryCreateResponse{reply}, nil
}

// preparedQueryList returns all the prepared queries.
func (s *HTTPServer) preparedQueryList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedPreparedQueries
	defer setMeta(resp, &reply.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("PreparedQuery.List", &args, &reply); err != nil {
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < reply.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	reply.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Use empty list instead of nil.
	if reply.Queries == nil {
		reply.Queries = make(structs.PreparedQueries, 0)
	}
	return reply.Queries, nil
}

// PreparedQueryGeneral handles all the general prepared query requests.
func (s *HTTPServer) PreparedQueryGeneral(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "POST":
		return s.preparedQueryCreate(resp, req)

	case "GET":
		return s.preparedQueryList(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "POST"}}
	}
}

// parseLimit parses the optional limit parameter for a prepared query execution.
func parseLimit(req *http.Request, limit *int) error {
	*limit = 0
	if arg := req.URL.Query().Get("limit"); arg != "" {
		i, err := strconv.Atoi(arg)
		if err != nil {
			return err
		}
		*limit = i
	}
	return nil
}

// preparedQueryExecute executes a prepared query.
func (s *HTTPServer) preparedQueryExecute(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQueryExecuteRequest{
		QueryIDOrName: id,
		Agent: structs.QuerySource{
			Node:       s.agent.config.NodeName,
			Datacenter: s.agent.config.Datacenter,
			Segment:    s.agent.config.SegmentName,
		},
	}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := parseLimit(req, &args.Limit); err != nil {
		return nil, fmt.Errorf("Bad limit: %s", err)
	}

	params := req.URL.Query()
	if raw := params.Get("connect"); raw != "" {
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("Error parsing 'connect' value: %s", err)
		}

		args.Connect = val
	}

	var reply structs.PreparedQueryExecuteResponse
	defer setMeta(resp, &reply.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("PreparedQuery.Execute", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < reply.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	reply.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()

	// Note that we translate using the DC that the results came from, since
	// a query can fail over to a different DC than where the execute request
	// was sent to. That's why we use the reply's DC and not the one from
	// the args.
	s.agent.TranslateAddresses(reply.Datacenter, reply.Nodes)

	// Use empty list instead of nil.
	if reply.Nodes == nil {
		reply.Nodes = make(structs.CheckServiceNodes, 0)
	}
	return reply, nil
}

// preparedQueryExplain shows which query a name resolves to, the fully
// interpolated template (if it's a template), as well as additional info
// about the execution of a query.
func (s *HTTPServer) preparedQueryExplain(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQueryExecuteRequest{
		QueryIDOrName: id,
		Agent: structs.QuerySource{
			Node:       s.agent.config.NodeName,
			Datacenter: s.agent.config.Datacenter,
			Segment:    s.agent.config.SegmentName,
		},
	}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := parseLimit(req, &args.Limit); err != nil {
		return nil, fmt.Errorf("Bad limit: %s", err)
	}

	var reply structs.PreparedQueryExplainResponse
	defer setMeta(resp, &reply.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("PreparedQuery.Explain", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < reply.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	reply.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	return reply, nil
}

// preparedQueryGet returns a single prepared query.
func (s *HTTPServer) preparedQueryGet(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQuerySpecificRequest{
		QueryID: id,
	}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.IndexedPreparedQueries
	defer setMeta(resp, &reply.QueryMeta)
RETRY_ONCE:
	if err := s.agent.RPC("PreparedQuery.Get", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(http.StatusNotFound)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}
	if args.QueryOptions.AllowStale && args.MaxStaleDuration > 0 && args.MaxStaleDuration < reply.LastContact {
		args.AllowStale = false
		args.MaxStaleDuration = 0
		goto RETRY_ONCE
	}
	reply.ConsistencyLevel = args.QueryOptions.ConsistencyLevel()
	return reply.Queries, nil
}

// preparedQueryUpdate updates a prepared query.
func (s *HTTPServer) preparedQueryUpdate(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQueryRequest{
		Op: structs.PreparedQueryUpdate,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if req.ContentLength > 0 {
		if err := decodeBody(req, &args.Query, nil); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "Request decode failed: %v", err)
			return nil, nil
		}
	}

	if args.Query == nil {
		args.Query = &structs.PreparedQuery{}
	}

	// Take the ID from the URL, not the embedded one.
	args.Query.ID = id

	var reply string
	if err := s.agent.RPC("PreparedQuery.Apply", &args, &reply); err != nil {
		return nil, err
	}
	return nil, nil
}

// preparedQueryDelete deletes prepared query.
func (s *HTTPServer) preparedQueryDelete(id string, resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.PreparedQueryRequest{
		Op: structs.PreparedQueryDelete,
		Query: &structs.PreparedQuery{
			ID: id,
		},
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	var reply string
	if err := s.agent.RPC("PreparedQuery.Apply", &args, &reply); err != nil {
		return nil, err
	}
	return nil, nil
}

// PreparedQuerySpecificOptions handles OPTIONS requests to prepared query endpoints.
func (s *HTTPServer) preparedQuerySpecificOptions(resp http.ResponseWriter, req *http.Request) interface{} {
	path := req.URL.Path
	switch {
	case strings.HasSuffix(path, "/execute"):
		resp.Header().Add("Allow", strings.Join([]string{"OPTIONS", "GET"}, ","))
		return resp

	case strings.HasSuffix(path, "/explain"):
		resp.Header().Add("Allow", strings.Join([]string{"OPTIONS", "GET"}, ","))
		return resp

	default:
		resp.Header().Add("Allow", strings.Join([]string{"OPTIONS", "GET", "PUT", "DELETE"}, ","))
		return resp
	}
}

// PreparedQuerySpecific handles all the prepared query requests specific to a
// particular query.
func (s *HTTPServer) PreparedQuerySpecific(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method == "OPTIONS" {
		return s.preparedQuerySpecificOptions(resp, req), nil
	}

	path := req.URL.Path
	id := strings.TrimPrefix(path, "/v1/query/")

	switch {
	case strings.HasSuffix(path, "/execute"):
		if req.Method != "GET" {
			return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
		}
		id = strings.TrimSuffix(id, "/execute")
		return s.preparedQueryExecute(id, resp, req)

	case strings.HasSuffix(path, "/explain"):
		if req.Method != "GET" {
			return nil, MethodNotAllowedError{req.Method, []string{"GET"}}
		}
		id = strings.TrimSuffix(id, "/explain")
		return s.preparedQueryExplain(id, resp, req)

	default:
		switch req.Method {
		case "GET":
			return s.preparedQueryGet(id, resp, req)

		case "PUT":
			return s.preparedQueryUpdate(id, resp, req)

		case "DELETE":
			return s.preparedQueryDelete(id, resp, req)

		default:
			return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
		}
	}
}
