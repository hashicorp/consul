package agent

import (
	"fmt"
	"net/http"

	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/raft"
)

// OperatorRaftConfiguration is used to inspect the current Raft configuration.
// This supports the stale query mode in case the cluster doesn't have a leader.
func (s *HTTPServer) OperatorRaftConfiguration(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.DCSpecificRequest
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var reply structs.RaftConfigurationResponse
	if err := s.agent.RPC("Operator.RaftGetConfiguration", &args, &reply); err != nil {
		return nil, err
	}

	return reply, nil
}

// OperatorRaftPeer supports actions on Raft peers. Currently we only support
// removing peers by address.
func (s *HTTPServer) OperatorRaftPeer(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "DELETE" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.RaftPeerByAddressRequest
	s.parseDC(req, &args.Datacenter)
	s.parseToken(req, &args.Token)

	params := req.URL.Query()
	if _, ok := params["address"]; ok {
		args.Address = raft.ServerAddress(params.Get("address"))
	} else {
		resp.WriteHeader(http.StatusBadRequest)
		resp.Write([]byte("Must specify ?address with IP:port of peer to remove"))
		return nil, nil
	}

	var reply struct{}
	if err := s.agent.RPC("Operator.RaftRemovePeerByAddress", &args, &reply); err != nil {
		return nil, err
	}
	return nil, nil
}

// OperatorKeyringInstall is used to install a new gossip encryption key into the cluster
func (s *HTTPServer) OperatorKeyringInstall(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.KeyringRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}
	s.parseToken(req, &args.Token)

	responses, err := s.agent.InstallKey(args.Key, args.Token)
	if err != nil {
		return nil, err
	}
	for _, response := range responses.Responses {
		if response.Error != "" {
			return nil, fmt.Errorf(response.Error)
		}
	}

	return nil, nil
}

// OperatorKeyringList is used to list the keys installed in the cluster
func (s *HTTPServer) OperatorKeyringList(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var token string
	s.parseToken(req, &token)

	responses, err := s.agent.ListKeys(token)
	if err != nil {
		return nil, err
	}
	for _, response := range responses.Responses {
		if response.Error != "" {
			return nil, fmt.Errorf(response.Error)
		}
	}

	return responses.Responses, nil
}

// OperatorKeyringRemove is used to list the keys installed in the cluster
func (s *HTTPServer) OperatorKeyringRemove(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "DELETE" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.KeyringRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}
	s.parseToken(req, &args.Token)

	responses, err := s.agent.RemoveKey(args.Key, args.Token)
	if err != nil {
		return nil, err
	}
	for _, response := range responses.Responses {
		if response.Error != "" {
			return nil, fmt.Errorf(response.Error)
		}
	}

	return nil, nil
}

// OperatorKeyringUse is used to change the primary gossip encryption key
func (s *HTTPServer) OperatorKeyringUse(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "PUT" {
		resp.WriteHeader(http.StatusMethodNotAllowed)
		return nil, nil
	}

	var args structs.KeyringRequest
	if err := decodeBody(req, &args, nil); err != nil {
		resp.WriteHeader(400)
		resp.Write([]byte(fmt.Sprintf("Request decode failed: %v", err)))
		return nil, nil
	}
	s.parseToken(req, &args.Token)

	responses, err := s.agent.UseKey(args.Key, args.Token)
	if err != nil {
		return nil, err
	}
	for _, response := range responses.Responses {
		if response.Error != "" {
			return nil, fmt.Errorf(response.Error)
		}
	}

	return nil, nil
}
