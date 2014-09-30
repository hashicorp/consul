package consul

import (
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/serf/serf"
)

// ingestKeyringResponse is a helper method to pick the relative information
// from a Serf message and stuff it into a KeyringResponse.
func ingestKeyringResponse(
	serfResp *serf.KeyResponse, reply *structs.KeyringResponses,
	dc string, wan bool, err error) {

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	reply.Responses = append(reply.Responses, &structs.KeyringResponse{
		WAN:        wan,
		Datacenter: dc,
		Messages:   serfResp.Messages,
		Keys:       serfResp.Keys,
		NumNodes:   serfResp.NumNodes,
		Error:      errStr,
	})
}

// forwardKeyringRPC is used to forward a keyring-related RPC request to one
// server in each datacenter. Since the net/rpc package writes replies in-place,
// we use this specialized method for dealing with keyring-related replies
// specifically by appending them to a wrapper response struct.
//
// This will only error for RPC-related errors. Otherwise, application-level
// errors are returned inside of the inner response objects.
func (s *Server) forwardKeyringRPC(
	method string,
	args *structs.KeyringRequest,
	replies *structs.KeyringResponses) error {

	for dc, _ := range s.remoteConsuls {
		if dc == s.config.Datacenter {
			continue
		}
		rr := structs.KeyringResponses{}
		if err := s.forwardDC(method, dc, args, &rr); err != nil {
			return err
		}
		for _, r := range rr.Responses {
			replies.Responses = append(replies.Responses, r)
		}
	}

	return nil
}
