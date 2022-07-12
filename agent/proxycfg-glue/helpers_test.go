package proxycfgglue

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/agent/proxycfg"
)

func getEventResult[ResultType any](t *testing.T, eventCh <-chan proxycfg.UpdateEvent) ResultType {
	t.Helper()

	select {
	case event := <-eventCh:
		require.NoError(t, event.Err, "event should not have an error")
		result, ok := event.Result.(ResultType)
		require.Truef(t, ok, "unexpected result type: %T", event.Result)
		return result
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	panic("this should never be reached")
}

func expectNoEvent(t *testing.T, eventCh <-chan proxycfg.UpdateEvent) {
	select {
	case <-eventCh:
		t.Fatal("expected no event")
	case <-time.After(100 * time.Millisecond):
	}
}
