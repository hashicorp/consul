// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/metadata"
	autopilot "github.com/hashicorp/raft-autopilot"
)

func (s *Server) autopilotPromoter() autopilot.Promoter {
	return autopilot.DefaultPromoter()
}

func (*Server) autopilotServerExt(_ *metadata.Server) interface{} {
	return nil
}
