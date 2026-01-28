// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package leafcert

import (
	"context"
	"errors"

	"github.com/hashicorp/consul/agent/cacheshim"
	"github.com/hashicorp/consul/agent/structs"
)

// NewCachedRootsReader returns a RootsReader that sources data from the agent cache.
func NewCachedRootsReader(cache cacheshim.Cache, dc string) RootsReader {
	return &agentCacheRootsReader{
		cache:      cache,
		datacenter: dc,
	}
}

type agentCacheRootsReader struct {
	cache      cacheshim.Cache
	datacenter string
}

var _ RootsReader = (*agentCacheRootsReader)(nil)

func (r *agentCacheRootsReader) Get() (*structs.IndexedCARoots, error) {
	// Background is fine here because this isn't a blocking query as no index is set.
	// Therefore this will just either be a cache hit or return once the non-blocking query returns.
	rawRoots, _, err := r.cache.Get(context.Background(), cacheshim.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: r.datacenter,
	})
	if err != nil {
		return nil, err
	}
	roots, ok := rawRoots.(*structs.IndexedCARoots)
	if !ok {
		return nil, errors.New("invalid RootCA response type")
	}
	return roots, nil
}

func (r *agentCacheRootsReader) Notify(ctx context.Context, correlationID string, ch chan<- cacheshim.UpdateEvent) error {
	return r.cache.Notify(ctx, cacheshim.ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: r.datacenter,
	}, correlationID, ch)
}
