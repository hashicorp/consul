package proto

import (
	"github.com/inconshreveable/muxado/proto/frame"
	"sync"
)

const (
	initMapCapacity = 128 // not too much extra memory wasted to avoid allocations
)

type StreamMap interface {
	Get(frame.StreamId) (stream, bool)
	Set(frame.StreamId, stream)
	Delete(frame.StreamId)
	Each(func(frame.StreamId, stream))
}

// ConcurrentStreamMap is a map of stream ids -> streams guarded by a read/write lock
type ConcurrentStreamMap struct {
	sync.RWMutex
	table map[frame.StreamId]stream
}

func (m *ConcurrentStreamMap) Get(id frame.StreamId) (s stream, ok bool) {
	m.RLock()
	s, ok = m.table[id]
	m.RUnlock()
	return
}

func (m *ConcurrentStreamMap) Set(id frame.StreamId, str stream) {
	m.Lock()
	m.table[id] = str
	m.Unlock()
}

func (m *ConcurrentStreamMap) Delete(id frame.StreamId) {
	m.Lock()
	delete(m.table, id)
	m.Unlock()
}

func (m *ConcurrentStreamMap) Each(fn func(frame.StreamId, stream)) {
	m.Lock()
	streams := make(map[frame.StreamId]stream, len(m.table))
	for k, v := range m.table {
		streams[k] = v
	}
	m.Unlock()

	for id, str := range streams {
		fn(id, str)
	}
}

func NewConcurrentStreamMap() *ConcurrentStreamMap {
	return &ConcurrentStreamMap{table: make(map[frame.StreamId]stream, initMapCapacity)}
}
