package submatview

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/lib/ttlcache"
)

type Store struct {
	lock       sync.RWMutex
	byKey      map[string]entry
	expiryHeap *ttlcache.ExpiryHeap
}

type entry struct {
	materializer *Materializer
	expiry       *ttlcache.Entry
	stop         func()
	// requests is the count of active requests using this entry. This entry will
	// remain in the store as long as this count remains > 0.
	requests int
}

// TODO: start expiration loop
func NewStore() *Store {
	return &Store{
		byKey:      make(map[string]entry),
		expiryHeap: ttlcache.NewExpiryHeap(),
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
		case <-s.expiryHeap.NotifyCh:
			timer.Stop()
			continue

		case <-timer.Wait():
			s.lock.Lock()

			he := timer.Entry
			s.expiryHeap.Remove(he.Index())

			e := s.byKey[he.Key()]

			// Only stop the materializer if there are no active requests.
			if e.requests == 0 {
				e.stop()
				delete(s.byKey, he.Key())
			}

			s.lock.Unlock()
		}
	}
}

// TODO: godoc
type Request interface {
	cache.Request
	NewMaterializer() *Materializer
	Type() string
}

// Get a value from the store, blocking if the store has not yet seen the
// req.Index value.
// See agent/cache.Cache.Get for complete documentation.
func (s *Store) Get(
	ctx context.Context,
	req Request,
	// TODO: only the Index field of ResultMeta is relevant, return a result struct instead.
) (interface{}, cache.ResultMeta, error) {
	info := req.CacheInfo()
	key, e := s.getEntry(req)
	defer s.releaseEntry(key)

	ctx, cancel := context.WithTimeout(ctx, info.Timeout)
	defer cancel()

	result, err := e.materializer.getFromView(ctx, info.MinIndex)

	// TODO: does context.DeadlineExceeded need to be translated into a nil error
	// to match the old interface?

	return result.Value, cache.ResultMeta{Index: result.Index}, err
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
	info := req.CacheInfo()
	key, e := s.getEntry(req)

	go func() {
		defer s.releaseEntry(key)

		index := info.MinIndex
		for {
			result, err := e.materializer.getFromView(ctx, index)
			switch {
			case ctx.Err() != nil:
				return
			case err != nil:
				// TODO: cache.Notify sends errors on updateCh, should this do the same?
				// It seems like only fetch errors would ever get sent along.
				// TODO: log warning
				continue
			}

			index = result.Index
			u := cache.UpdateEvent{
				CorrelationID: correlationID,
				Result:        result.Value,
				Meta:          cache.ResultMeta{Index: result.Index},
				Err:           err,
			}
			select {
			case updateCh <- u:
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

// getEntry from the store, and increment the requests counter. releaseEntry
// must be called when the request is finished to decrement the counter.
func (s *Store) getEntry(req Request) (string, entry) {
	info := req.CacheInfo()
	key := makeEntryKey(req.Type(), info)

	s.lock.Lock()
	defer s.lock.Unlock()
	e, ok := s.byKey[key]
	if ok {
		e.requests++
		s.byKey[key] = e
		return key, e
	}

	ctx, cancel := context.WithCancel(context.Background())
	mat := req.NewMaterializer()
	go mat.Run(ctx)

	e = entry{
		materializer: mat,
		stop:         cancel,
		requests:     1,
	}
	s.byKey[key] = e
	return key, e
}

// idleTTL is the duration of time an entry should remain in the Store after the
// last request for that entry has been terminated.
var idleTTL = 20 * time.Minute

// releaseEntry decrements the request count and starts an expiry timer if the
// count has reached 0. Must be called once for every call to getEntry.
func (s *Store) releaseEntry(key string) {
	s.lock.Lock()
	defer s.lock.Unlock()
	e := s.byKey[key]
	e.requests--
	s.byKey[key] = e

	if e.requests > 0 {
		return
	}

	if e.expiry.Index() == ttlcache.NotIndexed {
		e.expiry = s.expiryHeap.Add(key, idleTTL)
		s.byKey[key] = e
		return
	}

	s.expiryHeap.Update(e.expiry.Index(), idleTTL)
}

// makeEntryKey matches agent/cache.makeEntryKey, but may change in the future.
func makeEntryKey(typ string, r cache.RequestInfo) string {
	return fmt.Sprintf("%s/%s/%s/%s", typ, r.Datacenter, r.Token, r.Key)
}
