// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package xds

import (
	"context"
	"sync"
)

// LeafCancels holds the cancel functions for leaf certificates being watched by this controller instance.
type LeafCancels struct {
	sync.Mutex
	// Cancels is a map from a string key constructed from the pbproxystate.LeafReference to a cancel function for the
	// leaf watch.
	Cancels map[string]context.CancelFunc
}

func (l *LeafCancels) Get(key string) (context.CancelFunc, bool) {
	l.Lock()
	defer l.Unlock()
	v, ok := l.Cancels[key]
	return v, ok
}
func (l *LeafCancels) Set(key string, value context.CancelFunc) {
	l.Lock()
	defer l.Unlock()
	l.Cancels[key] = value
}
func (l *LeafCancels) Delete(key string) {
	l.Lock()
	defer l.Unlock()
	delete(l.Cancels, key)
}
