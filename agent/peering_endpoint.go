package agent

import (
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/hashicorp/consul/acl"
	external "github.com/hashicorp/consul/agent/grpc-external"
	"github.com/hashicorp/consul/agent/structs"
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
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args := pbpeering.PeeringReadRequest{
		Name:      name,
		Partition: entMeta.PartitionOrEmpty(),
	}

	var dc string
	options := structs.QueryOptions{}
	s.parse(resp, req, &dc, &options)
	options.AllowStale = false // To get all information on a peering, this request must be forward to a leader
	ctx, err := external.ContextWithQueryOptions(req.Context(), options)
	if err != nil {
		return nil, err
	}

	var header metadata.MD
	result, err := s.agent.rpcClientPeering.PeeringRead(ctx, &args, grpc.Header(&header))
	if err != nil {
		return nil, err
	}
	if result.Peering == nil {
		return nil, HTTPError{StatusCode: http.StatusNotFound, Reason: fmt.Sprintf("Peering not found for %q", name)}
	}

	meta, err := external.QueryMetaFromGRPCMeta(header)
	if err != nil {
		return result.Peering.ToAPI(), fmt.Errorf("could not convert gRPC metadata to query meta: %w", err)
	}
	if err := setMeta(resp, &meta); err != nil {
		return nil, err
	}

	return result.Peering.ToAPI(), nil
}

// PeeringList fetches all peerings in the datacenter in CE or in a given partition in Consul Enterprise.
func (s *HTTPHandlers) PeeringList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args := pbpeering.PeeringListRequest{
		Partition: entMeta.PartitionOrEmpty(),
	}

	var dc string
	options := structs.QueryOptions{}
	s.parse(resp, req, &dc, &options)
	options.AllowStale = false // To get all information on a peering, this request must be forward to a leader
	ctx, err := external.ContextWithQueryOptions(req.Context(), options)
	if err != nil {
		return nil, err
	}

	var header metadata.MD
	pbresp, err := s.agent.rpcClientPeering.PeeringList(ctx, &args, grpc.Header(&header))
	if err != nil {
		return nil, err
	}

	meta, err := external.QueryMetaFromGRPCMeta(header)
	if err != nil {
		return pbresp.ToAPI(), fmt.Errorf("could not convert gRPC metadata to query meta: %w", err)
	}
	if err := setMeta(resp, &meta); err != nil {
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

	var apiRequest api.PeeringGenerateTokenRequest
	if err := lib.DecodeJSON(req.Body, &apiRequest); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	args := pbpeering.NewGenerateTokenRequestFromAPI(&apiRequest)
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

	var token string
	s.parseToken(req, &token)
	options := structs.QueryOptions{Token: token}
	ctx, err := external.ContextWithQueryOptions(req.Context(), options)
	if err != nil {
		return nil, err
	}

	out, err := s.agent.rpcClientPeering.GenerateToken(ctx, args)
	if err != nil {
		return nil, err
	}

	return out.ToAPI(), nil
}

// PeeringEstablish handles POSTs to the /v1/peering/establish endpoint. The request
// will always be forwarded via RPC to the local leader.
func (s *HTTPHandlers) PeeringEstablish(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Body == nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "The peering arguments must be provided in the body"}
	}

	var apiRequest api.PeeringEstablishRequest
	if err := lib.DecodeJSON(req.Body, &apiRequest); err != nil {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Body decoding failed: %v", err)}
	}

	args := pbpeering.NewEstablishRequestFromAPI(&apiRequest)
	if args.PeerName == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeerName is required in the payload when establishing a peering."}
	}
	if args.PeeringToken == "" {
		return nil, HTTPError{StatusCode: http.StatusBadRequest, Reason: "PeeringToken is required in the payload when establishing a peering."}
	}

	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	if args.Partition == "" {
		args.Partition = entMeta.PartitionOrEmpty()
	}

	var token string
	s.parseToken(req, &token)
	options := structs.QueryOptions{Token: token}
	ctx, err := external.ContextWithQueryOptions(req.Context(), options)
	if err != nil {
		return nil, err
	}

	out, err := s.agent.rpcClientPeering.Establish(ctx, args)
	if err != nil {
		return nil, err
	}

	return out.ToAPI(), nil
}

// peeringDelete initiates a deletion for a peering that matches the name and partition.
// This assumes that the name and partition parameters are valid.
func (s *HTTPHandlers) peeringDelete(resp http.ResponseWriter, req *http.Request, name string) (interface{}, error) {
	var entMeta acl.EnterpriseMeta
	if err := s.parseEntMetaPartition(req, &entMeta); err != nil {
		return nil, err
	}
	args := pbpeering.PeeringDeleteRequest{
		Name:      name,
		Partition: entMeta.PartitionOrEmpty(),
	}

	var token string
	s.parseToken(req, &token)
	options := structs.QueryOptions{Token: token}
	ctx, err := external.ContextWithQueryOptions(req.Context(), options)
	if err != nil {
		return nil, err
	}

	_, err = s.agent.rpcClientPeering.PeeringDelete(ctx, &args)
	if err != nil {
		return nil, err
	}

	return nil, nil
}
