package agent

import (
	"bytes"
	"errors"
	"log"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	discover "github.com/hashicorp/go-discover"
	"github.com/stretchr/testify/require"
)

func TestGoDiscoverRegistration(t *testing.T) {
	d, err := discover.New()
	if err != nil {
		t.Fatal(err)
	}
	got := d.Names()
	want := []string{"aliyun", "aws", "azure", "digitalocean", "gce", "os", "packet", "scaleway", "softlayer", "triton", "vsphere"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got go-discover providers %v want %v", got, want)
	}
}

func TestRetryJoinerTrigger(t *testing.T) {
	var attempts uint32
	triggerCh := make(chan struct{}, 1)
	attemptCh := make(chan struct{})

	var logBuf bytes.Buffer

	j := retryJoiner{
		cluster:      "LAN",
		addrs:        []string{"10.10.10.10"}, // Doesn't matter as we have dummy join
		maxAttempts:  2,                       // we only need two
		interval:     time.Hour,               // >> than the expected test duration
		retryTrigger: triggerCh,
		join: func([]string) (int, error) {
			// Notify main thread when we're done
			defer func() { attemptCh <- struct{}{} }()
			old := atomic.SwapUint32(&attempts, 1)
			if old == 0 {
				// Fail the first attempt
				return 0, errors.New("failed to join")
			}
			// Succeed second time to allow joiner loop to exit
			return 1, nil
		},
		logger: log.New(&logBuf, "", log.LstdFlags),
	}

	// Start joiner
	go func() {
		require.NoError(t, j.retryJoin())
	}()

	// Wait until it's made the first attempt and is "waiting"
	select {
	case <-attemptCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out waiting for first attempt")
	}

	// Now trigger another attempt, long before the interval will schedule it
	triggerCh <- struct{}{}

	// And verify the attempt happens pretty soon
	select {
	case <-attemptCh:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("timed out waiting for triggered attempt")
	}

	// Sanity check logs indicate re-trigger
	require.Contains(t, logBuf.String(), "agent: Retry join re-triggered")
}
