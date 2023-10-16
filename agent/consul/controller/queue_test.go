// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package controller

import (
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/consul/controller/queue"
)

type countingWorkQueue[T queue.ItemType] struct {
	getCounter            uint64
	addCounter            uint64
	addAfterCounter       uint64
	addRateLimitedCounter uint64
	forgetCounter         uint64
	doneCounter           uint64

	inner queue.WorkQueue[T]
}

func newCountingWorkQueue[T queue.ItemType](inner queue.WorkQueue[T]) *countingWorkQueue[T] {
	return &countingWorkQueue[T]{
		inner: inner,
	}
}

func (c *countingWorkQueue[T]) reset() {
	atomic.StoreUint64(&c.getCounter, 0)
	atomic.StoreUint64(&c.addCounter, 0)
	atomic.StoreUint64(&c.addAfterCounter, 0)
	atomic.StoreUint64(&c.addRateLimitedCounter, 0)
	atomic.StoreUint64(&c.forgetCounter, 0)
	atomic.StoreUint64(&c.doneCounter, 0)
}

func (c *countingWorkQueue[T]) requeues() uint64 {
	return c.addAfters() + c.addRateLimiteds()
}

func (c *countingWorkQueue[T]) Get() (item T, shutdown bool) {
	item, err := c.inner.Get()
	atomic.AddUint64(&c.getCounter, 1)
	return item, err
}

func (c *countingWorkQueue[T]) gets() uint64 {
	return atomic.LoadUint64(&c.getCounter)
}

func (c *countingWorkQueue[T]) Add(item T) {
	c.inner.Add(item)
	atomic.AddUint64(&c.addCounter, 1)
}

func (c *countingWorkQueue[T]) adds() uint64 {
	return atomic.LoadUint64(&c.addCounter)
}

func (c *countingWorkQueue[T]) AddAfter(item T, duration time.Duration) {
	c.inner.AddAfter(item, duration)
	atomic.AddUint64(&c.addAfterCounter, 1)
}

func (c *countingWorkQueue[T]) addAfters() uint64 {
	return atomic.LoadUint64(&c.addAfterCounter)
}

func (c *countingWorkQueue[T]) AddRateLimited(item T) {
	c.inner.AddRateLimited(item)
	atomic.AddUint64(&c.addRateLimitedCounter, 1)
}

func (c *countingWorkQueue[T]) addRateLimiteds() uint64 {
	return atomic.LoadUint64(&c.addRateLimitedCounter)
}

func (c *countingWorkQueue[T]) Forget(item T) {
	c.inner.Forget(item)
	atomic.AddUint64(&c.forgetCounter, 1)
}

func (c *countingWorkQueue[T]) forgets() uint64 {
	return atomic.LoadUint64(&c.forgetCounter)
}

func (c *countingWorkQueue[T]) Done(item T) {
	c.inner.Done(item)
	atomic.AddUint64(&c.doneCounter, 1)
}

func (c *countingWorkQueue[T]) dones() uint64 {
	return atomic.LoadUint64(&c.doneCounter)
}
