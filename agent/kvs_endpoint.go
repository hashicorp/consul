// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
)

// Compiled regex patterns for KV key validation
// These patterns provide efficient, comprehensive security validation for KV keys
// and URL paths to prevent various attack vectors including path traversal,
// file extension abuse, and endpoint forwarding attacks.
var (
	// validKVKeyPattern validates allowed characters in KV keys
	// Allows: alphanumeric (a-zA-Z0-9), comma (,), dash (-), underscore (_), dot (.), forward slash (/)
	// This pattern enables hierarchical keys while blocking dangerous characters that could enable attacks
	validKVKeyPattern = regexp.MustCompile(`^[a-zA-Z0-9,\-_./]+$`)

	// pathTraversalPattern detects path traversal sequences in various forms
	// Matches: ../, ..\, .. at start/end of string
	// This prevents directory traversal attacks that could access parent directories
	pathTraversalPattern = regexp.MustCompile(`(\.\.[\\/])|(\.\.\\)|(^\.\.)|(\.\.$)`)

	// encodedTraversalPattern detects URL-encoded path traversal attempts (case-insensitive)
	// Matches various encodings of "../" including:
	// - %2e%2e%2f (../), %2e%2e%5c (..\), %2e%2e/ (partial encoding)
	// - ..%2f, ..%5c (mixed encoding), %252e%252e%252f (double encoding)
	// This prevents bypassing path traversal detection through URL encoding
	encodedTraversalPattern = regexp.MustCompile(`(?i)(%2e%2e(%2f|%5c|/))|(\.\.\%2f)|(\.\.\%5c)|(%252e%252e%252f)`)

	// suspiciousExtensionPattern detects file extensions that could be cached or exploited
	// Matches common web file types, executables, documents, and media files (case-insensitive)
	// Includes detection of extensions with query parameters (e.g., script.js?v=1)
	// This prevents cache deception attacks and reduces attack surface
	suspiciousExtensionPattern = regexp.MustCompile(`(?i)\.(js|css|html?|xml|json|txt|log|php|asp|jsp|cgi|pl|py|rb|sh|exe|dll|bat|cmd|com|scr|vbs|pdf|docx?|xlsx?|pptx?|zip|rar|tar|gz|7z|bz2|jpe?g|png|gif|svg|ico|bmp|mp[34]|avi|mov|wmv|flv|swf|woff2?|ttf|eot|otf)(\?|$)`)

	// leadingSlashPattern detects keys that start with a forward slash
	// Consistent with client-side validation and Consul conventions
	leadingSlashPattern = regexp.MustCompile(`^/`)

	// rawPathTraversalPattern detects traversal patterns in raw URL paths for early validation
	// Combines path traversal and encoded traversal detection for comprehensive URL path validation
	// This prevents path traversal attacks before URL normalization occurs
	rawPathTraversalPattern = regexp.MustCompile(`(\.\.[\\/])|(?i)(%2e%2e(%2f|%5c|/))|(\.\.\%2f)|(\.\.\%5c)|(%252e%252e%252f)`)
)

// validateKVKey validates a KV key to prevent path traversal attacks and cache deception
// Allows hierarchical keys with / but restricts dangerous characters for security
// If allowUnprintable is true, character validation is skipped (respects disable_http_unprintable_char_filter)
func validateKVKey(key string, allowUnprintable bool) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}

	// Check for leading slash (consistent with client-side validation)
	if leadingSlashPattern.MatchString(key) {
		return fmt.Errorf("key must not begin with a '/'")
	}

	// SECURITY: Allow alphanumeric, safe punctuation, and hierarchical separators
	// Allowed: a-zA-Z0-9 ,-_./ (includes slash for hierarchical keys)
	// Blocked: dangerous chars that could enable attacks: ?=&#<>[]{}()@!$%^*+|\\:;"'`~
	// Skip character validation if allowUnprintable is true (for disable_http_unprintable_char_filter)
	if !allowUnprintable {
		if !validKVKeyPattern.MatchString(key) {
			return fmt.Errorf("key contains invalid characters. Only alphanumeric characters and ,-_./ are allowed")
		}
	}

	// Check for path traversal patterns using regex
	if pathTraversalPattern.MatchString(key) {
		return fmt.Errorf("key contains path traversal sequence")
	}

	// Check for URL-encoded traversal attempts using regex
	if encodedTraversalPattern.MatchString(key) {
		return fmt.Errorf("key contains encoded path traversal sequence")
	}

	// Check for suspicious file extensions using regex
	if suspiciousExtensionPattern.MatchString(key) {
		return fmt.Errorf("key contains suspicious file extension that may be cached by proxies or exploited")
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
	if rawPathTraversalPattern.MatchString(rawPath) {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "URL path contains invalid or encoded traversal sequence"}
	}

	// Pull out the key name, validation left to each sub-handler
	args.Key = strings.TrimPrefix(req.URL.Path, "/v1/kv/")

	// Validate the key for operations that use it (not for listing keys without a path)
	if args.Key != "" {
		allowUnprintable := s.agent.GetConfig().DisableHTTPUnprintableCharFilter
		if err := validateKVKey(args.Key, allowUnprintable); err != nil {
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
	allowUnprintable := s.agent.GetConfig().DisableHTTPUnprintableCharFilter
	if err := validateKVKey(args.Key, allowUnprintable); err != nil {
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
		allowUnprintable := s.agent.GetConfig().DisableHTTPUnprintableCharFilter
		if err := validateKVKey(args.Key, allowUnprintable); err != nil {
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
