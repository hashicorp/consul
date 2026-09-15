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

	// carriedFields are RENDERED: proxycfg or the xDS renderer reads
	// them, so they must survive the wire.
	carriedFields = []string{"Processor", "Failover", "Meta", "Hash"}

	// storedFields are STORED: Consul only stores and returns them, and the
	// co-located processor fetches them from the config-entry HTTP API. They are in
	// the proto message's ignore-fields and must NOT survive the wire.
	storedFields = []string{"PII", "AuditLevel", "Observability"}
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
			UDSPath:          "/run/consul/ext_proc.sock",
			FailureMode:      structs.InferenceGatewayFailureModeOpen,
		},
		Failover: &structs.InferenceGatewayFailover{
			RetryOn:       []string{"401", "5xx"},
			MaxTiers:      3,
			PerTryTimeout: "30s",
		},
		PII: &structs.InferenceGatewayPII{
			Scope:               "both",
			DefaultAction:       "placeholder",
			StreamHoldbackBytes: 128,
			Mask:                &structs.InferenceGatewayPIIMask{Char: "*", KeepLast: 4},
			Detectors:           []structs.InferenceGatewayPIIDetector{{Name: "ssn", Action: "block"}},
		},
		AuditLevel: "full",
		Observability: &structs.InferenceGatewayObservability{
			Audit: &structs.InferenceGatewayAudit{
				Level:      "sampling",
				SampleRate: 0.1,
				Sink:       &structs.InferenceGatewayAuditSink{Type: "stdout", Format: "json"},
			},
			Metrics: &structs.InferenceGatewayMetrics{
				Prometheus:   &structs.InferenceGatewayMetricsPrometheus{Port: 9105, Path: "/metrics"},
				OTLP:         &structs.InferenceGatewayOTLPExport{Endpoint: "collector:4317", Insecure: true},
				CustomLabels: []string{"team"},
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

	// STORED: deliberately not on the wire. The processor fetches these from the
	// config-entry HTTP API, so they are in the proto message's ignore-fields. If a
	// control-plane component starts reading one, add it to the proto message and
	// move it to carriedFields — do not just delete the assertion.
	for _, name := range storedFields {
		t.Run("stored/"+name, func(t *testing.T) {
			require.False(t, srcVal.FieldByName(name).IsZero(),
				"fixture must set %s, otherwise this assertion is vacuous", name)
			require.True(t, gotVal.FieldByName(name).IsZero(),
				"%s is a STORED field and must not cross the pbsubscribe stream", name)
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
		"stored":   storedFields,
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
			"InferenceGatewayConfigEntry.%s is unclassified: decide whether it is RENDERED "+
				"(add it to the InferenceGateway proto message and to carriedFields) or STORED "+
				"(add it to the message's ignore-fields and to storedFields)", name)
	}

	// And nothing classified has since been removed from the struct.
	for name := range classified {
		_, ok := typ.FieldByName(name)
		require.True(t, ok, "%s is classified but no longer a field on InferenceGatewayConfigEntry", name)
	}
}
