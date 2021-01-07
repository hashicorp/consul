package proxycfg

import (
	"path"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	cachetype "github.com/hashicorp/consul/agent/cache-types"
)

// TestCacheTypes encapsulates all the different cache types proxycfg.State will
// watch/request for controlling one during testing.
type TestCacheTypes struct {
	roots             *ControllableCacheType
	leaf              *ControllableCacheType
	intentions        *ControllableCacheType
	health            *ControllableCacheType
	query             *ControllableCacheType
	compiledChain     *ControllableCacheType
	serviceHTTPChecks *ControllableCacheType
}

// NewTestCacheTypes creates a set of ControllableCacheTypes for all types that
// proxycfg will watch suitable for testing a proxycfg.State or Manager.
func NewTestCacheTypes(t *testing.T) *TestCacheTypes {
	t.Helper()
	ct := &TestCacheTypes{
		roots:             NewControllableCacheType(t),
		leaf:              NewControllableCacheType(t),
		intentions:        NewControllableCacheType(t),
		health:            NewControllableCacheType(t),
		query:             NewControllableCacheType(t),
		compiledChain:     NewControllableCacheType(t),
		serviceHTTPChecks: NewControllableCacheType(t),
	}
	ct.query.blocking = false
	return ct
}

// testCacheWithTypes registers ControllableCacheTypes for all types that
// proxycfg will watch suitable for testing a proxycfg.State or Manager.
func testCacheWithTypes(t *testing.T, types *TestCacheTypes) *cache.Cache {
	c := cache.New(cache.Options{})
	c.RegisterType(cachetype.ConnectCARootName, types.roots)
	c.RegisterType(cachetype.ConnectCALeafName, types.leaf)
	c.RegisterType(cachetype.IntentionMatchName, types.intentions)
	c.RegisterType(cachetype.HealthServicesName, types.health)
	c.RegisterType(cachetype.PreparedQueryName, types.query)
	c.RegisterType(cachetype.CompiledDiscoveryChainName, types.compiledChain)
	c.RegisterType(cachetype.ServiceHTTPChecksName, types.serviceHTTPChecks)

	return c
}

// ControllableCacheType is a cache.Type that simulates a typical blocking RPC
// but lets us control the responses and when they are delivered easily.
type ControllableCacheType struct {
	index uint64
	value sync.Map
	// Need a condvar to trigger all blocking requests (there might be multiple
	// for same type due to background refresh and timing issues) when values
	// change. Chans make it nondeterministic which one triggers or need extra
	// locking to coordinate replacing after close etc.
	triggerMu sync.Mutex
	trigger   *sync.Cond
	blocking  bool
	lastReq   atomic.Value
}

// NewControllableCacheType returns a cache.Type that can be controlled for
// testing.
func NewControllableCacheType(t *testing.T) *ControllableCacheType {
	c := &ControllableCacheType{
		index:    5,
		blocking: true,
	}
	c.trigger = sync.NewCond(&c.triggerMu)
	return c
}

// Set sets the response value to be returned from subsequent cache gets for the
// type.
func (ct *ControllableCacheType) Set(key string, value interface{}) {
	atomic.AddUint64(&ct.index, 1)
	ct.value.Store(key, value)
	ct.triggerMu.Lock()
	ct.trigger.Broadcast()
	ct.triggerMu.Unlock()
}

// Fetch implements cache.Type. It simulates blocking or non-blocking queries.
func (ct *ControllableCacheType) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	index := atomic.LoadUint64(&ct.index)

	ct.lastReq.Store(req)

	shouldBlock := ct.blocking && opts.MinIndex > 0 && opts.MinIndex == index
	if shouldBlock {
		// Wait for return to be triggered. We ignore timeouts based on opts.Timeout
		// since in practice they will always be way longer than our tests run for
		// and the caller can simulate timeout by triggering return without changing
		// index or value.
		ct.triggerMu.Lock()
		ct.trigger.Wait()
		ct.triggerMu.Unlock()
	}

	info := req.CacheInfo()
	key := path.Join(info.Key, info.Datacenter) // omit token for testing purposes

	// reload index as it probably got bumped
	index = atomic.LoadUint64(&ct.index)
	val, _ := ct.value.Load(key)

	if err, ok := val.(error); ok {
		return cache.FetchResult{
			Value: nil,
			Index: index,
		}, err
	}
	return cache.FetchResult{
		Value: val,
		Index: index,
	}, nil
}

func (ct *ControllableCacheType) RegisterOptions() cache.RegisterOptions {
	return cache.RegisterOptions{
		Refresh:          ct.blocking,
		SupportsBlocking: ct.blocking,
		QueryTimeout:     10 * time.Minute,
	}
}
