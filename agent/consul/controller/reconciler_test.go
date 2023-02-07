package controller

import (
	"context"
	"sync"
	"time"
)

type testReconciler struct {
	received chan Request
	response error
	mutex    sync.Mutex
	stepChan chan struct{}
	stopChan chan struct{}
	ctx      context.Context
}

func (r *testReconciler) Reconcile(ctx context.Context, req Request) error {
	if r.stepChan != nil {
		select {
		case <-r.stopChan:
			return nil
		case <-r.stepChan:
		}
	}

	select {
	case <-r.stopChan:
		return nil
	case r.received <- req:
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.response
}

func (r *testReconciler) setResponse(err error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.response = err
}

func (r *testReconciler) step() {
	r.stepChan <- struct{}{}
}
func (r *testReconciler) stepFor(duration time.Duration) {
	select {
	case r.stepChan <- struct{}{}:
	case <-time.After(duration):
	}
}

func (r *testReconciler) stop() {
	close(r.stopChan)
}

func newTestReconciler(stepping bool) *testReconciler {
	r := &testReconciler{
		received: make(chan Request, 1000),
		stopChan: make(chan struct{}),
	}
	if stepping {
		r.stepChan = make(chan struct{})
	}

	return r
}
