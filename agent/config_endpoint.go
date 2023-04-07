package agent

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
)

const ConfigEntryNotFoundErr string = "Config entry not found"

// Config switches on the different CRUD operations for config entries.
func (s *HTTPHandlers) Config(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
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
func (s *HTTPHandlers) configGet(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ConfigEntryQuery
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}
	kindAndName := strings.TrimPrefix(req.URL.Path, "/v1/config/")
	pathArgs := strings.SplitN(kindAndName, "/", 2)

	switch len(pathArgs) {
	case 2:
		// Both kind/name provided.
		args.Kind = pathArgs[0]
		args.Name = pathArgs[1]

		if err := s.parseEntMetaForConfigEntryKind(args.Kind, req, &args.EnterpriseMeta); err != nil {
			return nil, err
		}

		var reply structs.ConfigEntryResponse
		if err := s.agent.RPC(req.Context(), "ConfigEntry.Get", &args, &reply); err != nil {
			return nil, err
		}
		setMeta(resp, &reply.QueryMeta)

		if reply.Entry == nil {
			return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: fmt.Sprintf("%s for %q / %q", ConfigEntryNotFoundErr, pathArgs[0], pathArgs[1])}
		}

		return reply.Entry, nil
	case 1:
		if err := s.parseEntMeta(req, &args.EnterpriseMeta); err != nil {
			return nil, err
		}
		// Only kind provided, list entries.
		args.Kind = pathArgs[0]

		var reply structs.IndexedConfigEntries
		if err := s.agent.RPC(req.Context(), "ConfigEntry.List", &args, &reply); err != nil {
			return nil, err
		}
		setMeta(resp, &reply.QueryMeta)

		return reply.Entries, nil
	default:
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: "Must provide either a kind or both kind and name"}
	}
}

// configDelete deletes the given config entry.
func (s *HTTPHandlers) configDelete(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var args structs.ConfigEntryRequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)
	kindAndName := strings.TrimPrefix(req.URL.Path, "/v1/config/")
	pathArgs := strings.SplitN(kindAndName, "/", 2)

	if len(pathArgs) != 2 {
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: "Must provide both a kind and name to delete"}
	}

	entry, err := structs.MakeConfigEntry(pathArgs[0], pathArgs[1])
	if err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: err.Error()}
	}
	args.Entry = entry
	// Parse enterprise meta.
	meta := args.Entry.GetEnterpriseMeta()

	if err := s.parseEntMetaForConfigEntryKind(entry.GetKind(), req, meta); err != nil {
		return nil, err
	}

	// Check for cas value
	if casStr := req.URL.Query().Get("cas"); casStr != "" {
		casVal, err := strconv.ParseUint(casStr, 10, 64)
		if err != nil {
			return nil, err
		}
		args.Op = structs.ConfigEntryDeleteCAS
		args.Entry.GetRaftIndex().ModifyIndex = casVal
	}

	var reply structs.ConfigEntryDeleteResponse
	if err := s.agent.RPC(req.Context(), "ConfigEntry.Delete", &args, &reply); err != nil {
		return nil, err
	}

	// Return the `deleted` boolean for CAS operations, but not normal deletions
	// to maintain backwards-compatibility with existing callers.
	if args.Op == structs.ConfigEntryDeleteCAS {
		return reply.Deleted, nil
	}
	return struct{}{}, nil
}

// ConfigApply applies the given config entry update.
func (s *HTTPHandlers) ConfigApply(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.ConfigEntryRequest{
		Op: structs.ConfigEntryUpsert,
	}
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	var raw map[string]interface{}
	if err := decodeBodyDeprecated(req, &raw, nil); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decoding failed: %v", err)}
	}

	if entry, err := structs.DecodeConfigEntry(raw); err == nil {
		args.Entry = entry
	} else {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Request decoding failed: %v", err)}
	}

	// Parse enterprise meta.
	var meta acl.EnterpriseMeta
	if err := s.parseEntMetaForConfigEntryKind(args.Entry.GetKind(), req, &meta); err != nil {
		return nil, err
	}
	args.Entry.GetEnterpriseMeta().Merge(&meta)

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
	if err := s.agent.RPC(req.Context(), "ConfigEntry.Apply", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

func (s *HTTPHandlers) parseEntMetaForConfigEntryKind(kind string, req *http.Request, entMeta *acl.EnterpriseMeta) error {
	if kind == structs.ServiceIntentions {
		return s.parseEntMeta(req, entMeta)
	}
	return s.parseEntMetaNoWildcard(req, entMeta)
}
