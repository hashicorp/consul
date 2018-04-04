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
	return New(TestRPC(t))
}

// TestCacheGetCh returns a channel that returns the result of the Get call.
// This is useful for testing timing and concurrency with Get calls. Any
// error will be logged, so the result value should always be asserted.
func TestCacheGetCh(t testing.T, c *Cache, typ string, r Request) <-chan interface{} {
	resultCh := make(chan interface{})
	go func() {
		result, err := c.Get(typ, r)
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
	}
}

// TestRequest returns a Request that returns the given cache key and index.
// The Reset method can be called to reset it for custom usage.
func TestRequest(t testing.T, key string, index uint64) *MockRequest {
	req := &MockRequest{}
	req.On("CacheKey").Return(key)
	req.On("CacheMinIndex").Return(index)
	return req
}

// TestRPC returns a mock implementation of the RPC interface.
func TestRPC(t testing.T) *MockRPC {
	// This function is relatively useless but this allows us to perhaps
	// perform some initialization later.
	return &MockRPC{}
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
