package finder

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
)

// Finder finds intentions by a src/dst exact match. There is currently
// no direct API to do this so this struct downloads all intentions and
// caches them once, and searches in-memory for this. For now this works since
// even with a very large number of intentions, the size of the data gzipped
// over HTTP will be relatively small.
//
// The Finder will only downlaod the intentions one time. This struct is
// not expected to be used over a long period of time. Though it may be
// reused multile times, the intentions list is only downloaded once.
type Finder struct {
	// Client is the API client to use for any requests.
	Client *api.Client

	lock sync.Mutex
	ixns []*api.Intention // cached list of intentions
}

// ID returns the intention ID for the given CLI args. An error is returned
// if args is not 1 or 2 elements.
func (f *Finder) IDFromArgs(args []string) (string, error) {
	switch len(args) {
	case 1:
		return args[0], nil

	case 2:
		ixn, err := f.Find(args[0], args[1])
		if err != nil {
			return "", err
		}
		if ixn == nil {
			return "", fmt.Errorf(
				"Intention with source %q and destination %q not found.",
				args[0], args[1])
		}

		return ixn.ID, nil

	default:
		return "", fmt.Errorf("command requires exactly 1 or 2 arguments")
	}
}

// Find finds the intention that matches the given src and dst. This will
// return nil when the result is not found.
func (f *Finder) Find(src, dst string) (*api.Intention, error) {
	src = StripDefaultNS(src)
	dst = StripDefaultNS(dst)

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

// StripDefaultNS strips the default namespace from an argument. For now,
// the API and lookups strip this value from string output so we strip it.
func StripDefaultNS(v string) string {
	if idx := strings.IndexByte(v, '/'); idx > 0 {
		if v[:idx] == api.IntentionDefaultNamespace {
			return v[:idx+1]
		}
	}

	return v
}
