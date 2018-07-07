package cache

import (
	"reflect"
	"time"

	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/mock"
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
	typ := &MockType{}
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
