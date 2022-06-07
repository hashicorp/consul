package agent

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// PeeringEndpoint handles GET, DELETE on v1/peering/name
func (s *HTTPHandlers) PeeringEndpoint(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name := strings.TrimPrefix(req.URL.Path, "/v1/peering/")
	if name == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "Must specify a name to fetch."}
	}

	// Switch on the method
	switch req.Method {
	case "GET":
		return s.peeringRead(resp, req, name)
	case "DELETE":
		return s.peeringDelete(resp, req, name)
	default:
		return nil, MethodNotAllowedError{req.Method, []string{"GET", "DELETE"}}
	}
}

// peeringRead fetches a peering that matches the name and partition.
// This assumes that the name and partition parameters are valid
func (s *HTTPHandlers) peeringRead(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	args := pbpeering.PeeringReadRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
	}
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args.Partition = entMeta.PartitionOrEmpty()

	result, err := s.agent.rpcClientPeering.PeeringRead(req.Context(), &args)
	if err != nil {
		return nil, err
	}
	if result.Peering == nil {
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: fmt.Sprintf("Peering not found for %q", name)}
	}

	return result.Peering.ToAPI(), nil
}

// PeeringList fetches all peerings in the datacenter in OSS or in a given partition in Consul Enterprise.
func (s *HTTPHandlers) PeeringList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := pbpeering.PeeringListRequest{
		Datacenter: s.agent.config.Datacenter,
	}
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args.Partition = entMeta.PartitionOrEmpty()

	pbresp, err := s.agent.rpcClientPeering.PeeringList(req.Context(), &args)
	if err != nil {
		return nil, err
	}

	return pbresp.ToAPI(), nil
}

// PeeringGenerateToken handles POSTs to the /v1/peering/token endpoint. The request
// will always be forwarded via RPC to the local leader.
func (s *HTTPHandlers) PeeringGenerateToken(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Body == nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The peering arguments must be provided in the body"}
	}

	apiRequest := &api.PeeringGenerateTokenRequest{
		Datacenter: s.agent.config.Datacenter,
	}
	if err := lib.DecodeJSON(req.Body, apiRequest); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}
	args := pbpeering.NewGenerateTokenRequestFromAPI(apiRequest)

	if args.PeerName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeerName is required in the payload when generating a new peering token."}
	}

	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	if args.Partition == "" {
		args.Partition = entMeta.PartitionOrEmpty()
	}

	out, err := s.agent.rpcClientPeering.GenerateToken(req.Context(), args)
	if err != nil {
		return nil, err
	}

	return out.ToAPI(), nil
}

// PeeringInitiate handles POSTs to the /v1/peering/initiate endpoint. The request
// will always be forwarded via RPC to the local leader.
func (s *HTTPHandlers) PeeringInitiate(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Body == nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The peering arguments must be provided in the body"}
	}

	apiRequest := &api.PeeringInitiateRequest{
		Datacenter: s.agent.config.Datacenter,
	}
	if err := lib.DecodeJSON(req.Body, apiRequest); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}
	args := pbpeering.NewInitiateRequestFromAPI(apiRequest)

	if args.PeerName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeerName is required in the payload when initiating a peering."}
	}

	if args.PeeringToken == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeeringToken is required in the payload when initiating a peering."}
	}

	out, err := s.agent.rpcClientPeering.Initiate(req.Context(), args)
	if err != nil {
		return nil, err
	}

	return out.ToAPI(), nil
}

// peeringDelete initiates a deletion for a peering that matches the name and partition.
// This assumes that the name and partition parameters are valid.
func (s *HTTPHandlers) peeringDelete(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	args := pbpeering.PeeringDeleteRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
	}
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args.Partition = entMeta.PartitionOrEmpty()

	_, err := s.agent.rpcClientPeering.PeeringDelete(req.Context(), &args)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
