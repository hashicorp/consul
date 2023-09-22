package cachetype

import (
	"reflect"
	"time"

	testinf "github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/cache"
)

// TestRPC returns a mock implementation of the RPC interface.
func TestRPC(t testinf.T) *MockRPC {
	// This function is relatively useless but this allows us to perhaps
	// perform some initialization later.
	return &MockRPC{}
}

// TestFetchCh returns a channel that returns the result of the Fetch call.
// This is useful for testing timing and concurrency with Fetch calls.
// Errors will show up as an error type on the resulting channel so a
// type switch should be used.
func TestFetchCh(
	t testinf.T,
	typ cache.Type,
	opts cache.FetchOptions,
	req cache.Request,
) <-chan interface{} {
	resultCh := make(chan interface{})
	go func() {
		result, err := typ.Fetch(opts, req)
		if err != nil {
			resultCh <- err
			return
		}

		resultCh <- result
	}()

	return resultCh
}

// TestFetchChResult tests that the result from TestFetchCh matches
// within a reasonable period of time (it expects it to be "immediate" but
// waits some milliseconds).
func TestFetchChResult(t testinf.T, ch <-chan interface{}, expected interface{}) {
	t.Helper()

	select {
	case result := <-ch:
		if err, ok := result.(error); ok {
			t.Fatalf("Result was error: %s", err)
			return
		}

		if !reflect.DeepEqual(result, expected) {
			t.Fatalf("Result doesn't match!\n\n%#v\n\n%#v", result, expected)
		}

	case <-time.After(50 * time.Millisecond):
	}
}
