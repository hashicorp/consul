package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/consul"
	"github.com/hashicorp/consul/agent/consul/structs"
)

const (
	preparedQueryExecuteSuffix = "/execute"
	preparedQueryExplainSuffix = "/explain"
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
	if req.ContentLength > 0 {
		if err := decodeBody(req, &args.Query, nil); err != nil {
			resp.WriteHeader(400)
			fmt.Fprintf(resp, "Request decode failed: %v", err)
			return nil, nil
		}
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
	if err := s.agent.RPC("PreparedQuery.List", &args, &reply); err != nil {
		return nil, err
	}

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
		resp.WriteHeader(405)
		return nil, nil
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
		},
	}
	s.parseSource(req, &args.Source)
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := parseLimit(req, &args.Limit); err != nil {
		return nil, fmt.Errorf("Bad limit: %s", err)
	}

	var reply structs.PreparedQueryExecuteResponse
	if err := s.agent.RPC("PreparedQuery.Execute", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(404)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}

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
	if err := s.agent.RPC("PreparedQuery.Explain", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(404)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}
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
	if err := s.agent.RPC("PreparedQuery.Get", &args, &reply); err != nil {
		// We have to check the string since the RPC sheds
		// the specific error type.
		if err.Error() == consul.ErrQueryNotFound.Error() {
			resp.WriteHeader(404)
			fmt.Fprint(resp, err.Error())
			return nil, nil
		}
		return nil, err
	}
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
			resp.WriteHeader(400)
			fmt.Fprintf(resp, "Request decode failed: %v", err)
			return nil, nil
		}
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

// PreparedQuerySpecific handles all the prepared query requests specific to a
// particular query.
func (s *HTTPServer) PreparedQuerySpecific(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	id := strings.TrimPrefix(req.URL.Path, "/v1/query/")

	execute, explain := false, false
	if strings.HasSuffix(id, preparedQueryExecuteSuffix) {
		execute = true
		id = strings.TrimSuffix(id, preparedQueryExecuteSuffix)
	} else if strings.HasSuffix(id, preparedQueryExplainSuffix) {
		explain = true
		id = strings.TrimSuffix(id, preparedQueryExplainSuffix)
	}

	switch req.Method {
	case "GET":
		if execute {
			return s.preparedQueryExecute(id, resp, req)
		} else if explain {
			return s.preparedQueryExplain(id, resp, req)
		} else {
			return s.preparedQueryGet(id, resp, req)
		}

	case "PUT":
		return s.preparedQueryUpdate(id, resp, req)

	case "DELETE":
		return s.preparedQueryDelete(id, resp, req)

	default:
		resp.WriteHeader(405)
		return nil, nil
	}
}
