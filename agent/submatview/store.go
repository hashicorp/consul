package submatview

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib/ttlcache"
)

// Store of Materializers. Store implements an interface similar to
// agent/cache.Cache, and allows a single Materializer to fulfil multiple requests
// as long as the requests are identical.
// Store is used in place of agent/cache.Cache because with the streaming
// backend there is no longer any need to run a background goroutine to refresh
// stored values.
type Store struct {
	logger hclog.Logger
	lock   sync.RWMutex
	byKey  map[string]entry

	// expiryHeap tracks entries with 0 remaining requests. Entries are ordered
	// by most recent expiry first.
	expiryHeap *ttlcache.ExpiryHeap

	// idleTTL is the duration of time an entry should remain in the Store after the
	// last request for that entry has been terminated. It is a field on the struct
	// so that it can be patched in tests without needing a global lock.
	idleTTL time.Duration
}

// A Materializer maintains a materialized view of a subscription on an event stream.
type Materializer interface {
	Run(ctx context.Context)
	Query(ctx context.Context, minIndex uint64) (Result, error)
}

type entry struct {
	materializer Materializer
	expiry       *ttlcache.Entry
	stop         func()
	// requests is the count of active requests using this entry. This entry will
	// remain in the store as long as this count remains > 0.
	requests int
	// evicting is used to mark an entry that will be evicted when the current in-
	// flight requests finish.
	evicting bool
}

// NewStore creates and returns a Store that is ready for use. The caller must
// call Store.Run (likely in a separate goroutine) to start the expiration loop.
func NewStore(logger hclog.Logger) *Store {
	return &Store{
		logger:     logger,
		byKey:      make(map[string]entry),
		expiryHeap: ttlcache.NewExpiryHeap(),
		idleTTL:    20 * time.Minute,
	}
}

// Run the expiration loop until the context is cancelled.
func (s *Store) Run(ctx context.Context) {
	for {
		s.lock.RLock()
		timer := s.expiryHeap.Next()
		s.lock.RUnlock()

		select {
		case <-ctx.Done():
			timer.Stop()
			return

		// the first item in the heap has changed, restart the timer with the
		// new TTL.
		case <-s.expiryHeap.NotifyCh:
			timer.Stop()
			continue

		// the TTL for the first item has been reached, attempt an expiration.
		case <-timer.Wait():
			s.lock.Lock()

			he := timer.Entry
			s.expiryHeap.Remove(he.Index())

			e := s.byKey[he.Key()]

			// Only stop the materializer if there are no active requests.
			if e.requests == 0 {
				s.logger.Trace("evicting item from store", "key", he.Key())
				e.stop()
				delete(s.byKey, he.Key())
			}

			s.lock.Unlock()
		}
	}
}

// Request is used to request data from the Store.
// Note that cache.Request is required, but some of the fields cache.RequestInfo
// fields are ignored (ex: MaxAge, and MustRevalidate).
type Request interface {
	cache.Request
	// NewMaterializer will be called if there is no active materializer to fulfil
	// the request. It should return a Materializer appropriate for streaming
	// data to fulfil this request.
	NewMaterializer() (Materializer, error)
	// Type should return a string which uniquely identifies this type of request.
	// The returned value is used as the prefix of the key used to index
	// entries in the Store.
	Type() string
}

// Get a value from the store, blocking if the store has not yet seen the
// req.Index value.
// See agent/cache.Cache.Get for complete documentation.
func (s *Store) Get(ctx context.Context, req Request) (Result, error) {
	info := req.CacheInfo()
	key, materializer, err := s.readEntry(req)
	if err != nil {
		return Result{}, err
	}
	defer s.releaseEntry(key)

	if info.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, info.Timeout)
		defer cancel()
	}

	result, err := materializer.Query(ctx, info.MinIndex)
	// context.DeadlineExceeded is translated to nil to match the timeout
	// behaviour of agent/cache.Cache.Get.
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		return result, nil
	}
	return result, err
}

// Notify the updateCh when there are updates to the entry identified by req.
// See agent/cache.Cache.Notify for complete documentation.
//
// Request.CacheInfo().Timeout is ignored because it is not really relevant in
// this case. Instead set a deadline on the context.
func (s *Store) Notify(
	ctx context.Context,
	req Request,
	correlationID string,
	updateCh chan<- cache.UpdateEvent,
) error {
	return s.NotifyCallback(ctx, req, correlationID, func(ctx context.Context, event cache.UpdateEvent) {
		select {
		case updateCh <- event:
		case <-ctx.Done():
			return
		}
	})
}

