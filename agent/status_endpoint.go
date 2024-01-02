// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package agent

import (
	"net/http"

	"github.com/hashicorp/consul/agent/structs"
)

func (s *HTTPHandlers) StatusLeader(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out string
	if err := s.agent.RPC(req.Context(), "Status.Leader", &args, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *HTTPHandlers) StatusPeers(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	args := structs.DCSpecificRequest{}
	if done := s.parse(resp, req, &args.Datacenter, &args.QueryOptions); done {
		return nil, nil
	}

	var out []string
	if err := s.agent.RPC(req.Context(), "Status.Peers", &args, &out); err != nil {
		return nil, err
	}
	return out, nil
}
