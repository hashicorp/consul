// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package proxystateconverter

import (
	"fmt"
	"strings"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/proxycfg"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/agent/xds/naming"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

const (
	meshGatewayExportedClusterNamePrefix = "exported~"
)

func makeExposeClusterName(destinationPort int) string {
	return fmt.Sprintf("exposed_cluster_%d", destinationPort)
}

func clusterNameForDestination(cfgSnap *proxycfg.ConfigSnapshot, name string, address string, namespace string, partition string) string {
	name = destinationSpecificServiceName(name, address)
	sni := connect.ServiceSNI(name, "", namespace, partition, cfgSnap.Datacenter, cfgSnap.Roots.TrustDomain)

	// Prefixed with destination to distinguish from non-passthrough clusters for the same upstream.
	return "destination." + sni
}

func destinationSpecificServiceName(name string, address string) string {
	address = strings.ReplaceAll(address, ":", "-")
	address = strings.ReplaceAll(address, ".", "-")
	return fmt.Sprintf("%s.%s", address, name)
}

// generatePeeredClusterName returns an SNI-like cluster name which mimics PeeredServiceSNI
// but excludes partition information which could be ambiguous (local vs remote partition).
func generatePeeredClusterName(uid proxycfg.UpstreamID, tb *pbpeering.PeeringTrustBundle) string {
	return strings.Join([]string{
		uid.Name,
		uid.NamespaceOrDefault(),
		uid.Peer,
		"external",
		tb.TrustDomain,
	}, ".")
}

func (s *Converter) getTargetClusterName(upstreamsSnapshot *proxycfg.ConfigSnapshotUpstreams, chain *structs.CompiledDiscoveryChain, tid string, forMeshGateway bool) string {
	target := chain.Targets[tid]
	clusterName := target.Name
	targetUID := proxycfg.NewUpstreamIDFromTargetID(tid)
	if targetUID.Peer != "" {
		tbs, ok := upstreamsSnapshot.UpstreamPeerTrustBundles.Get(targetUID.Peer)
		// We can't generate cluster on peers without the trust bundle. The
		// trust bundle should be ready soon.
		if !ok {
			s.Logger.Debug("peer trust bundle not ready for discovery chain target",
				"peer", targetUID.Peer,
				"target", tid,
			)
			return ""
		}

		clusterName = generatePeeredClusterName(targetUID, tbs)
	}
	clusterName = naming.CustomizeClusterName(clusterName, chain)
	if forMeshGateway {
		clusterName = meshGatewayExportedClusterNamePrefix + clusterName
	}
	return clusterName
}
