package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

// SessionCreate is used to create a new session
func (s *HTTPServer) SessionCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Default the session to our node + serf check
	args := structs.SessionRequest{
		Op: structs.SessionCreate,
		Session: structs.Session{
			Node:   s.agent.config.NodeName,
			Checks: []string{consul.SerfCheckID},
		},
	}
	s.parseDC(req, &args.Datacenter)

	// Handle optional request body
	if req.ContentLength > 0 {
		if err := decodeBody(req, &args.Session, nil); err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
			return nil, nil
		}
	}

	// Create the session, get the ID
	var out string
	if err := s.agent.RPC("Session.Apply", &args, &out); err != nil {
		return nil, err
	}

	// Format the response as a JSON object
	type response struct {
		ID string
	}
	return response{out}, nil
}

// SessionDestroy is used to destroy an existing session
func (s *HTTPServer) SessionDestroy(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionRequest{
		Op: structs.SessionDestroy,
	}
	s.parseDC(req, &args.Datacenter)

	// Pull out the session id
	args.Session.ID = strings.TrimPrefix(req.URL.Path, "/v1/session/destroy/")
	if args.Session.ID == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing session"))
		return nil, nil
	}

	var out string
	if err := s.agent.RPC("Session.Apply", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

// SessionGet is used to get info for a particular session
func (s *HTTPServer) SessionGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the session id
	args.Session = strings.TrimPrefix(req.URL.Path, "/v1/session/info/")
	if args.Session == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing session"))
		return nil, nil
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.Get", &args, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

// SessionList is used to list all the sessions
func (s *HTTPServer) SessionList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.List", &args, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}

// SessionsForNode returns all the nodes belonging to a node
func (s *HTTPServer) SessionsForNode(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/session/node/")
	if args.Node == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing node name"))
		return nil, nil
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.NodeSessions", &args, &out); err != nil {
		return nil, err
	}
	return out.Sessions, nil
}
