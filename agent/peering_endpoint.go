package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// PeeringEndpoint handles GET, DELETE on v1/peering/name
func (s *HTTPHandlers) PeeringEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name, err := getPathSuffixUnescaped(req.URL.Path, "/v1/peering/")
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Must specify a name to fetch."}
	}

	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	// Switch on the method
	switch req.Method {
	case "GET":
		return s.peeringRead(resp, req, name, entMeta.PartitionOrEmpty())
	case "DELETE":
		return s.peeringDelete(resp, req, name, entMeta.PartitionOrEmpty())
	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "DELETE"}}
	}
}

// peeringRead fetches a peering that matches the name and partition.
// This assumes that the name and partition parameters are valid
func (s *HTTPHandlers) peeringRead(resp http.ResponseWriter, req *http.Request, name, partition string) (interface{}, error) {
	args := pbpeering.PeeringReadRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
		Partition:  partition, // should be "" in OSS
	}

	result, err := s.agent.rpcClientPeering.PeeringRead(req.Context(), &args)
	if err != nil {
		return nil, err
	}
	if result.Peering == nil {
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: fmt.Sprintf("Peering not found for %q", name)}
	}

	// TODO(peering): replace with API types
	return result.Peering, nil
}

// PeeringList fetches all peerings in the datacenter in OSS or in a given partition in Consul Enterprise.
func (s *HTTPHandlers) PeeringList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	args := pbpeering.PeeringListRequest{
		Datacenter: s.agent.config.Datacenter,
		Partition:  entMeta.PartitionOrEmpty(), // should be "" in OSS
	}

	pbresp, err := s.agent.rpcClientPeering.PeeringList(req.Context(), &args)
	if err != nil {
		return nil, err
	}
	return pbresp.Peerings, nil
}

// PeeringGenerateToken handles POSTs to the /v1/peering/token endpoint. The request
// will always be forwarded via RPC to the local leader.
func (s *HTTPHandlers) PeeringGenerateToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := pbpeering.GenerateTokenRequest{
		Datacenter: s.agent.config.Datacenter,
	}

	if req.Body == nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The peering arguments must be provided in the body"}
	}

	if err := lib.DecodeJSON(req.Body, &args); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	if args.PeerName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeerName is required in the payload when generating a new peering token."}
	}

	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	if args.Partition == "" {
		args.Partition = entMeta.PartitionOrEmpty()
	}

	return s.agent.rpcClientPeering.GenerateToken(req.Context(), &args)
}

// PeeringInitiate handles POSTs to the /v1/peering/initiate endpoint. The request
// will always be forwarded via RPC to the local leader.
func (s *HTTPHandlers) PeeringInitiate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := pbpeering.InitiateRequest{
		Datacenter: s.agent.config.Datacenter,
	}

	if req.Body == nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The peering arguments must be provided in the body"}
	}

	if err := lib.DecodeJSON(req.Body, &args); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	if args.PeerName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeerName is required in the payload when initiating a peering."}
	}

	if args.PeeringToken == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeeringToken is required in the payload when initiating a peering."}
	}

	return s.agent.rpcClientPeering.Initiate(req.Context(), &args)
}

// peeringDelete initiates a deletion for a peering that matches the name and partition.
// This assumes that the name and partition parameters are valid.
func (s *HTTPHandlers) peeringDelete(resp http.ResponseWriter, req *http.Request, name, partition string) (interface{}, error) {
	args := pbpeering.PeeringDeleteRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
		Partition:  partition, // should be "" in OSS
	}

	result, err := s.agent.rpcClientPeering.PeeringDelete(req.Context(), &args)
	if err != nil {
		return nil, err
	}

	// TODO(peering) -- today pbpeering.PeeringDeleteResponse is a {} so the result below is actually {}
	return result, nil
}
