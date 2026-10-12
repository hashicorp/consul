// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package pbconfigentry

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/agent/structs"
)

// The three field classes of structs.InferenceGatewayConfigEntry, as documented on
// the struct itself. Every field must be listed in exactly one of them; the
// exhaustiveness check below fails on any field that is not, so adding a field to
// the config entry forces a deliberate choice about whether it crosses the
// pbsubscribe stream.
var (
	// identityFields are populated by the ConfigEntry envelope (or the entry's own
	// GetKind), not by the inner InferenceGateway message.
	identityFields = []string{"Kind", "Name", "EnterpriseMeta", "RaftIndex"}

	// carriedFields must survive the wire. That is every non-identity field:
	// RENDERED ones (Processor, Failover, RequestTimeout) because proxycfg and the xDS renderer
	// read them, and FORWARDED ones (PII, Observability) because this stream is the
	// only way they reach the policy processor at all — Consul renders them into
	// listener metadata rather than the processor fetching them itself.
	//
	// There is deliberately no third list. A field that does not cross this stream
	// reaches nothing.
	carriedFields = []string{"Processor", "Failover", "RequestTimeout", "Meta", "Hash", "PII", "Observability"}
)

// testInferenceGatewayEntry is an entry with every field set to a non-zero value,
// so neither the "survives" nor the "does not survive" assertions can pass
// vacuously.
func testInferenceGatewayEntry() *structs.InferenceGatewayConfigEntry {
	return &structs.InferenceGatewayConfigEntry{
		Kind: structs.InferenceGateway,
		Name: "travel-inference-gateway",
		Meta: map[string]string{"owner": "platform"},
		Hash: 12345,
		Processor: structs.InferenceGatewayProcessor{
			BodyModelRouting: true,
			FailureMode:      structs.InferenceGatewayFailureModeOpen,
		},
		Failover: &structs.InferenceGatewayFailover{
			RetryOn:       []string{"401", "5xx"},
			MaxTiers:      3,
			PerTryTimeout: "30s",
		},
		RequestTimeout: "10m",
		PII: &structs.InferenceGatewayPII{
			Scope:               "both",
			DefaultAction:       "placeholder",
			StreamHoldbackBytes: 128,
			Mask:                &structs.InferenceGatewayPIIMask{Char: "*", KeepLast: 4},
			Detectors:           []structs.InferenceGatewayPIIDetector{{Name: "ssn", Action: "block"}},
		},
		Observability: &structs.InferenceGatewayObservability{
			Metrics: &structs.InferenceGatewayMetrics{
				Prometheus: &structs.InferenceGatewayMetricsPrometheus{Port: intPtr(9105)},
				OTLP:       &structs.InferenceGatewayOTLPExport{Endpoint: "collector:4317", Insecure: true},
			},
			Tracing: &structs.InferenceGatewayTracing{
				Enabled:     true,
				OTLP:        &structs.InferenceGatewayOTLPExport{Endpoint: "collector:4317"},
				SampleRatio: 0.05,
			},
		},
	}
}

// TestInferenceGateway_ProtoRoundTrip guards the inference-gateway config entry's
// generated protobuf bindings. On a server-managed proxy, proxycfg receives config
// entries over the pbsubscribe stream, which serializes through this message — so
// every RENDERED field must survive structs -> proto -> bytes ->
// proto -> structs. If one is dropped from the generated pb.go, the gateway
// silently renders without it even though `consul config read` still shows it.
func TestInferenceGateway_ProtoRoundTrip(t *testing.T) {
	src := testInferenceGatewayEntry()

	pb := ConfigEntryFromStructs(src)
	b, err := proto.Marshal(pb)
	require.NoError(t, err)

	var pb2 ConfigEntry
	require.NoError(t, proto.Unmarshal(b, &pb2))

	got, ok := ConfigEntryToStructs(&pb2).(*structs.InferenceGatewayConfigEntry)
	require.True(t, ok)

	require.Equal(t, structs.InferenceGateway, got.GetKind())
	require.Equal(t, src.Name, got.Name)

	srcVal, gotVal := reflect.ValueOf(*src), reflect.ValueOf(*got)

	// RENDERED: unchanged across the wire.
	for _, name := range carriedFields {
		t.Run("carried/"+name, func(t *testing.T) {
			require.False(t, srcVal.FieldByName(name).IsZero(),
				"fixture must set %s, otherwise this assertion is vacuous", name)
			require.Equal(t, srcVal.FieldByName(name).Interface(), gotVal.FieldByName(name).Interface(),
				"%s is read by proxycfg/xDS and must survive the pbsubscribe stream", name)
		})
	}

}

