package cache

import (
	"reflect"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestCache returns a Cache instance configuring for testing.
func TestCache(t testing.T) *Cache {
	// Simple but lets us do some fine-tuning later if we want to.
	return New(nil)
}

// TestCacheGetCh returns a channel that returns the result of the Get call.
// This is useful for testing timing and concurrency with Get calls. Any
// error will be logged, so the result value should always be asserted.
func TestCacheGetCh(t testing.T, c *Cache, typ string, r Request) <-chan interface{} {
	resultCh := make(chan interface{})
	go func() {
		result, _, err := c.Get(typ, r)
		if err != nil {
			t.Logf("Error: %s", err)
			close(resultCh)
			return
		}

		resultCh <- result
	}()

	return resultCh
}

// TestCacheGetChResult tests that the result from TestCacheGetCh matches
// within a reasonable period of time (it expects it to be "immediate" but
// waits some milliseconds).
func TestCacheGetChResult(t testing.T, ch <-chan interface{}, expected interface{}) {
	t.Helper()

	select {
	case result := <-ch:
		if !reflect.DeepEqual(result, expected) {
			t.Fatalf("Result doesn't match!\n\n%#v\n\n%#v", result, expected)
		}

	case <-time.After(50 * time.Millisecond):
		t.Fatalf("Result not sent on channel")
	}
}

// TestCacheNotifyChResult tests that the expected updated was delivered on a
// Notify() chan within a reasonable period of time (it expects it to be
// "immediate" but waits some milliseconds). Expected may be given multiple
// times and if so these are all waited for and asserted to match but IN ANY
// ORDER to ensure we aren't timing dependent.
func TestCacheNotifyChResult(t testing.T, ch <-chan UpdateEvent, expected ...UpdateEvent) {
	t.Helper()

	expectLen := len(expected)
	if expectLen < 1 {
		panic("asserting nothing")
	}

	got := make([]UpdateEvent, 0, expectLen)
	timeoutCh := time.After(50 * time.Millisecond)

OUT:
	for {
		select {
		case result := <-ch:
			// Ignore age as it's non-deterministic
			result.Meta.Age = 0
			got = append(got, result)
			if len(got) == expectLen {
				break OUT
			}

		case <-timeoutCh:
			t.Fatalf("got %d results on chan in 50ms, want %d", len(got), expectLen)
		}
	}

	// We already asserted len since you can only get here if we appended enough.
	// Just check all the results we got are in the expected slice
	require.ElementsMatch(t, expected, got)
}

// TestRequest returns a Request that returns the given cache key and index.
// The Reset method can be called to reset it for custom usage.
func TestRequest(t testing.T, info RequestInfo) *MockRequest {
	req := &MockRequest{}
	req.On("CacheInfo").Return(info)
	return req
}

// TestType returns a MockType that can be used to setup expectations
// on data fetching.
func TestType(t testing.T) *MockType {
	return testTypeInternal(t, true)
}

// TestTypeNonBlocking returns a MockType that returns false to SupportsBlocking.
func TestTypeNonBlocking(t testing.T) *MockType {
	return testTypeInternal(t, false)
}

func testTypeInternal(t testing.T, enableBlocking bool) *MockType {
	typ := &MockType{}
	typ.On("SupportsBlocking").Return(enableBlocking).Maybe()
	return typ
}

// A bit weird, but we add methods to the auto-generated structs here so that
// they don't get clobbered. The helper methods are conveniences.

// Static sets a static value to return for a call to Fetch.
func (m *MockType) Static(r FetchResult, err error) *mock.Call {
	return m.Mock.On("Fetch", mock.Anything, mock.Anything).Return(r, err)
}

func (m *MockRequest) Reset() {
	m.Mock = mock.Mock{}
}
