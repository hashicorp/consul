package lib

import (
	"sync/atomic"
)

type AtomicBool int32

func NewAtomicBool(value bool) *AtomicBool {
	ab := new(AtomicBool)
	ab.Set(value)
	return ab
}

func (ab *AtomicBool) Set(value bool) {
	if value {
		atomic.StoreInt32((*int32)(ab), 1)
	} else {
		atomic.StoreInt32((*int32)(ab), 0)
	}
}

func (ab *AtomicBool) IsSet() bool {
	return atomic.LoadInt32((*int32)(ab)) == 1
}
