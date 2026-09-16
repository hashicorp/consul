// Copyright IBM Corp. 2026
// SPDX-License-Identifier: BUSL-1.1

package locality

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/consul/testing/deployer/topology"
	"github.com/stretchr/testify/require"
)

// TestCommonTopologySetup boots the shared multi-DC topology and asserts
// locality-aware first-hop DNS answers per zone (catalog readiness is gated in Launch).
func TestCommonTopologySetup(t *testing.T) {
	ct := NewCommonTopo(t)
	ct.Launch(t)
	// this will write out the topology of a sparwled clusters
	ct.LogTopology(t)
	// and this one will write out some example of commands for manual DNS inspection
	ct.LogManualDNSCommands(t)

	assertDNSLocalityAwareLookup(t, ct)

	// sprawltest only skips Sprawl.Stop when t.Failed() && SPRAWL_KEEP_RUNNING=1.
	// For a passing test the harness always tears down, so for the manual
	// inspection workflow in README.md we intentionally fail after assertions.
	if os.Getenv("SPRAWL_KEEP_RUNNING") == "1" {
		t.Log("SPRAWL_KEEP_RUNNING=1: marking test failed so sprawltest leaves containers up; go test exits non-zero; destroy with terraform under SPRAWL_WORKDIR_ROOT when done")
		t.Fail()
	}
}

// ipsFromServiceEntries returns service IPs from health catalog entries, optionally
// filtered to a single zone (empty zone means all entries).
func ipsFromServiceEntries(entries []*api.ServiceEntry, zone string) []string {
	ips := make([]string, 0, len(entries))
	for _, entry := range entries {
		addr := ""
		if entry.Service != nil && entry.Service.Address != "" {
			addr = entry.Service.Address
		} else if entry.Node != nil {
			addr = entry.Node.Address
		}

		if addr == "" {
			continue
		}

		nodeZone := ""
		if entry.Node != nil && entry.Node.Locality != nil {
			nodeZone = entry.Node.Locality.Zone
		}

		if zone == "" || nodeZone == zone {
			ips = append(ips, addr)
		}
	}
	return ips
}

// assertDNSLocalityAwareLookup checks that DNS A records from each zone's query
// client match locality-aware filtering for that cluster's dns_config mode.
func assertDNSLocalityAwareLookup(t *testing.T, ct *commonTopo) {
	t.Helper()

	for _, cluster := range ct.allClustersSpecs() {
		client := ct.APIClientForCluster(t, ct.clusterLive(cluster.Name))

		queryNodes := ct.NodesLive(cluster.Name, "client", "query", "")
		require.NotEmpty(t, queryNodes, "missing query nodes for %s", cluster.Name)

		for _, svc := range topologyServices {
			expectedZoneWorkloads := expectedWorkloadsByZone(cluster, svc)
			expectedTotal := 0
			for _, z := range cluster.Zones {
				expectedTotal += expectedZoneWorkloads[z]
			}

			for _, queryNode := range queryNodes {
				queryZone := queryNode.Meta["zone"]
				require.NotEmpty(t, queryZone, "query node %s missing zone meta", queryNode.Name)

				retry.Run(t, func(r *retry.R) {
					entries, _, err := client.Health().Service(svc.Name, "", true, nil)
					require.NoError(r, err)
					if len(entries) != expectedTotal {
						allEntries, _, allErr := client.Health().Service(svc.Name, "", false, nil)
						require.NoError(r, allErr)
						r.Logf("passing %s entries in %s: %s", svc.Name, cluster.Name, summarizeServiceEntries(entries))
						r.Logf("all %s entries in %s: %s", svc.Name, cluster.Name, summarizeServiceEntries(allEntries))
					}
					require.Len(r, entries, expectedTotal,
						"want %d passing %s instances in %s before DNS check (got %d)",
						expectedTotal, svc.Name, cluster.Name, len(entries))

					gotIPs := lookupARecordsInContainer(r, queryNode, svc.Name+".service.consul")

					if expectedTotal == 0 {
						require.Empty(r, gotIPs,
							"expected no DNS answers for %s in %s via %s (no workloads registered)",
							svc.Name, cluster.Name, queryNode.Name)
						return
					}

					expectedIPs := expectedDNSIPsForQuery(cluster, svc.Name, entries, queryZone)
					require.NotEmpty(r, expectedIPs,
						"no expected %s IPs for %s zone %s (mode=%q)",
						svc.Name, cluster.Name, queryZone, cluster.LocalityAwareLookup)

					require.NotEmpty(r, gotIPs, "expected A record answers for %s via %s", svc.Name, queryNode.Name)

					if cluster.LocalityAwareLookup == "proportional" && localityAwareLookupAppliesToService(cluster, svc.Name) {
						assertProportionalDNSIPs(r, entries, queryZone, gotIPs)
						return
					}

					require.ElementsMatch(r, expectedIPs, gotIPs,
						"DNS answers for %s in %s via %s (zone=%s, mode=%q)",
						svc.Name, cluster.Name, queryNode.Name, queryZone, cluster.LocalityAwareLookup)
				}, retry.WithRetryer(retry.ThirtySeconds()))
			}
		}
	}
}