// TestInferenceGateway_ProtoFieldsClassified fails when a field is added to the
// config entry without deciding whether it crosses the pbsubscribe stream. The
// classification is documented on structs.InferenceGatewayConfigEntry.
func TestInferenceGateway_ProtoFieldsClassified(t *testing.T) {
	classified := make(map[string]string)
	for class, names := range map[string][]string{
		"identity": identityFields,
		"carried":  carriedFields,
	} {
		for _, n := range names {
			require.NotContains(t, classified, n, "%s is classified twice", n)
			classified[n] = class
		}
	}

	typ := reflect.TypeOf(structs.InferenceGatewayConfigEntry{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		require.Contains(t, classified, name,
			"InferenceGatewayConfigEntry.%s is unclassified: add it to the InferenceGateway "+
				"proto message and to carriedFields. Whether Consul RENDERS it into Envoy "+
				"config or merely FORWARDS it to the processor as listener metadata, it has "+
				"to cross the pbsubscribe stream — there is no path to the processor that "+
				"skips it", name)
	}

	// And nothing classified has since been removed from the struct.
	for name := range classified {
		_, ok := typ.FieldByName(name)
		require.True(t, ok, "%s is classified but no longer a field on InferenceGatewayConfigEntry", name)
	}
}

// TestInferenceGateway_MetricsEnabledThreeState pins the one field where a lossy
// pointer conversion would be silently destructive.
//
// InferenceGatewayMetrics.Enabled defaults to ON when unset, so unset and explicit
// false are different instructions. The numeric mog helpers in this package
// deliberately collapse nil to the zero value — harmless where 0 and unset mean the
// same thing — and reusing that shape here would turn every gateway that never
// mentioned Enabled into one with metrics switched off, on server-managed proxies
// only. Hence google.protobuf.BoolValue rather than a bare bool.
func TestInferenceGateway_MetricsEnabledThreeState(t *testing.T) {
	ptr := func(b bool) *bool { return &b }

	for name, want := range map[string]*bool{
		"unset stays unset":       nil,
		"explicit true survives":  ptr(true),
		"explicit false survives": ptr(false),
	} {
		t.Run(name, func(t *testing.T) {
			src := testInferenceGatewayEntry()
			src.Observability.Metrics.Enabled = want

			var pb InferenceGateway
			InferenceGatewayFromStructs(src, &pb)

			var got structs.InferenceGatewayConfigEntry
			InferenceGatewayToStructs(&pb, &got)

			require.Equal(t, want, got.Observability.Metrics.Enabled)
		})
	}
}

// TestInferenceGateway_PrometheusPortThreeState is the Port counterpart of the
// Enabled test above. Port = 0 turns the scrape endpoint off and an unset port keeps
// the default, so a bare int32 - which cannot tell the two apart - would silently
// re-enable the endpoint on every server-managed proxy that asked for it off.
func TestInferenceGateway_PrometheusPortThreeState(t *testing.T) {
	for name, want := range map[string]*int{
		"unset stays unset":      nil,
		"explicit 0 survives":    intPtr(0),
		"explicit port survives": intPtr(9200),
	} {
		t.Run(name, func(t *testing.T) {
			src := testInferenceGatewayEntry()
			src.Observability.Metrics.Prometheus.Port = want

			// Through bytes, not just the struct conversion: the pbsubscribe stream
			// is where a bare int32 would lose the distinction.
			b, err := proto.Marshal(ConfigEntryFromStructs(src))
			require.NoError(t, err)
			var pb ConfigEntry
			require.NoError(t, proto.Unmarshal(b, &pb))

			got := ConfigEntryToStructs(&pb).(*structs.InferenceGatewayConfigEntry)
			require.Equal(t, want, got.Observability.Metrics.Prometheus.Port)
		})
	}
}

// TestInferenceGatewayPIIEnumConversions checks every PII enum value maps across
// the wire and back to itself, and that the empty struct value maps to the proto
// zero value rather than to a real choice. The processor applies its own default
// to an empty value, so the two must never be confused.
func TestInferenceGatewayPIIEnumConversions(t *testing.T) {
	for _, s := range []structs.InferenceGatewayPIIScope{
		"",
		structs.InferenceGatewayPIIScopeRequest,
		structs.InferenceGatewayPIIScopeResponse,
		structs.InferenceGatewayPIIScopeBoth,
	} {
		require.Equal(t, s, inferenceGatewayPIIScopeToStructs(inferenceGatewayPIIScopeFromStructs(s)), "scope %q", s)
	}
	require.Equal(t, InferenceGatewayPIIScope_InferenceGatewayPIIScopeUnset, inferenceGatewayPIIScopeFromStructs(""))
	// Every proto value except unset has a struct constant.
	require.Len(t, InferenceGatewayPIIScope_name, 4)

	for _, a := range []structs.InferenceGatewayPIIAction{
		"",
		structs.InferenceGatewayPIIActionPlaceholder,
		structs.InferenceGatewayPIIActionMask,
		structs.InferenceGatewayPIIActionBlock,
		structs.InferenceGatewayPIIActionOff,
	} {
		require.Equal(t, a, inferenceGatewayPIIActionToStructs(inferenceGatewayPIIActionFromStructs(a)), "action %q", a)
	}
	require.Equal(t, InferenceGatewayPIIAction_InferenceGatewayPIIActionUnset, inferenceGatewayPIIActionFromStructs(""))
	require.Len(t, InferenceGatewayPIIAction_name, 5)
}
