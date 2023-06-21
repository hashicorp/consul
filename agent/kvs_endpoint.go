// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package agent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

func (s *HTTPHandlers) KVSEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.KeyRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// Pull out the key name, validation left to each sub-handler
	args.Key = strings.TrimPrefix(req.URL.Path, "/v1/kv/")

	// Check for a key list
	keyList := false
	params := req.URL.Query()
	if _, ok := params["keys"]; ok {
		keyList = true
	}

	// Switch on the method
	switch req.Method {
	case "GET":
		if keyList {
			return s.KVSGetKeys(resp, req, &args)
		}
		return s.KVSGet(resp, req, &args)
	case "PUT":
		return s.KVSPut(resp, req, &args)
	case "DELETE":
		return s.KVSDelete(resp, req, &args)
	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "PUT", "DELETE"}}
	}
}

// KVSGet handles a GET request
func (s *HTTPHandlers) KVSGet(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	// Check for recurse
	method := "KVS.Get"
	params := req.URL.Query()
	if _, ok := params["recurse"]; ok {
		method = "KVS.List"
	} else if args.Key == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing key name"}
	}

	// Do not allow wildcard NS on GET reqs
	if method == "KVS.Get" {
		if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
			return nil, err
		}
	} else {
		if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
			return nil, err
		}
	}

	// Make the RPC
	var out structs.IndexedDirEntries
	if err := s.agent.RPC(req.Context(), method, args, &out); err != nil {
		return nil, err
	}
	setMeta(resp, &out.QueryMeta)

	// Check if we get a not found
	if len(out.Entries) == 0 {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	// Check if we are in raw mode with a normal get, write out the raw body
	// while setting the Content-Type, Content-Security-Policy, and
	// X-Content-Type-Options headers to prevent XSS attacks from malicious KV
	// entries. Otherwise, the net/http server will sniff the body to set the
	// Content-Type. The nosniff option then indicates to the browser that it
	// should also skip sniffing the body, otherwise it might ignore the Content-Type
	// header in some situations. The sandbox option provides another layer of defense
	// using the browser's content security policy to prevent code execution.
	if _, ok := params["raw"]; ok && method == "KVS.Get" {
		body := out.Entries[0].Value
		resp.Header().Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("X-Content-Type-Options", "nosniff")
		resp.Header().Set("Content-Security-Policy", "sandbox")
		resp.Write(body)
		return nil, nil
	}

	return out.Entries, nil
}

// KVSGetKeys handles a GET request for keys
func (s *HTTPHandlers) KVSGetKeys(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}

	// Check for a separator, due to historic spelling error,
	// we now are forced to check for both spellings
	var sep string
	params := req.URL.Query()
	if _, ok := params["seperator"]; ok {
		sep = params.Get("seperator")
	}
	if _, ok := params["separator"]; ok {
		sep = params.Get("separator")
	}

	// Construct the args
	listArgs := structs.KeyListRequest{
		Datacenter:     args.Datacenter,
		Prefix:         args.Key,
		Seperator:      sep,
		EnterpriseMeta: args.EnterpriseMeta,
		QueryOptions:   args.QueryOptions,
	}

	// Make the RPC
	var out structs.IndexedKeyList
	if err := s.agent.RPC(req.Context(), "KVS.ListKeys", &listArgs, &out); err != nil {
		return nil, err
	}
	setMeta(resp, &out.QueryMeta)

	// Check if we get a not found. We do not generate
	// not found for the root, but just provide the empty list
	if len(out.Keys) == 0 && listArgs.Prefix != "" {
		resp.WriteHeader(http.StatusNotFound)
		return nil, nil
	}

	// Use empty list instead of null
	if out.Keys == nil {
		out.Keys = []string{}
	}
	return out.Keys, nil
}

// KVSPut handles a PUT request
func (s *HTTPHandlers) KVSPut(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if args.Key == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing key name"}
	}
	if conflictingFlags(resp, req, "cas", "acquire", "release") {
		return nil, nil
	}
	applyReq := structs.KVSRequest{
		Datacenter: args.Datacenter,
		Op:         api.KVSet,
		DirEnt: structs.DirEntry{
			Key:            args.Key,
			Flags:          0,
			Value:          nil,
			EnterpriseMeta: args.EnterpriseMeta,
		},
	}
	applyReq.Token = args.Token

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
		applyReq.Op = api.KVCAS
	}

	// Check for lock acquisition
	if _, ok := params["acquire"]; ok {
		applyReq.DirEnt.Session = params.Get("acquire")
		applyReq.Op = api.KVLock
	}

	// Check for lock release
	if _, ok := params["release"]; ok {
		applyReq.DirEnt.Session = params.Get("release")
		applyReq.Op = api.KVUnlock
	}

	// Check the content-length
	if req.ContentLength > int64(s.agent.config.KVMaxValueSize) {
		return nil, HTTPError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Reason: fmt.Sprintf("Request body(%d bytes) too large, max size: %d bytes. See %s.",
				req.ContentLength, s.agent.config.KVMaxValueSize, "https://www.consul.io/docs/agent/config/config-files#kv_max_value_size"),
		}
	}

	// Copy the value
	buf := bytes.NewBuffer(nil)
	if _, err := io.Copy(buf, req.Body); err != nil {
		return nil, err
	}
	applyReq.DirEnt.Value = buf.Bytes()

	// Make the RPC
	var out bool
	if err := s.agent.RPC(req.Context(), "KVS.Apply", &applyReq, &out); err != nil {
		return nil, err
	}

	// Only use the out value if this was a CAS
	if applyReq.Op == api.KVSet {
		return true, nil
	}
	return out, nil
}

// KVSPut handles a DELETE request
func (s *HTTPHandlers) KVSDelete(resp http.ResponseWriter, req *http.Request, args *structs.KeyRequest) (interface{}, error) {
	if err := s.parseEntMetaNoWildcard(req, &args.EnterpriseMeta); err != nil {
		return nil, err
	}
	if conflictingFlags(resp, req, "recurse", "cas") {
		return nil, nil
	}
	applyReq := structs.KVSRequest{
		Datacenter: args.Datacenter,
		Op:         api.KVDelete,
		DirEnt: structs.DirEntry{
			Key:            args.Key,
			EnterpriseMeta: args.EnterpriseMeta,
		},
	}
	applyReq.Token = args.Token

	// Check for recurse
	params := req.URL.Query()
	if _, ok := params["recurse"]; ok {
		applyReq.Op = api.KVDeleteTree
	} else if args.Key == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Missing key name"}
	}

	// Check for cas value
	if _, ok := params["cas"]; ok {
		casVal, err := strconv.ParseUint(params.Get("cas"), 10, 64)
		if err != nil {
			return nil, err
		}
		applyReq.DirEnt.ModifyIndex = casVal
		applyReq.Op = api.KVDeleteCAS
	}

	// Make the RPC
	var out bool
	if err := s.agent.RPC(req.Context(), "KVS.Apply", &applyReq, &out); err != nil {
		return nil, err
	}

	// Only use the out value if this was a CAS
	if applyReq.Op == api.KVDeleteCAS {
		return out, nil
	}
	return true, nil
}

// conflictingFlags determines if non-composable flags were passed in a request.
func conflictingFlags(resp http.ResponseWriter, req *http.Request, flags ...string) bool {
	params := req.URL.Query()

	found := false
	for _, conflict := range flags {
		if _, ok := params[conflict]; ok {
			if found {
				resp.WriteHeader(http.StatusBadRequest)
				fmt.Fprint(resp, "Conflicting flags: "+params.Encode())
				return true
			}
			found = true
		}
	}

	return false
}
