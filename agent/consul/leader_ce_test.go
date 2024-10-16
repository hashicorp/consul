// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"github.com/hashicorp/consul/internal/gossip/libserf"
)

func updateSerfTags(s *Server, key, value string) {
	libserf.UpdateTag(s.serfLAN, key, value)

	if s.serfWAN != nil {
		libserf.UpdateTag(s.serfWAN, key, value)
	}
}
