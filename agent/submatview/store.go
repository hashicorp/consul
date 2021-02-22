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
	// notifier is the count of active Notify goroutines. This entry will
	// remain in the store as long as this count remains > 0.
	notifier int
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

			// Only stop the materializer if there are no active calls to Notify.
			if e.notifier == 0 {
				e.stop()
				delete(s.byKey, he.Key())
			}

			s.lock.Unlock()
		}
	}
}

// TODO: godoc
var idleTTL = 20 * time.Minute

// Get a value from the store, blocking if the store has not yet seen the
// req.Index value.
// See agent/cache.Cache.Get for complete documentation.
func (s *Store) Get(
	ctx context.Context,
	// TODO: remove typ param, make it part of the Request interface.
	typ string,
	req Request,
	// TODO: only the Index field of ResultMeta is relevant, return a result struct instead.
) (interface{}, cache.ResultMeta, error) {
	info := req.CacheInfo()
	e := s.getEntry(getEntryOpts{
		typ:             typ,
		info:            info,
		newMaterializer: req.NewMaterializer,
	})

	// TODO: no longer any need to return cache.FetchResult from Materializer.Fetch
	// TODO: pass context instead of Done chan, also replaces Timeout param
	result, err := e.materializer.Fetch(ctx.Done(), cache.FetchOptions{
		MinIndex: info.MinIndex,
		Timeout:  info.Timeout,
	})
	return result.Value, cache.ResultMeta{Index: result.Index}, err
}

// Notify the updateCh when there are updates to the entry identified by req.
// See agent/cache.Cache.Notify for complete documentation.
func (s *Store) Notify(
	ctx context.Context,
	typ string,
	req Request,
	correlationID string,
	updateCh chan<- cache.UpdateEvent,
) error {
	info := req.CacheInfo()
	e := s.getEntry(getEntryOpts{
		typ:             typ,
		info:            info,
		newMaterializer: req.NewMaterializer,
		notifier:        true,
	})

	go func() {
		index := info.MinIndex

		// TODO: better way to handle this?
		defer func() {
			s.lock.Lock()
			e.notifier--
			s.byKey[e.expiry.Key()] = e
			s.expiryHeap.Update(e.expiry.Index(), idleTTL)
			s.lock.Unlock()
		}()

		for {
			result, err := e.materializer.Fetch(ctx.Done(), cache.FetchOptions{MinIndex: index})
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

func (s *Store) getEntry(opts getEntryOpts) entry {
	info := opts.info
	key := makeEntryKey(opts.typ, info)

	s.lock.Lock()
	defer s.lock.Unlock()
	e, ok := s.byKey[key]
	if ok {
		s.expiryHeap.Update(e.expiry.Index(), info.Timeout+idleTTL)
		if opts.notifier {
			e.notifier++
		}
		return e
	}

	ctx, cancel := context.WithCancel(context.Background())
	mat := opts.newMaterializer()
	go mat.Run(ctx)

	e = entry{
		materializer: mat,
		stop:         cancel,
		expiry:       s.expiryHeap.Add(key, info.Timeout+idleTTL),
	}
	if opts.notifier {
		e.notifier++
	}
	s.byKey[key] = e
	return e
}

// makeEntryKey matches agent/cache.makeEntryKey, but may change in the future.
func makeEntryKey(typ string, r cache.RequestInfo) string {
	return fmt.Sprintf("%s/%s/%s/%s", typ, r.Datacenter, r.Token, r.Key)
}

type Request interface {
	cache.Request
	NewMaterializer() *Materializer
}

type getEntryOpts struct {
	typ             string
	info            cache.RequestInfo
	newMaterializer func() *Materializer
	notifier        bool
}