// serviceZonesInBalance mirrors agent/dns.go localityZoneCountsInBalance for the
// passing health catalog entries in a cluster region.
func serviceZonesInBalance(entries []*api.ServiceEntry, region, queryZone string) bool {
	if queryZone == "" {
		return false
	}

	counts := map[string]int{}
	for _, entry := range entries {
		if entry.Node == nil || entry.Node.Locality == nil {
			continue
		}
		loc := entry.Node.Locality
		if loc.Region != region || loc.Zone == "" {
			continue
		}
		counts[loc.Zone]++
	}

	if len(counts) == 0 {
		return false
	}

	localCount, ok := counts[queryZone]
	if !ok {
		return false
	}

	for _, count := range counts {
		if count != localCount {
			return false
		}
	}
	return true
}

// expectedDNSIPsForQuery returns the service IPs that should appear in DNS answers
// for a query client in queryZone, given the cluster's locality-aware DNS settings.
// IPs are derived from the same health entries used for the balance check so
// expectations stay consistent while the catalog is still converging.
func expectedDNSIPsForQuery(cluster clusterSpec, service string, entries []*api.ServiceEntry, queryZone string) []string {
	if !localityAwareLookupAppliesToService(cluster, service) {
		return ipsFromServiceEntries(entries, "")
	}

	switch cluster.LocalityAwareLookup {
	case "off":
		return ipsFromServiceEntries(entries, "")
	case "always":
		return ipsFromServiceEntries(entries, queryZone)
	case "balanced":
		if serviceZonesInBalance(entries, cluster.Region, queryZone) {
			return ipsFromServiceEntries(entries, queryZone)
		}
		return ipsFromServiceEntries(entries, "")
	case "proportional":
		// Proportional answers vary per query; callers that need a non-empty
		// readiness gate can use the local zone set (always included when present).
		local := ipsFromServiceEntries(entries, queryZone)
		if len(local) > 0 {
			return local
		}
		return ipsFromServiceEntries(entries, "")
	default:
		return ipsFromServiceEntries(entries, queryZone)
	}
}

// assertProportionalDNSIPs checks invariants of a single proportional-mode DNS
// answer: every local-zone IP is present, every answer IP is in-region, and the
// answer length is between the local count and the ceil of mean instances/zone.
func assertProportionalDNSIPs(t require.TestingT, entries []*api.ServiceEntry, queryZone string, gotIPs []string) {
	localIPs := ipsFromServiceEntries(entries, queryZone)
	regionIPs := ipsFromServiceEntries(entries, "")
	require.NotEmpty(t, localIPs, "proportional mode expects local-zone instances for %s", queryZone)

	gotSet := map[string]struct{}{}
	for _, ip := range gotIPs {
		gotSet[ip] = struct{}{}
	}
	regionSet := map[string]struct{}{}
	for _, ip := range regionIPs {
		regionSet[ip] = struct{}{}
	}

	for _, ip := range localIPs {
		_, ok := gotSet[ip]
		require.True(t, ok, "proportional answer missing local IP %s (got %v)", ip, gotIPs)
	}
	for _, ip := range gotIPs {
		_, ok := regionSet[ip]
		require.True(t, ok, "proportional answer includes out-of-region IP %s", ip)
	}

	n := len(localIPs)
	N := len(regionIPs)
	// zone count from entries
	zones := map[string]struct{}{}
	for _, entry := range entries {
		if entry.Node == nil || entry.Node.Locality == nil || entry.Node.Locality.Zone == "" {
			continue
		}
		zones[entry.Node.Locality.Zone] = struct{}{}
	}
	K := len(zones)
	require.Greater(t, K, 0)

	maxLen := n
	if K > 0 {
		// ceil(N/K)
		ceilMean := (N + K - 1) / K
		if ceilMean > maxLen {
			maxLen = ceilMean
		}
	}
	if maxLen > N {
		maxLen = N
	}
	require.GreaterOrEqual(t, len(gotIPs), n)
	require.LessOrEqual(t, len(gotIPs), maxLen)
}

// localityAwareLookupAppliesToService mirrors dnsServerConfig.localityAwareLookupAppliesTo.
func localityAwareLookupAppliesToService(cluster clusterSpec, service string) bool {
	if len(cluster.ServiceAllowlist) > 0 {
		return containsService(cluster.ServiceAllowlist, service)
	}
	if len(cluster.ServiceBlocklist) > 0 {
		return !containsService(cluster.ServiceBlocklist, service)
	}
	return true
}

func containsService(services []string, service string) bool {
	for _, candidate := range services {
		if candidate == service {
			return true
		}
	}
	return false
}

// lookupARecordsInContainer runs dig inside the node's container and returns A record IPs.
func lookupARecordsInContainer(t require.TestingT, node *topology.Node, name string) []string {
	args := []string{
		"exec", node.DockerName(),
		"dig", "@127.0.0.1", "-p", "8600", name, "A", "+short",
	}
	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		require.Fail(t, "dns lookup failed for %s via %s: %v: %s", name, node.Name, err, string(out))
		return nil
	}

	var ips []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		ip := strings.TrimSpace(line)
		if ip == "" || ip == "127.0.0.1" {
			continue
		}
		ips = append(ips, ip)
	}
	return ips
}