// NotifyCallback subscribes to updates of the entry identified by req in the
// same way as Notify, but accepts a callback function instead of a channel.
func (s *Store) NotifyCallback(
	ctx context.Context,
	req Request,
	correlationID string,
	cb cache.Callback,
) error {
	info := req.CacheInfo()
	key, materializer, err := s.readEntry(req)
	if err != nil {
		return err
	}

	go func() {
		defer s.releaseEntry(key)

		index := info.MinIndex
		for {
			result, err := materializer.Query(ctx, index)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				s.logger.Warn("handling error in Store.Notify",
					"error", err,
					"request-type", req.Type(),
					"index", index)
			}

			index = result.Index
			cb(ctx, cache.UpdateEvent{
				CorrelationID: correlationID,
				Result:        result.Value,
				Err:           err,
				Meta:          cache.ResultMeta{Index: result.Index, Hit: result.Cached},
			})
		}
	}()
	return nil
}

// readEntry from the store, and increment the requests counter. releaseEntry
// must be called when the request is finished to decrement the counter.
func (s *Store) readEntry(req Request) (string, Materializer, error) {
	info := req.CacheInfo()
	key := makeEntryKey(req.Type(), info)

	s.lock.Lock()
	defer s.lock.Unlock()
	e, ok := s.byKey[key]
	if ok {
		if e.evicting {
			return "", nil, errors.New("item is marked for eviction")
		}
		e.requests++
		s.byKey[key] = e
		return key, e.materializer, nil
	}

	mat, err := req.NewMaterializer()
	if err != nil {
		return "", nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		mat.Run(ctx)

		// Materializers run until they either reach their TTL and are evicted (which
		// cancels the given context) or encounter an irrecoverable error.
		//
		// If the context hasn't been canceled, we know it's the error case so we
		// trigger an immediate eviction.
		if ctx.Err() == nil {
			s.evictNow(key)
		}
	}()

	e = entry{
		materializer: mat,
		stop:         cancel,
		requests:     1,
	}
	s.byKey[key] = e
	return key, e.materializer, nil
}

// evictNow causes the item with the given key to be evicted immediately.
//
// If there are requests in-flight, the item is marked for eviction such that
// once the requests have been served releaseEntry will move it to the top of
// the expiry heap. If there are no requests in-flight, evictNow will move the
// item to the top of the expiry heap itself.
//
// In either case, the entry's evicting flag prevents it from being served by
// readEntry (and thereby gaining new in-flight requests).
func (s *Store) evictNow(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	e := s.byKey[key]
	e.evicting = true
	s.byKey[key] = e

	if e.requests == 0 {
		s.expireNowLocked(key)
	}
}

// releaseEntry decrements the request count and starts an expiry timer if the
// count has reached 0. Must be called once for every call to readEntry.
func (s *Store) releaseEntry(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	e := s.byKey[key]
	e.requests--
	s.byKey[key] = e

	if e.requests > 0 {
		return
	}

	if e.evicting {
		s.expireNowLocked(key)
		return
	}

	if e.expiry.Index() == ttlcache.NotIndexed {
		e.expiry = s.expiryHeap.Add(key, s.idleTTL)
		s.byKey[key] = e
		return
	}

	s.expiryHeap.Update(e.expiry.Index(), s.idleTTL)
}

// expireNowLocked moves the item with the given key to the top of the expiry
// heap, causing it to be picked up by the expiry loop and evicted immediately.
func (s *Store) expireNowLocked(key string) {
	e := s.byKey[key]
	if idx := e.expiry.Index(); idx != ttlcache.NotIndexed {
		s.expiryHeap.Remove(idx)
	}
	e.expiry = s.expiryHeap.Add(key, time.Duration(0))
	s.byKey[key] = e
}

// makeEntryKey matches agent/cache.makeEntryKey, but may change in the future.
func makeEntryKey(typ string, r cache.RequestInfo) string {
	return fmt.Sprintf("%s/%s/%s/%s", typ, r.Datacenter, r.Token, r.Key)
}
