package finder

import (
	"sync"

	"github.com/hashicorp/consul/api"
)

// Finder finds intentions by a src/dst exact match. There is currently
// no direct API to do this so this struct downloads all intentions and
// caches them once, and searches in-memory for this. For now this works since
// even with a very large number of intentions, the size of the data gzipped
// over HTTP will be relatively small.
type Finder struct {
	// Client is the API client to use for any requests.
	Client *api.Client

	lock sync.Mutex
	ixns []*api.Intention // cached list of intentions
}

// Find finds the intention that matches the given src and dst. This will
// return nil when the result is not found.
func (f *Finder) Find(src, dst string) (*api.Intention, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	// If the list of ixns is nil, then we haven't fetched yet, so fetch
	if f.ixns == nil {
		ixns, _, err := f.Client.Connect().Intentions(nil)
		if err != nil {
			return nil, err
		}

		f.ixns = ixns
	}

	// Go through the intentions and find an exact match
	for _, ixn := range f.ixns {
		if ixn.SourceString() == src && ixn.DestinationString() == dst {
			return ixn, nil
		}
	}

	return nil, nil
}
