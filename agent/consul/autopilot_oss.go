// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !consulent
// +build !consulent

package consul

import (
	autopilot "github.com/hashicorp/raft-autopilot"

	"github.com/hashicorp/consul/agent/metadata"
)

func (s *Server) autopilotPromoter() autopilot.Promoter {
	return autopilot.DefaultPromoter()
}

func (_ *Server) autopilotServerExt(_ *metadata.Server) interface{} {
	return nil
}
