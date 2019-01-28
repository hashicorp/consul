package agent

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/types"
)

const (
	// maxTxnOps is used to set an upper limit on the number of operations
	// inside a transaction. If there are more operations than this, then the
	// client is likely abusing transactions.
	maxTxnOps = 64
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

// fixupTxnOp looks for non-nil Txn operations and passes them on for
// value conversion.
func fixupTxnOp(rawOp interface{}) error {
	rawMap, ok := rawOp.(map[string]interface{})
	if !ok {
		return fmt.Errorf("unexpected raw op type: %T", rawOp)
	}
	for k, v := range rawMap {
		switch strings.ToLower(k) {
		case "kv":
			if v == nil {
				return nil
			}
			return decodeValue(v)
		}
	}
	return nil
}

// fixupTxnOps takes the raw decoded JSON and base64 decodes values in Txn ops,
// replacing them with byte arrays.
func fixupTxnOps(raw interface{}) error {
	rawSlice, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected raw type: %t", raw)
	}
	for _, rawOp := range rawSlice {
		if err := fixupTxnOp(rawOp); err != nil {
			return err
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
func (s *HTTPServer) convertOps(resp http.ResponseWriter, req *http.Request) (structs.TxnOps, int, bool) {
	// Note the body is in API format, and not the RPC format. If we can't
	// decode it, we will return a 400 since we don't have enough context to
	// associate the error with a given operation.
	var ops api.TxnOps
	if err := decodeBody(req, &ops, fixupTxnOps); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(resp, "Failed to parse body: %v", err)
		return nil, 0, false
	}

	// Enforce a reasonable upper limit on the number of operations in a
	// transaction in order to curb abuse.
	if size := len(ops); size > maxTxnOps {
		resp.WriteHeader(http.StatusRequestEntityTooLarge)
		fmt.Fprintf(resp, "Transaction contains too many operations (%d > %d)",
			size, maxTxnOps)

		return nil, 0, false
	}

	// Convert the KV API format into the RPC format. Note that fixupKVOps
	// above will have already converted the base64 encoded strings into
	// byte arrays so we can assign right over.
	var opsRPC structs.TxnOps
	var writes int
	var netKVSize int
	for _, in := range ops {
		switch {
		case in.KV != nil:
			size := len(in.KV.Value)
			if size > maxKVSize {
				resp.WriteHeader(http.StatusRequestEntityTooLarge)
				fmt.Fprintf(resp, "Value for key %q is too large (%d > %d bytes)", in.KV.Key, size, maxKVSize)
				return nil, 0, false
			}
			netKVSize += size

			verb := api.KVOp(in.KV.Verb)
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
						Address:         node.Address,
						Datacenter:      node.Datacenter,
						TaggedAddresses: node.TaggedAddresses,
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
						Tags:    svc.Tags,
						Address: svc.Address,
						Meta:    svc.Meta,
						Port:    svc.Port,
						Weights: &structs.Weights{
							Passing: svc.Weights.Passing,
							Warning: svc.Weights.Warning,
						},
						EnableTagOverride: svc.EnableTagOverride,
						RaftIndex: structs.RaftIndex{
							ModifyIndex: svc.ModifyIndex,
						},
					},
				},
			}
			opsRPC = append(opsRPC, out)

		case in.Check != nil:
			if in.Check.Verb != api.CheckGet {
				writes++
			}

			check := in.Check.Check
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
						Definition: structs.HealthCheckDefinition{
							HTTP:                           check.Definition.HTTP,
							TLSSkipVerify:                  check.Definition.TLSSkipVerify,
							Header:                         check.Definition.Header,
							Method:                         check.Definition.Method,
							TCP:                            check.Definition.TCP,
							Interval:                       check.Definition.IntervalDuration,
							Timeout:                        check.Definition.TimeoutDuration,
							DeregisterCriticalServiceAfter: check.Definition.DeregisterCriticalServiceAfterDuration,
						},
						RaftIndex: structs.RaftIndex{
							ModifyIndex: check.ModifyIndex,
						},
					},
				},
			}
			opsRPC = append(opsRPC, out)
		}
	}

	// Enforce an overall size limit to help prevent abuse.
	if netKVSize > maxKVSize {
		resp.WriteHeader(http.StatusRequestEntityTooLarge)
		fmt.Fprintf(resp, "Cumulative size of key data is too large (%d > %d bytes)",
			netKVSize, maxKVSize)

		return nil, 0, false
	}

	return opsRPC, writes, true
}

// Txn handles requests to apply multiple operations in a single, atomic
// transaction. A transaction consisting of only read operations will be fast-
// pathed to an endpoint that supports consistency modes (but not blocking),
// and everything else will be routed through Raft like a normal write.
func (s *HTTPServer) Txn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	// Convert the ops from the API format to the internal format.
	ops, writes, ok := s.convertOps(resp, req)
	if !ok {
		return nil, nil
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
		if err := s.agent.RPC("Txn.Read", &args, &reply); err != nil {
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
		if err := s.agent.RPC("Txn.Apply", &args, &reply); err != nil {
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
