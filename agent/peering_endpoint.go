package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/proto/pbpeering"
)

// PeeringRead fetches a peering that matches the request parameters.
func (s *HTTPHandlers) PeeringRead(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	name, err := getPathSuffixUnescaped(req.URL.Path, "/v1/peering/")
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, BadRequestError{Reason: "Must specify a name to fetch."}
	}

	entMeta := s.agent.AgentEnterpriseMeta()
	if err := s.parseEntMetaPartition(req, entMeta); err != nil {
		return nil, err
	}

	args := pbpeering.PeeringReadRequest{
		Name:       name,
		Datacenter: s.agent.config.Datacenter,
		Partition:  entMeta.PartitionOrEmpty(), // should be "" in OSS
	}

	result, err := s.agent.rpcClientPeering.PeeringRead(req.Context(), &args)
	if err != nil {
		return nil, err
	}
	if result.Peering == nil {
		return nil, NotFoundError{}
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
		return nil, BadRequestError{Reason: "The peering arguments must be provided in the body"}
	}

	if err := lib.DecodeJSON(req.Body, &args); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	if args.PeerName == "" {
		return nil, BadRequestError{Reason: "PeerName is required in the payload when generating a new peering token."}
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
		return nil, BadRequestError{Reason: "The peering arguments must be provided in the body"}
	}

	if err := lib.DecodeJSON(req.Body, &args); err != nil {
		return nil, BadRequestError{Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	if args.PeerName == "" {
		return nil, BadRequestError{Reason: "PeerName is required in the payload when initiating a peering."}
	}

	if args.PeeringToken == "" {
		return nil, BadRequestError{Reason: "PeeringToken is required in the payload when initiating a peering."}
	}

	return s.agent.rpcClientPeering.Initiate(req.Context(), &args)
}
