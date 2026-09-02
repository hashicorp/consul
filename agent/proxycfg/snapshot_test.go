// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package proxycfg

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/proto/private/pbpeering"
)

func TestConfigSnapshot_Clone(t *testing.T) {
	// ConfigSnapshot is a complex struct that (directly or indirectly) has a copy
	// of most of the structs in the agent/structs package. It'd be easy to break
	// the Clone method accidentally by changing one of those distant structs, so
	// we test it by using a fuzzer to traverse the graph and fill every field and
	// then compare the original to the copy.
	f := fuzz.New()
	f.NilChance(0)
	f.NumElements(1, 3)
	f.SkipFieldsWithPattern(regexp.MustCompile("^ServerSNIFn$"))

	f.Funcs(
		// Populate map[string]interface{} since gofuzz panics on these. We force them
		// to be any rather than concrete types otherwise they won't compare equal when
		// coming back out the other side.
		func(m map[string]any, c fuzz.Continue) {
			m[c.RandString()] = any(float64(c.RandUint64()))
			m[c.RandString()] = any(c.RandString())
			m[c.RandString()] = any([]any{c.RandString(), c.RandString()})
			m[c.RandString()] = any(map[string]any{c.RandString(): c.RandString()})
		},
		func(*context.CancelFunc, fuzz.Continue) {},
	)

	snapshot := new(ConfigSnapshot)
	f.Fuzz(snapshot)

	clone := snapshot.Clone()

	diff := cmp.Diff(snapshot, clone,
		cmpopts.IgnoreUnexported(indexedTarget{}),
		cmpopts.IgnoreUnexported(pbpeering.PeeringTrustBundle{}),
		cmpopts.IgnoreTypes(context.CancelFunc(nil)),
		cmpopts.IgnoreTypes(computedFields{}),
	)
	if diff != "" {
		t.Logf("Copied snaspshot is different to the original. You may need to re-run `make deep-copy`.\nDiff:\n%s", diff)
		t.FailNow()
	}
}

func TestConfigSnapshotUpstreams_PeeredUpstreamPortVIPs(t *testing.T) {
	mkUID := func(name, peer, partition, namespace string) UpstreamID {
		entMeta := acl.NewEnterpriseMetaWithPartition(partition, namespace)
		return NewUpstreamIDFromPeeredServiceName(structs.PeeredServiceName{
			ServiceName: structs.NewServiceName(name, &entMeta),
			Peer:        peer,
		})
	}

	base := mkUID("service-response", "peer-a", "default", "default")
	upstreams := ConfigSnapshotUpstreams{
		PeeredPortUpstreamVIPs: map[UpstreamID]string{
			base: "240.0.0.9",
			mkUID("api.service-response", "peer-a", "default", "default"):         "240.0.0.6",
			mkUID("metrics.service-response", "peer-a", "default", "default"):     "240.0.0.7",
			mkUID("admin.service-response", "peer-b", "default", "default"):       "240.0.0.8",
			mkUID("nested.port.service-response", "peer-a", "default", "default"): "240.0.0.12",
			mkUID("api.other-service", "peer-a", "default", "default"):            "240.0.0.13",
		},
	}

	require.Equal(t, map[string]string{
		"api":     "240.0.0.6",
		"metrics": "240.0.0.7",
	}, upstreams.PeeredUpstreamPortVIPs(base))

	explicitPort := base
	explicitPort.DestinationPort = "api"
	require.Nil(t, upstreams.PeeredUpstreamPortVIPs(explicitPort))

	clone := (&ConfigSnapshot{
		Kind: structs.ServiceKindConnectProxy,
		ConnectProxy: configSnapshotConnectProxy{
			ConfigSnapshotUpstreams: upstreams,
		},
	}).Clone()
	clone.ConnectProxy.PeeredPortUpstreamVIPs[base] = "240.0.0.99"
	require.Equal(t, "240.0.0.9", upstreams.PeeredPortUpstreamVIPs[base])
}
