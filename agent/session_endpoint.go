package agent

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
)

// sessionCreateResponse is used to wrap the session ID
type sessionCreateResponse struct {
	ID string
}

// SessionCreate is used to create a new session
func (s *HTTPServer) SessionCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Default the session to our node + serf check + release session
	// invalidate behavior.
	args := structs.SessionRequest{
		Op: structs.SessionCreate,
		Session: structs.Session{
			Node:      s.agent.config.NodeName,
			Checks:    []types.CheckID{structs.SerfCheckID},
			LockDelay: 15 * time.Second,
			Behavior:  structs.SessionKeysRelease,
			TTL:       "",
		},
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := s.parseEntMeta(req, &args.Session.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Handle optional request body
	if req.ContentLength > 0 {
		if err := s.rewordUnknownEnterpriseFieldError(lib.DecodeJSON(req.Body, &args.Session)); err != nil {
			resp.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(resp, "Request decode failed: %v", err)
			return nil, nil
		}
	}

	// Create the session, get the ID
	var out string
	if err := s.agent.RPC("Session.Apply", &args, &out); err != nil {
		return nil, err
	}

	// Format the response as a JSON object
	return sessionCreateResponse{out}, nil
}

// FixupChecks is used to handle parsing the JSON body to default-add the Serf
// health check if they didn't specify any checks, but to allow an empty list
// to take out the Serf health check. This behavior broke when mapstructure was
// updated after 0.9.3, likely because we have a type wrapper around the string.
func FixupChecks(raw interface{}, s *structs.Session) error {
	rawMap, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	for k := range rawMap {
		if strings.ToLower(k) == "checks" {
			// If they supplied a checks key in the JSON, then
			// remove the default entries and respect whatever they
			// specified.
			s.Checks = nil
			return nil
		}
	}
	return nil
}

// SessionDestroy is used to destroy an existing session
func (s *HTTPServer) SessionDestroy(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionRequest{
		Op: structs.SessionDestroy,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	if err := s.parseEntMeta(req, &args.Session.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Pull out the session id
	args.Session.ID = strings.TrimPrefix(req.URL.Path, "/v1/session/destroy/")
	if args.Session.ID == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing session")
		return nil, nil
	}

	var out string
	if err := s.agent.RPC("Session.Apply", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

// SessionRenew is used to renew the TTL on an existing TTL session
func (s *HTTPServer) SessionRenew(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Pull out the session id
	args.SessionID = strings.TrimPrefix(req.URL.Path, "/v1/session/renew/")
	if args.SessionID == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing session")
		return nil, nil
	}

	var out structs.IndexedSessions
	if err := s.agent.RPC("Session.Renew", &args, &out); err != nil {
		return nil, err
	} else if out.Sessions == nil {
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(resp, "Session id '%s' not found", args.SessionID)
		return nil, nil
	}

	return out.Sessions, nil
}

// SessionGet is used to get info for a particular session
func (s *HTTPServer) SessionGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Pull out the session id
	args.SessionID = strings.TrimPrefix(req.URL.Path, "/v1/session/info/")
	if args.SessionID == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing session")
		return nil, nil
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.Get", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.Sessions == nil {
		out.Sessions = make(structs.Sessions, 0)
	}
	return out.Sessions, nil
}

// SessionList is used to list all the sessions
func (s *HTTPServer) SessionList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.SessionSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.List", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.Sessions == nil {
		out.Sessions = make(structs.Sessions, 0)
	}
	return out.Sessions, nil
}

// SessionsForNode returns all the nodes belonging to a node
func (s *HTTPServer) SessionsForNode(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.NodeSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Pull out the node name
	args.Node = strings.TrimPrefix(req.URL.Path, "/v1/session/node/")
	if args.Node == "" {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(resp, "Missing node name")
		return nil, nil
	}

	var out structs.IndexedSessions
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("Session.NodeSessions", &args, &out); err != nil {
		return nil, err
	}

	// Use empty list instead of nil
	if out.Sessions == nil {
		out.Sessions = make(structs.Sessions, 0)
	}
	return out.Sessions, nil
}
