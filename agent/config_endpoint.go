package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
)

// Config switches on the different CRUD operations for config entries.
func (s *HTTPServer) Config(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	switch req.Method {
	case "GET":
		return s.configGet(resp, req)

	case "DELETE":
		return s.configDelete(resp, req)

	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "DELETE"}}
	}
}

// configGet gets either a specific config entry, or lists all config entries
// of a kind if no name is provided.
func (s *HTTPServer) configGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ConfigEntryQuery
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	pathArgs := strings.SplitN(strings.TrimPrefix(req.URL.Path, "/v1/config/"), "/", 2)

	switch len(pathArgs) {
	case 2:
		// Both kind/name provided.
		args.Kind = pathArgs[0]
		args.Name = pathArgs[1]

		var reply structs.ConfigEntryResponse
		if err := s.agent.RPC("ConfigEntry.Get", &args, &reply); err != nil {
			return nil, err
		}

		if reply.Entry == nil {
			return nil, NotFoundError{Reason: fmt.Sprintf("Config entry not found for %q / %q", pathArgs[0], pathArgs[1])}
		}

		return reply.Entry, nil
	case 1:
		// Only kind provided, list entries.
		args.Kind = pathArgs[0]

		var reply structs.IndexedConfigEntries
		if err := s.agent.RPC("ConfigEntry.List", &args, &reply); err != nil {
			return nil, err
		}

		return reply.Entries, nil
	default:
		return nil, NotFoundError{Reason: "Must provide either a kind or both kind and name"}
	}
}

// configDelete deletes the given config entry.
func (s *HTTPServer) configDelete(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ConfigEntryRequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	pathArgs := strings.SplitN(strings.TrimPrefix(req.URL.Path, "/v1/config/"), "/", 2)

	if len(pathArgs) != 2 {
		resp.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(resp, "Must provide both a kind and name to delete")
		return nil, nil
	}

	entry, err := structs.MakeConfigEntry(pathArgs[0], pathArgs[1])
	if err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "%v", err)
		return nil, nil
	}
	args.Entry = entry

	var reply struct{}
	if err := s.agent.RPC("ConfigEntry.Delete", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// ConfigCreate applies the given config entry update.
func (s *HTTPServer) ConfigApply(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ConfigEntryRequest{
		Op: structs.ConfigEntryUpsert,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	var raw map[string]interface{}
	if err := decodeBody(req, &raw, nil); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
	}

	if entry, err := structs.DecodeConfigEntry(raw); err == nil {
		args.Entry = entry
	} else {
		return nil, BadRequestError{Reason: fmt.Sprintf("Request decoding failed: %v", err)}
	}

	// Check for cas value
	if casStr := req.URL.Query().Get("cas"); casStr != "" {
		casVal, err := strconv.ParseUint(casStr, 10, 64)
		if err != nil {
			return nil, err
		}
		args.Op = structs.ConfigEntryUpsertCAS
		args.Entry.GetRaftIndex().ModifyIndex = casVal
	}

	var reply bool
	if err := s.agent.RPC("ConfigEntry.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}
