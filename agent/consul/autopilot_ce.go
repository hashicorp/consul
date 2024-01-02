// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package consul

import (
	"github.com/hashicorp/consul/agent/metadata"
	autopilot "github.com/hashicorp/raft-autopilot"
)

func (s *Server) autopilotPromoter() autopilot.Promoter {
	return autopilot.DefaultPromoter()
}

func (_ *Server) autopilotServerExt(_ *metadata.Server) interface{} {
	return nil
}
