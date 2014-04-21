package agent

import (
	"bytes"
	"github.com/hashicorp/consul/consul/structs"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func (s *HTTPServer) KVSEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.KeyRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the key name, validation left to each sub-handler
	args.Key = strings.TrimPrefix(req.URL.Path, "/v1/kv/")

	// Switch on the method
	switch req.Method {
	case "GET":
		return s.KVSGet(resp, req, &args)
	case "PUT":
		return s.KVSPut(resp, req, &args)
	case "DELETE":
		return s.KVSDelete(resp, req, &args)
	default:
		resp.WriteHeader(405)
		return nil, nil
	}
	return nil, nil
}

// KVSGet handles a GET request
func (s *HTTPServer) KVSGet(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	// Check for recurse
	method := "KVS.Get"
	params := req.URL.Query()
	if _, ok := params["recurse"]; ok {
		method = "KVS.List"
	} else if missingKey(resp, args) {
		return nil, nil
	}

	// Make the RPC
	var out structs.IndexedDirEntries
	if err := s.agent.RPC(method, &args, &out); err != nil {
		return nil, err
	}
	setIndex(resp, out.Index)

	// Check if we get a not found
	if len(out.Entries) == 0 {
		resp.WriteHeader(404)
		return nil, nil
	}
	return out.Entries, nil
}

// KVSPut handles a PUT request
func (s *HTTPServer) KVSPut(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	if missingKey(resp, args) {
		return nil, nil
	}
	applyReq := structs.KVSRequest{
		Datacenter: args.Datacenter,
		Op:         structs.KVSSet,
		DirEnt: structs.DirEntry{
			Key:   args.Key,
			Flags: 0,
			Value: nil,
		},
	}

	// Check for flags
	params := req.URL.Query()
	if _, ok := params["flags"]; ok {
		flagVal, err := strconv.ParseUint(params.Get("flags"), 10, 64)
		if err != nil {
			return nil, err
		}
		applyReq.DirEnt.Flags = flagVal
	}

	// Check for cas value
	if _, ok := params["cas"]; ok {
		casVal, err := strconv.ParseUint(params.Get("cas"), 10, 64)
		if err != nil {
			return nil, err
		}
		applyReq.DirEnt.ModifyIndex = casVal
		applyReq.Op = structs.KVSCAS
	}

	// Copy the value
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, req.Body); err != nil {
		return nil, err
	}
	applyReq.DirEnt.Value = buf.Bytes()

	// Make the RPC
	var out bool
	if err := s.agent.RPC("KVS.Apply", &applyReq, &out); err != nil {
		return nil, err
	}

	// Only use the out value if this was a CAS
	if applyReq.Op == structs.KVSSet {
		return true, nil
	} else {
		return out, nil
	}
}

// KVSPut handles a DELETE request
func (s *HTTPServer) KVSDelete(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	applyReq := structs.KVSRequest{
		Datacenter: args.Datacenter,
		Op:         structs.KVSDelete,
		DirEnt: structs.DirEntry{
			Key: args.Key,
		},
	}

	// Check for recurse
	params := req.URL.Query()
	if _, ok := params["recurse"]; ok {
		applyReq.Op = structs.KVSDeleteTree
	} else if missingKey(resp, args) {
		return nil, nil
	}

	// Make the RPC
	var out bool
	if err := s.agent.RPC("KVS.Apply", &applyReq, &out); err != nil {
		return nil, err
	}
	return nil, nil
}

// missingKey checks if the key is missing
func missingKey(resp http.ResponseWriter, args *structs.KeyRequest) bool {
	if args.Key == "" {
		resp.WriteHeader(400)
		resp.Write([]byte("Missing key name"))
		return true
	}
	return false
}
