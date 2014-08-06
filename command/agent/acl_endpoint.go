package agent

import (
	"fmt"
	"github.com/hashicorp/consul/consul/structs"
	"net/http"
	"strings"
)

// aclCreateResponse is used to wrap the ACL ID
type aclCreateResponse struct {
	ID string
}

func (s *HTTPServer) ACLDelete(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ACLRequest{
		Op: structs.ACLDelete,
	}
	s.parseDC(req, &args.Datacenter)

	// Pull out the session id
	args.ACL.ID = strings.TrimPrefix(req.URL.Path, "/v1/acl/delete/")
	if args.ACL.ID == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing ACL"))
		return nil, nil
	}

	var out string
	if err := s.agent.RPC("ACL.Apply", &args, &out); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *HTTPServer) ACLCreate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.aclSet(resp, req, false)
}

func (s *HTTPServer) ACLUpdate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	return s.aclSet(resp, req, true)
}

func (s *HTTPServer) aclSet(resp http.ResponseWriter, req *http.Request, update bool) (interface{}, error) {
	// Mandate a PUT request
	if req.Method != "PUT" {
		resp.WriteHeader(405)
		return nil, nil
	}

	args := structs.ACLRequest{
		Op: structs.ACLSet,
		ACL: structs.ACL{
			Type: structs.ACLTypeClient,
		},
	}
	s.parseDC(req, &args.Datacenter)

	// Handle optional request body
	if req.ContentLength > 0 {
		if err := decodeBody(req, &args.ACL, nil); err != nil {
			resp.WriteHeader(400)
			resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
			return nil, nil
		}
	}

	// Ensure there is no ID set for create
	if !update && args.ACL.ID != "" {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("ACL ID cannot be set")))
		return nil, nil
	}

	// Ensure there is an ID set for update
	if update && args.ACL.ID == "" {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("ACL ID must be set")))
		return nil, nil
	}

	// Create the acl, get the ID
	var out string
	if err := s.agent.RPC("ACL.Apply", &args, &out); err != nil {
		return nil, err
	}

	// Format the response as a JSON object
	return aclCreateResponse{out}, nil
}

func (s *HTTPServer) ACLClone(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ACLSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the session id
	args.ACL = strings.TrimPrefix(req.URL.Path, "/v1/acl/clone/")
	if args.ACL == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing ACL"))
		return nil, nil
	}

	var out structs.IndexedACLs
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.Get", &args, &out); err != nil {
		return nil, err
	}

	// Bail if the ACL is not found
	if len(out.ACLs) == 0 {
		resp.WriteHeader(404)
		resp.Write([]byte(fmt.Sprintf("Target ACL not found")))
		return nil, nil
	}

	// Create a new ACL
	createArgs := structs.ACLRequest{
		Datacenter: args.Datacenter,
		Op:         structs.ACLSet,
		ACL:        *out.ACLs[0],
	}
	createArgs.ACL.ID = ""

	// Create the acl, get the ID
	var outID string
	if err := s.agent.RPC("ACL.Apply", &createArgs, &outID); err != nil {
		return nil, err
	}

	// Format the response as a JSON object
	return aclCreateResponse{outID}, nil
}

func (s *HTTPServer) ACLGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ACLSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the session id
	args.ACL = strings.TrimPrefix(req.URL.Path, "/v1/acl/info/")
	if args.ACL == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing ACL"))
		return nil, nil
	}

	var out structs.IndexedACLs
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.Get", &args, &out); err != nil {
		return nil, err
	}
	return out.ACLs, nil
}

func (s *HTTPServer) ACLList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out structs.IndexedACLs
	defer setMeta(resp, &out.QueryMeta)
	if err := s.agent.RPC("ACL.List", &args, &out); err != nil {
		return nil, err
	}
	return out.ACLs, nil
}
