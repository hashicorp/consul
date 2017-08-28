package types

import "testing"

func TestTraceID(t *testing.T) {

	traceID := TraceID{High: 1, Low: 2}

	if len(traceID.ToHex()) != 32 {
		t.Errorf("Expected zero-padded TraceID to have 32 characters")
	}

}
