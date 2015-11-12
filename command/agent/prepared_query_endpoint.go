package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/consul/structs"
)

const (
	preparedQueryEndpoint      = "PreparedQuery"
	preparedQueryExecuteSuffix = "/execute"
)

// preparedQueryCreateResponse is used to wrap the query ID.
type preparedQueryCreateResponse struct {
	ID string
}

// PreparedQueryGeneral handles all the general prepared query requests.
func (s *HTTPServer) PreparedQueryGeneral(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	endpoint := s.agent.getEndpoint(preparedQueryEndpoint)
	switch req.Method {
	case "POST": // Create a new prepared query.
		args := structs.PreparedQueryRequest{
			Op: structs.PreparedQueryCreate,
		}
		s.parseDC(req, &args.Datacenter)
		s.parseToken(req, &args.Token)
		if req.ContentLength > 0 {
			if err := decodeBody(req, &args.Query, nil); err != nil {
				resp.WriteHeader(400)
				resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
				return nil, nil
			}
		}

		var reply string
		if err := s.agent.RPC(endpoint+".Apply", &args, &reply); err != nil {
			return nil, err
		}
		return preparedQueryCreateResponse{reply}, nil

	case "GET": // List all the prepared queries.
		var args structs.DCSpecificRequest
		if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
			return nil, nil
		}

		var reply structs.IndexedPreparedQueries
		if err := s.agent.RPC(endpoint+".List", &args, &reply); err != nil {
			return nil, err
		}
		return reply.Queries, nil

	default:
		resp.WriteHeader(405)
		return nil, nil
	}
}

// parseLimit parses the optional limit parameter for a prepared query execution.
func parseLimit(req *http.Request, limit *int) error {
	*limit = 0
	if arg := req.URL.Query().Get("limit"); arg != "" {
		if i, err := strconv.Atoi(arg); err != nil {
			return err
		} else {
			*limit = i
		}
	}
	return nil
}

// PreparedQuerySpecifc handles all the prepared query requests specific to a
// particular query.
func (s *HTTPServer) PreparedQuerySpecific(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	id := strings.TrimPrefix(req.URL.Path, "/v1/query/")
	execute := false
	if strings.HasSuffix(id, preparedQueryExecuteSuffix) {
		execute = true
		id = strings.TrimSuffix(id, preparedQueryExecuteSuffix)
	}

	endpoint := s.agent.getEndpoint(preparedQueryEndpoint)
	switch req.Method {
	case "GET": // Execute or retrieve a prepared query.
		if execute {
			args := structs.PreparedQueryExecuteRequest{
				QueryIDOrName: id,
			}
			s.parseSource(req, &args.Source)
			if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
				return nil, nil
			}
			if err := parseLimit(req, &args.Limit); err != nil {
				return nil, fmt.Errorf("Bad limit: %s", err)
			}

			var reply structs.PreparedQueryExecuteResponse
			if err := s.agent.RPC(endpoint+".Execute", &args, &reply); err != nil {
				return nil, err
			}
			return reply, nil
		} else {
			args := structs.PreparedQuerySpecificRequest{
				QueryID: id,
			}
			if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
				return nil, nil
			}

			var reply structs.IndexedPreparedQueries
			if err := s.agent.RPC(endpoint+".Get", &args, &reply); err != nil {
				return nil, err
			}
			return reply.Queries, nil
		}

	case "PUT": // Update an existing prepared query.
		args := structs.PreparedQueryRequest{
			Op: structs.PreparedQueryUpdate,
		}
		s.parseDC(req, &args.Datacenter)
		s.parseToken(req, &args.Token)
		if req.ContentLength > 0 {
			if err := decodeBody(req, &args.Query, nil); err != nil {
				resp.WriteHeader(400)
				resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
				return nil, nil
			}
		}

		// Take the ID from the URL, not the embedded one.
		args.Query.ID = id

		var reply string
		if err := s.agent.RPC(endpoint+".Apply", &args, &reply); err != nil {
			return nil, err
		}
		return nil, nil

	case "DELETE": // Delete a prepared query.
		args := structs.PreparedQueryRequest{
			Op: structs.PreparedQueryDelete,
			Query: &structs.PreparedQuery{
				ID: id,
			},
		}
		s.parseDC(req, &args.Datacenter)
		s.parseToken(req, &args.Token)

		var reply string
		if err := s.agent.RPC(endpoint+".Apply", &args, &reply); err != nil {
			return nil, err
		}
		return nil, nil

	default:
		resp.WriteHeader(405)
		return nil, nil
	}
}
