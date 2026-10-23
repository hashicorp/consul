// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/structs"
)

// TestUpstreamLimits_PassiveHealthCheck_ProtoRoundTrip guards against the
// PassiveHealthCheck field being dropped from the generated UpstreamLimits
// protobuf bindings. API gateway config entries (their Defaults and route
// service-level Limits) reach proxycfg over the streaming/subscribe path, which
// serializes through this pbconfigentry message. If the generated pb.go for
// UpstreamLimits is regenerated without field 4, PassiveHealthCheck is silently
// lost on the wire and services routed through an API gateway never receive
// Envoy outlier detection, even though the config entry stored via msgpack
// still reports it.
func TestUpstreamLimits_PassiveHealthCheck_ProtoRoundTrip(t *testing.T) {
	src := &structs.UpstreamLimits{
		MaxConnections:        intPtr(5),
		MaxPendingRequests:    intPtr(3),
		MaxConcurrentRequests: intPtr(4),
		PassiveHealthCheck: &structs.PassiveHealthCheck{
			Interval:           5 * time.Second,
			MaxFailures:        5,
			MaxEjectionPercent: uint32Ptr(50),
		},
	}

	// structs -> proto -> bytes -> proto -> structs (the subscribe wire path).
	var pb UpstreamLimits
	UpstreamLimitsFromStructs(src, &pb)

	b, err := proto.Marshal(&pb)
	require.NoError(t, err)

	var pb2 UpstreamLimits
	require.NoError(t, proto.Unmarshal(b, &pb2))

	var out structs.UpstreamLimits
	UpstreamLimitsToStructs(&pb2, &out)

	require.NotNil(t, out.MaxConnections)
	require.Equal(t, 5, *out.MaxConnections)

	require.NotNil(t, out.PassiveHealthCheck, "PassiveHealthCheck must survive the pbconfigentry proto round-trip")
	require.Equal(t, 5*time.Second, out.PassiveHealthCheck.Interval)
	require.Equal(t, uint32(5), out.PassiveHealthCheck.MaxFailures)
	require.NotNil(t, out.PassiveHealthCheck.MaxEjectionPercent)
	require.Equal(t, uint32(50), *out.PassiveHealthCheck.MaxEjectionPercent)
}

func intPtr(v int) *int          { return &v }
func uint32Ptr(v uint32) *uint32 { return &v }
