package agent

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/consul/structs"
)

// fixupValues takes the raw decoded JSON and base64 decodes all the values,
// replacing them with byte arrays with the data.
func fixupValues(raw interface{}) error {
	// decodeValue decodes the value member of the given operation.
	decodeValue := func(rawOp interface{}) error {
		rawMap, ok := rawOp.(map[string]interface{})
		if !ok {
			return fmt.Errorf("unexpected raw op type: %T", rawOp)
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

	rawSlice, ok := raw.([]interface{})
	if !ok {
		return fmt.Errorf("unexpected raw type: %t", raw)
	}
	for _, rawOp := range rawSlice {
		if err := decodeValue(rawOp); err != nil {
			return err
		}
	}

	return nil
}

// Txn handles requests to apply multiple operations in a single, atomic
// transaction.
func (s *HTTPServer) Txn(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.KVSAtomicRequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	// Note the body is in API format, and not the RPC format. If we can't
	// decode it, we will return a 400 since we don't have enough context to
	// associate the error with a given operation.
	var txn api.KVTxn
	if err := decodeBody(req, &txn, fixupValues); err != nil {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte(fmt.Sprintf("Failed to parse body: %v", err)))
		return nil, nil
	}

	// Convert the API format into the RPC format. Note that fixupValues
	// above will have already converted the base64 encoded strings into
	// byte arrays so we can assign right over.
	for _, in := range txn {
		// TODO @slackpad - Verify the size here, or move that down into
		// the endpoint.
		out := &structs.KVSAtomicOp{
			Op: structs.KVSOp(in.Op),
			DirEnt: structs.DirEntry{
				Key:     in.Key,
				Value:   in.Value,
				Flags:   in.Flags,
				Session: in.Session,
				RaftIndex: structs.RaftIndex{
					ModifyIndex: in.Index,
				},
			},
		}
		args.Ops = append(args.Ops, out)
	}

	// Make the request and return a conflict status if there were errors
	// reported from the transaction.
	var reply structs.KVSAtomicResponse
	if err := s.agent.RPC("Txn.Apply", &args, &reply); err != nil {
		return nil, err
	}
	if len(reply.Errors) > 0 {
		var buf []byte
		var err error
		buf, err = s.marshalJSON(req, reply)
		if err != nil {
			return nil, err
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.WriteHeader(http.StatusConflict)
		resp.Write(buf)
		return nil, nil
	}

	// Otherwise, return the results of the successful transaction.
	return reply, nil
}
