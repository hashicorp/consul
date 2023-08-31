package controller

import (
	"sync/atomic"
	"time"
)

var _ WorkQueue = &countingWorkQueue{}

type countingWorkQueue struct {
	getCounter            uint64
	addCounter            uint64
	addAfterCounter       uint64
	addRateLimitedCounter uint64
	forgetCounter         uint64
	doneCounter           uint64

	inner WorkQueue
}

func newCountingWorkQueue(inner WorkQueue) *countingWorkQueue {
	return &countingWorkQueue{
		inner: inner,
	}
}

func (c *countingWorkQueue) reset() {
	atomic.StoreUint64(&c.getCounter, 0)
	atomic.StoreUint64(&c.addCounter, 0)
	atomic.StoreUint64(&c.addAfterCounter, 0)
	atomic.StoreUint64(&c.addRateLimitedCounter, 0)
	atomic.StoreUint64(&c.forgetCounter, 0)
	atomic.StoreUint64(&c.doneCounter, 0)
}

func (c *countingWorkQueue) requeues() uint64 {
	return c.addAfters() + c.addRateLimiteds()
}

func (c *countingWorkQueue) Get() (item Request, shutdown bool) {
	item, err := c.inner.Get()
	atomic.AddUint64(&c.getCounter, 1)
	return item, err
}

func (c *countingWorkQueue) gets() uint64 {
	return atomic.LoadUint64(&c.getCounter)
}

func (c *countingWorkQueue) Add(item Request) {
	c.inner.Add(item)
	atomic.AddUint64(&c.addCounter, 1)
}

func (c *countingWorkQueue) adds() uint64 {
	return atomic.LoadUint64(&c.addCounter)
}

func (c *countingWorkQueue) AddAfter(item Request, duration time.Duration) {
	c.inner.AddAfter(item, duration)
	atomic.AddUint64(&c.addAfterCounter, 1)
}

func (c *countingWorkQueue) addAfters() uint64 {
	return atomic.LoadUint64(&c.addAfterCounter)
}

func (c *countingWorkQueue) AddRateLimited(item Request) {
	c.inner.AddRateLimited(item)
	atomic.AddUint64(&c.addRateLimitedCounter, 1)
}

func (c *countingWorkQueue) addRateLimiteds() uint64 {
	return atomic.LoadUint64(&c.addRateLimitedCounter)
}

func (c *countingWorkQueue) Forget(item Request) {
	c.inner.Forget(item)
	atomic.AddUint64(&c.forgetCounter, 1)
}

func (c *countingWorkQueue) forgets() uint64 {
	return atomic.LoadUint64(&c.forgetCounter)
}

func (c *countingWorkQueue) Done(item Request) {
	c.inner.Done(item)
	atomic.AddUint64(&c.doneCounter, 1)
}

func (c *countingWorkQueue) dones() uint64 {
	return atomic.LoadUint64(&c.doneCounter)
}
