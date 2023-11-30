// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

const (
	// maxTxnOps is used to set an upper limit on the number of operations
	// inside a transaction. If there are more operations than this, then the
	// client is likely abusing transactions.
	maxTxnOps = 128
)

// decodeValue decodes the value member of the given operation.
func decodeValue(rawKV interface{}) error {
	rawMap, ok := rawKV.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected raw KV type: %T", rawKV)
	}
	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "value":
			// Leave the byte slice nil if we have a nil
			// value.
			if v == nil {
				return nil
			}

			// Otherwise, base64 decode it.
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("unexpected value type: %T", v)
			}
			decoded, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("failed to decode value: %v", err)
			}
			rawMap[k] = decoded
			return nil
		}
	}
	return nil
}

// isWrite returns true if the given operation alters the state store.
func isWrite(op api.KVOp) bool {
	switch op {
	case api.KVSet, api.KVDelete, api.KVDeleteCAS, api.KVDeleteTree, api.KVCAS, api.KVLock, api.KVUnlock:
		return true
	}
	return false
}

// convertOps takes the incoming body in API format and converts it to the
// internal RPC format. This returns a count of the number of write ops, and
// a boolean, that if false means an error response has been generated and
// processing should stop.
func (s *HTTPHandlers) convertOps(resp http.ResponseWriter, req *http.Request) (structs.TxnOps, int, error) {
	// The TxnMaxReqLen limit and KVMaxValueSize limit both default to the
	// suggested raft data size and can be configured independently. The
	// TxnMaxReqLen is enforced on the cumulative size of the transaction,
	// whereas the KVMaxValueSize limit is imposed on the values of individual KV
	// operations -- this is to keep consistent with the behavior for KV values
	// in the kvs endpoint.
	//
	// The defaults are set to the suggested raft size to keep the total
	// transaction size reasonable to account for timely heartbeat signals. If
	// the TxnMaxReqLen limit is above the raft's suggested threshold, large
	// transactions are automatically set to attempt a chunking apply.
	// Performance may degrade and warning messages may appear.
	maxTxnLen := int64(s.agent.config.TxnMaxReqLen)
	kvMaxValueSize := int64(s.agent.config.KVMaxValueSize)

	// For backward compatibility, KVMaxValueSize is used as the max txn request
	// length if it is configured greater than TxnMaxReqLen or its default
	if maxTxnLen < kvMaxValueSize {
		maxTxnLen = kvMaxValueSize
	}

	// Check Content-Length first before decoding to return early
	if req.ContentLength > maxTxnLen {
		return nil, 0, HTTPError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Reason: fmt.Sprintf("Request body(%d bytes) too large, max size: %d bytes. See %s.",
				req.ContentLength, maxTxnLen, "https://www.consul.io/docs/agent/config/config-files#txn_max_req_len"),
		}
	}

	var ops api.TxnOps
	req.Body = http.MaxBytesReader(resp, req.Body, maxTxnLen)
	if err := decodeBody(req.Body, &ops); err != nil {
		if err.Error() == "http: request body too large" {
			// The request size is also verified during decoding to double check
			// if the Content-Length header was not set by the client.
			return nil, 0, HTTPError{
				StatusCode: http.StatusRequestEntityTooLarge,
				Reason: fmt.Sprintf("Request body too large, max size: %d bytes. See %s.",
					maxTxnLen, "https://www.consul.io/docs/agent/config/config-files#txn_max_req_len"),
			}
		} else {
			// Note the body is in API format, and not the RPC format. If we can't
			// decode it, we will return a 400 since we don't have enough context to
			// associate the error with a given operation.
			return nil, 0, HTTPError{StatusCode: http.StatusBadRequest, Reason: fmt.Sprintf("Failed to parse body: %v", err)}
		}
	}

	// Enforce a reasonable upper limit on the number of operations in a
	// transaction in order to curb abuse.
	if size := len(ops); size > maxTxnOps {
		return nil, 0, HTTPError{
			StatusCode: http.StatusRequestEntityTooLarge,
			Reason:     fmt.Sprintf("Transaction contains too many operations (%d > %d)", size, maxTxnOps),
		}
	}

	// Convert the KV API format into the RPC format. Note that fixupKVOps
	// above will have already converted the base64 encoded strings into
	// byte arrays so we can assign right over.
	var opsRPC structs.TxnOps
	var writes int
	for _, in := range ops {
		switch {
		case in.KV != nil:
			size := len(in.KV.Value)
			if int64(size) > kvMaxValueSize {
				return nil, 0, HTTPError{
					StatusCode: http.StatusRequestEntityTooLarge,
					Reason:     fmt.Sprintf("Value for key %q is too large (%d > %d bytes)", in.KV.Key, size, s.agent.config.KVMaxValueSize),
				}
			}

			verb := in.KV.Verb
			if isWrite(verb) {
				writes++
			}

			out := &structs.TxnOp{
				KV: &structs.TxnKVOp{
					Verb: verb,
					DirEnt: structs.DirEntry{
						Key:     in.KV.Key,
						Value:   in.KV.Value,
						Flags:   in.KV.Flags,
						Session: in.KV.Session,
						EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(
							in.KV.Partition,
							in.KV.Namespace,
						),
						RaftIndex: structs.RaftIndex{
							ModifyIndex: in.KV.Index,
						},
					},
				},
			}
			opsRPC = append(opsRPC, out)

		case in.Node != nil:
			if in.Node.Verb != api.NodeGet {
				writes++
			}

			// Setup the default DC if not provided
			if in.Node.Node.Datacenter == "" {
				in.Node.Node.Datacenter = s.agent.config.Datacenter
			}

			node := in.Node.Node
			out := &structs.TxnOp{
				Node: &structs.TxnNodeOp{
					Verb: in.Node.Verb,
					Node: structs.Node{
						ID:              types.NodeID(node.ID),
						Node:            node.Node,
						Partition:       node.Partition,
						Address:         node.Address,
						Datacenter:      node.Datacenter,
						TaggedAddresses: node.TaggedAddresses,
						PeerName:        node.PeerName,
						Meta:            node.Meta,
						RaftIndex: structs.RaftIndex{
							ModifyIndex: node.ModifyIndex,
						},
					},
				},
			}
			opsRPC = append(opsRPC, out)

		case in.Service != nil:
			if in.Service.Verb != api.ServiceGet {
				writes++
			}

			svc := in.Service.Service
			out := &structs.TxnOp{
				Service: &structs.TxnServiceOp{
					Verb: in.Service.Verb,
					Node: in.Service.Node,
					Service: structs.NodeService{
						ID:      svc.ID,
						Service: svc.Service,
						Kind:    structs.ServiceKind(svc.Kind),
						Tags:    svc.Tags,
						Address: svc.Address,
						Meta:    svc.Meta,
						Port:    svc.Port,
						Weights: &structs.Weights{
							Passing: svc.Weights.Passing,
							Warning: svc.Weights.Warning,
						},
						EnableTagOverride: svc.EnableTagOverride,
						EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(
							svc.Partition,
							svc.Namespace,
						),
						RaftIndex: structs.RaftIndex{
							ModifyIndex: svc.ModifyIndex,
						},
					},
				},
			}

			if svc.Proxy != nil {
				out.Service.Service.Proxy = structs.ConnectProxyConfig{}
				t := &out.Service.Service.Proxy
				if svc.Proxy.DestinationServiceName != "" {
					t.DestinationServiceName = svc.Proxy.DestinationServiceName
				}
				if svc.Proxy.DestinationServiceID != "" {
					t.DestinationServiceID = svc.Proxy.DestinationServiceID
				}
				if svc.Proxy.LocalServiceAddress != "" {
					t.LocalServiceAddress = svc.Proxy.LocalServiceAddress
				}
				if svc.Proxy.LocalServicePort != 0 {
					t.LocalServicePort = svc.Proxy.LocalServicePort
				}
				if svc.Proxy.LocalServiceSocketPath != "" {
					t.LocalServiceSocketPath = svc.Proxy.LocalServiceSocketPath
				}
				if svc.Proxy.MeshGateway.Mode != "" {
					t.MeshGateway.Mode = structs.MeshGatewayMode(svc.Proxy.MeshGateway.Mode)
				}

				if svc.Proxy.TransparentProxy != nil {
					if svc.Proxy.TransparentProxy.DialedDirectly {
						t.TransparentProxy.DialedDirectly = svc.Proxy.TransparentProxy.DialedDirectly
					}

					if svc.Proxy.TransparentProxy.OutboundListenerPort != 0 {
						t.TransparentProxy.OutboundListenerPort = svc.Proxy.TransparentProxy.OutboundListenerPort
					}
				}
			}
			opsRPC = append(opsRPC, out)

		case in.Check != nil:
			if in.Check.Verb != api.CheckGet {
				writes++
			}

			check := in.Check.Check

			// Check if the internal duration fields are set as well as the normal ones. This is
			// to be backwards compatible with a bug where the internal duration fields were being
			// deserialized from instead of the correct fields.
			// See https://github.com/hashicorp/consul/issues/5477 for more details.
			interval := check.Definition.IntervalDuration
			if dur := time.Duration(check.Definition.Interval); dur != 0 {
				interval = dur
			}
			timeout := check.Definition.TimeoutDuration
			if dur := time.Duration(check.Definition.Timeout); dur != 0 {
				timeout = dur
			}
			deregisterCriticalServiceAfter := check.Definition.DeregisterCriticalServiceAfterDuration
			if dur := time.Duration(check.Definition.DeregisterCriticalServiceAfter); dur != 0 {
				deregisterCriticalServiceAfter = dur
			}

			out := &structs.TxnOp{
				Check: &structs.TxnCheckOp{
					Verb: in.Check.Verb,
					Check: structs.HealthCheck{
						Node:        check.Node,
						CheckID:     types.CheckID(check.CheckID),
						Name:        check.Name,
						Status:      check.Status,
						Notes:       check.Notes,
						Output:      check.Output,
						ServiceID:   check.ServiceID,
						ServiceName: check.ServiceName,
						ServiceTags: check.ServiceTags,
						PeerName:    check.PeerName,
						ExposedPort: check.ExposedPort,
						Definition: structs.HealthCheckDefinition{
							HTTP:                           check.Definition.HTTP,
							TLSServerName:                  check.Definition.TLSServerName,
							TLSSkipVerify:                  check.Definition.TLSSkipVerify,
							Header:                         check.Definition.Header,
							Method:                         check.Definition.Method,
							Body:                           check.Definition.Body,
							TCP:                            check.Definition.TCP,
							GRPC:                           check.Definition.GRPC,
							GRPCUseTLS:                     check.Definition.GRPCUseTLS,
							OSService:                      check.Definition.OSService,
							Interval:                       interval,
							Timeout:                        timeout,
							DeregisterCriticalServiceAfter: deregisterCriticalServiceAfter,
						},
						EnterpriseMeta: acl.NewEnterpriseMetaWithPartition(
							check.Partition,
							check.Namespace,
						),
						RaftIndex: structs.RaftIndex{
							ModifyIndex: check.ModifyIndex,
						},
					},
				},
			}
			opsRPC = append(opsRPC, out)
		}
	}

	return opsRPC, writes, nil
}

