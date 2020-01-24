package consul

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

type loggerStore struct {
	root hclog.Logger

	// The key/value pairs that we store are (string, hclog.Logger) values
	cache *sync.Map
}

func newLoggerStore(root hclog.Logger) *loggerStore {
	return &loggerStore{
		root:  root,
		cache: &sync.Map{},
	}
}

func (ls *loggerStore) Named(name string) hclog.Logger {
	l, ok := ls.cache.Load(name)
	if !ok {
		l = ls.root.Named(name)
		ls.cache.Store(name, l)
	}

	// We are safe to cast this because this function is the only one with access
	// to the cache
	return l.(hclog.Logger)
}
