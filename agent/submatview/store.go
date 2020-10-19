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
	byKey      map[string]*Materializer
	expiryHeap *ttlcache.ExpiryHeap
}

func NewStore() *Store {
	return &Store{
		byKey:      make(map[string]*Materializer),
		expiryHeap: ttlcache.NewExpiryHeap(),
	}
}

// Get a value from the store, blocking if the store has not yet seen the
// req.Index value.
// See agent/cache.Cache.Get for complete documentation.
func (s *Store) Get(
	ctx context.Context,
	typ string,
	req cache.Request,
) (result interface{}, meta cache.ResultMeta, err error) {
	return nil, cache.ResultMeta{}, nil
}

// Notify the updateCh when there are updates to the entry identified by req.
// See agent/cache.Cache.Notify for complete documentation.
func (s *Store) Notify(
	ctx context.Context,
	typ string,
	req cache.Request,
	correlationID string,
	updateCh chan<- cache.UpdateEvent,
) error {
	return nil
}

func (s *Store) getMaterializer(opts GetOptions) *Materializer {
	// TODO: use makeEntryKey
	var key string

	s.lock.RLock()
	mat, ok := s.byKey[key]
	s.lock.RUnlock()

	if ok {
		return mat
	}

	s.lock.Lock()
	mat, ok = s.byKey[key]
	if !ok {
		mat = opts.NewMaterializer()
		s.byKey[opts.Key] = mat
	}
	s.lock.Unlock()
	return mat
}

// makeEntryKey matches agent/cache.makeEntryKey, but may change in the future.
func makeEntryKey(t, dc, token, key string) string {
	return fmt.Sprintf("%s/%s/%s/%s", t, dc, token, key)
}

type GetOptions struct {
	// TODO: needs to use makeEntryKey
	Key string

	// MinIndex is the index previously seen by the caller. If MinIndex>0 Fetch
	// will not return until the index is >MinIndex, or Timeout is hit.
	MinIndex uint64

	// TODO: maybe remove and use a context deadline.
	Timeout time.Duration

	// NewMaterializer returns a new Materializer to be used if the store does
	// not have one already running for the given key.
	NewMaterializer func() *Materializer
}

type FetchResult struct {
	// Value is the result of the fetch.
	Value interface{}

	// Index is the corresponding index value for this data.
	Index uint64
}