// Txn handles requests to apply multiple operations in a single, atomic
// transaction. A transaction consisting of only read operations will be fast-
// pathed to an endpoint that supports consistency modes (but not blocking),
// and everything else will be routed through Raft like a normal write.
func (s *HTTPHandlers) Txn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Convert the ops from the API format to the internal format.
	ops, writes, err := s.convertOps(resp, req)
	if err != nil {
		return nil, err
	}

	// Fast-path a transaction with only writes to the read-only endpoint,
	// which bypasses Raft, and allows for staleness.
	conflict := false
	var ret interface{}
	if writes == 0 {
		args := structs.TxnReadRequest{Ops: ops}
		if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
			return nil, nil
		}

		var reply structs.TxnReadResponse
		if err := s.agent.RPC(req.Context(), "Txn.Read", &args, &reply); err != nil {
			return nil, err
		}

		// Since we don't do blocking, we only add the relevant headers
		// for metadata.
		setLastContact(resp, reply.LastContact)
		setKnownLeader(resp, reply.KnownLeader)

		ret, conflict = reply, len(reply.Errors) > 0
	} else {
		args := structs.TxnRequest{Ops: ops}
		s.parseDC(req, &args.Datacenter)
		s.parseToken(req, &args.Token)

		var reply structs.TxnResponse
		if err := s.agent.RPC(req.Context(), "Txn.Apply", &args, &reply); err != nil {
			return nil, err
		}
		ret, conflict = reply, len(reply.Errors) > 0
	}

	// If there was a conflict return the response object but set a special
	// status code.
	if conflict {
		var buf []byte
		var err error
		buf, err = s.marshalJSON(req, ret)
		if err != nil {
			return nil, err
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusConflict)
		resp.Write(buf)
		return nil, nil
	}

	// Otherwise, return the results of the successful transaction.
	return ret, nil
}
