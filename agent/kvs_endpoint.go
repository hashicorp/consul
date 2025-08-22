// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// validateKVKey validates a KV key to prevent path traversal attacks and cache deception
func validateKVKey(key string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Check for leading slash (consistent with client-side validation)
	if strings.HasPrefix(key, "/") {
		return fmt.Errorf("key must not begin with a '/'")
	}

	// Check for path traversal sequences - both direct and after normalization
	if strings.Contains(key, "../") || strings.Contains(key, "..\\") {
		return fmt.Errorf("key contains invalid path traversal sequence")
	}

	// Check for various encoded path traversal attempts
	if strings.Contains(key, "%2e%2e%2f") || strings.Contains(key, "%2e%2e/") ||
		strings.Contains(key, "..%2f") || strings.Contains(key, "%2e%2e%5c") {
		return fmt.Errorf("key contains encoded path traversal sequence")
	}

	// Check for suspicious file extensions that could be used for cache deception
	// These extensions might be cached by CDNs/proxies and could lead to security issues
	suspiciousExtensions := []string{
		".js", ".css", ".html", ".htm", ".xml", ".json", ".txt", ".log",
		".php", ".asp", ".jsp", ".cgi", ".pl", ".py", ".rb", ".sh",
		".exe", ".dll", ".bat", ".cmd", ".com", ".scr", ".vbs",
		".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx",
		".zip", ".rar", ".tar", ".gz", ".7z", ".bz2",
		".jpg", ".jpeg", ".png", ".gif", ".svg", ".ico", ".bmp",
		".mp3", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".swf",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
	}

	// Check if key ends with any suspicious extension
	keyLower := strings.ToLower(key)
	for _, ext := range suspiciousExtensions {
		if strings.HasSuffix(keyLower, ext) {
			return fmt.Errorf("key contains suspicious file extension '%s' that may be cached by proxies", ext)
		}
	}

	// Also check for extensions with query parameters or fragments (e.g., "file.js?v=1")
	if strings.Contains(keyLower, ".js?") || strings.Contains(keyLower, ".css?") ||
		strings.Contains(keyLower, ".html?") || strings.Contains(keyLower, ".htm?") {
		return fmt.Errorf("key contains suspicious file extension with parameters that may be cached")
	}

	// Normalize the path and check if it's different from the original
	// This catches various path normalization attacks including "app/../admin"
	normalized := filepath.Clean(key)
	if normalized != key && (strings.Contains(normalized, "..") || strings.HasPrefix(normalized, "/")) {
		return fmt.Errorf("key path normalization results in invalid path")
	}

	// Additional check: ensure normalized path doesn't escape directory structure
	if strings.HasPrefix(normalized, "..") || strings.Contains(normalized, "/..") {
		return fmt.Errorf("key contains path traversal sequence after normalization")
	}

	return nil
}

func (s *HTTPHandlers) KVSEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Set default DC
	args := structs.KeyRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	// SECURITY: Validate raw URL path BEFORE normalization to prevent path traversal forwarding
	// This prevents requests like /v1/kv/../admin from being forwarded to /admin endpoint
	rawPath := req.URL.Path
	if strings.Contains(rawPath, "../") || strings.Contains(rawPath, "..\\") {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "URL path contains invalid traversal sequence"}
	}

	// Check for URL-encoded path traversal in raw path
	if strings.Contains(rawPath, "%2e%2e%2f") || strings.Contains(rawPath, "%2e%2e/") ||
		strings.Contains(rawPath, "..%2f") || strings.Contains(rawPath, "%2e%2e%5c") ||
		strings.Contains(rawPath, "%252e%252e%252f") {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "URL path contains encoded traversal sequence"}
	}

	// Pull out the key name, validation left to each sub-handler
	args.Key = strings.TrimPrefix(req.URL.Path, "/v1/kv/")

	// Validate the key for operations that use it (not for listing keys without a path)
	if args.Key != "" {
		if err := validateKVKey(args.Key); err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: err.Error()}
		}
	}

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

	// Additional validation for KV key (belt and suspenders approach)
	if err := validateKVKey(args.Key); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: err.Error()}
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
				req.ContentLength, s.agent.config.KVMaxValueSize, "https://developer.hashicorp.com/docs/agent/config/config-files#kv_max_value_size"),
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

	// Validate the key (additional safety check)
	if args.Key != "" {
		if err := validateKVKey(args.Key); err != nil {
			return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: err.Error()}
		}
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
				fmt.Fprintf(resp, "Conflicting flags: %v\n", params.Encode())
				return true
			}
			found = true
		}
	}

	return false
}
