package wire_test

import (
	"testing"

	"github.com/openzipkin/zipkin-go-opentracing"
	"github.com/openzipkin/zipkin-go-opentracing/flag"
	"github.com/openzipkin/zipkin-go-opentracing/types"
	"github.com/openzipkin/zipkin-go-opentracing/wire"
)

func TestProtobufCarrier(t *testing.T) {
	var carrier zipkintracer.DelegatingCarrier = &wire.ProtobufCarrier{}

	traceID := types.TraceID{High: 1, Low: 2}
	var spanID, parentSpanID uint64 = 3, 0
	sampled := true
	flags := flag.Debug | flag.Sampled | flag.SamplingSet
	baggageKey, expVal := "key1", "val1"

	carrier.SetState(traceID, spanID, &parentSpanID, sampled, flags)
	carrier.SetBaggageItem(baggageKey, expVal)
	gotTraceID, gotSpanID, gotParentSpanId, gotSampled, gotFlags := carrier.State()

	if gotParentSpanId == nil {
		t.Errorf("Expected a valid parentSpanID of 0 got nil (no parent)")
	}

	if gotFlags&flag.IsRoot == flag.IsRoot {
		t.Errorf("Expected a child span with a valid parent span with id 0 got IsRoot flag")
	}

	if traceID != gotTraceID || spanID != gotSpanID || parentSpanID != *gotParentSpanId || sampled != gotSampled || flags != gotFlags {
		t.Errorf("Wanted state %d %d %d %t %d, got %d %d %d %t %d", spanID, traceID, parentSpanID, sampled, flags, gotTraceID, gotSpanID, *gotParentSpanId, gotSampled, gotFlags)
	}

	gotBaggage := map[string]string{}
	f := func(k, v string) {
		gotBaggage[k] = v
	}

	carrier.GetBaggage(f)
	value, ok := gotBaggage[baggageKey]
	if !ok {
		t.Errorf("Expected baggage item %s to exist", baggageKey)
	}
	if value != expVal {
		t.Errorf("Expected key %s to be %s, got %s", baggageKey, expVal, value)
	}
}
