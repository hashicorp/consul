// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent

package consul

import (
	"testing"

	"github.com/hashicorp/consul/agent/structs"
)

func testInternalPeeredUpstreamsPortVIPs(_ *testing.T, _ structs.IndexedPeeredServiceList) {}
