// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package consul

import (
	"github.com/hashicorp/serf/serf"

	"github.com/hashicorp/consul/agent/structs"
)

func meshEntryConfiguresECDHCurves(entry structs.ConfigEntry) bool { return false }

func meshEntryEnablesPQC(entry structs.ConfigEntry) bool { return false }

func serversSupportMeshECDHCurves(provider checkServersProvider, datacenter string) error {
	return nil
}

func agentsWithoutMeshPQCSupport(members []serf.Member) []string { return []string{"stub"} }

func formatNodeNames(names []string, limit int) string { return "" }
