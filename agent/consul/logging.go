// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"sync"

	"github.com/hashicorp/go-hclog"
)

type loggerStore struct {
	root  hclog.Logger
	l     sync.Mutex
	cache map[string]hclog.Logger
}

func newLoggerStore(root hclog.Logger) *loggerStore {
	return &loggerStore{
		root:  root,
		cache: make(map[string]hclog.Logger),
	}
}

func (ls *loggerStore) Named(name string) hclog.Logger {
	ls.l.Lock()
	defer ls.l.Unlock()
	l, ok := ls.cache[name]
	if !ok {
		l = ls.root.Named(name)
		ls.cache[name] = l
	}
	return l
}
