package consul

import (
	"reflect"
	"testing"
	"time"

	fuzz "github.com/google/gofuzz"
	"github.com/stretchr/testify/require"
)

func TestCloneSerfLANConfig(t *testing.T) {
	config := DefaultConfig().SerfLANConfig

	// NOTE: ALL fields on serf.Config and memberlist.Config MUST BE
	// represented either here or in the CloneSerfLANConfig function body.
	// Failure to add it to the clone or ignore sections will fail the test.
	memberlistIgnoreFieldNames := []string{
		"Alive",
		"AwarenessMaxMultiplier",
		"Conflict",
		"Delegate",
		"DelegateProtocolMax",
		"DelegateProtocolMin",
		"DelegateProtocolVersion",
		"DisableTcpPings",
		"DisableTcpPingsForNode",
		"DNSConfigPath",
		"EnableCompression",
		"Events",
		"GossipToTheDeadTime",
		"HandoffQueueDepth",
		"IndirectChecks",
		"Label",
		"Logger",
		"LogOutput",
		"Merge",
		"Name",
		"Ping",
		"ProtocolVersion",
		"PushPullInterval",
		"QueueCheckInterval",
		"RequireNodeNames",
		"SkipInboundLabelCheck",
		"SuspicionMaxTimeoutMult",
		"TCPTimeout",
		"Transport",
		"UDPBufferSize",
	}
	serfIgnoreFieldNames := []string{
		"BroadcastTimeout",
		"CoalescePeriod",
		"DisableCoordinates",
		"EnableNameConflictResolution",
		"EventBuffer",
		"EventCh",
		"FlapTimeout",
		"LeavePropagateDelay",
		"LogOutput",
		"Logger",
		"MaxQueueDepth",
		"MemberlistConfig",
		"Merge",
		"MinQueueDepth",
		"NodeName",
		"ProtocolVersion",
		"QueryBuffer",
		"QueryResponseSizeLimit",
		"QuerySizeLimit",
		"QueryTimeoutMult",
		"QueueCheckInterval",
		"QueueDepthWarning",
		"QuiescentPeriod",
		"RecentIntentTimeout",
		"ReconnectInterval",
		"ReconnectTimeoutOverride",
		"RejoinAfterLeave",
		"SnapshotPath",
		"Tags",
		"UserCoalescePeriod",
		"UserEventSizeLimit",
		"UserQuiescentPeriod",
		"ValidateNodeNames",
	}

	serfFuzzed := fuzzNonIgnoredFields(config, serfIgnoreFieldNames)
	t.Logf("Fuzzing serf.Config fields: %v", serfFuzzed)

	memberlistFuzzed := fuzzNonIgnoredFields(config.MemberlistConfig, memberlistIgnoreFieldNames)
	t.Logf("Fuzzing memberlist.Config fields: %v", memberlistFuzzed)

	clone := CloneSerfLANConfig(config)
	require.Equal(t, config, clone)
}

func fuzzNonIgnoredFields(value interface{}, ignoredFields []string) []string {
	ignored := make(map[string]struct{})
	for _, field := range ignoredFields {
		ignored[field] = struct{}{}
	}

	var fuzzed []string

	// Walk the fields of our object to fuzz and selectively only fuzz the
	// fields that were not ignored.
	fuzzer := fuzz.NewWithSeed(time.Now().UnixNano())

	v := reflect.ValueOf(value).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanInterface() {
			continue // skip unexported fields
		}

		fieldName := v.Type().Field(i).Name
		if _, ok := ignored[fieldName]; ok {
			continue
		}

		fuzzed = append(fuzzed, fieldName)

		// copy the data somewhere mutable
		tmp := reflect.New(field.Type())
		tmp.Elem().Set(field)
		// fuzz the copy
		fuzzer.Fuzz(tmp.Interface())
		// and set the fuzzed copy back to the original location
		field.Set(reflect.Indirect(tmp))
	}

	return fuzzed
}
