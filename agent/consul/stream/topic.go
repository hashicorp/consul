package stream
/*
import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/singleflight"
)

var (
	ErrCursorNotInBuffer = errors.New("cursor is not in buffer")
)

// TopicBuffer is a component that stores a list of events published to a topic.
type TopicBuffer interface {
	Append([]Event) error
	NextEvent(ctx context.Context, cursor uint64) ([]Event, error)
	Truncate(uint64) error
}

type BufferedEvent struct {
	Event   Event
	Encoded []byte
}

type TopicManager struct {
	buf         TopicBuffer
	snapSF      *singleflight.Group
	snapFunc    SnapFn
	snap        []Event
	snapIndex   uint64
	snapLock    sync.Mutex
	snapDoneCh  chan struct{}
	subsByToken map[string]*Subscription
}

type snapshot struct {
	Index  uint64
	Events []Event
}

func (m *TopicManager) Subscribe(ctx context.Context, req *SubscribeRequest) (*Subscription, error) {
	return &Subscription{}, nil
}

func (m *TopicManager) getSnapshot(ctx context.Context, req *SubscribeRequest) (snapshot, error) {
	// TODO(banks): snapshot caching by key? Requires thought about cache
	// limits/config and eviction policy.

	// Keep trying to get a snapshot as long as our context is good in case the
	// leader's context was cancelled.
	for {
		r := <-m.snapSF.DoChan(req.Key, func() (interface{}, error) {
			// Pass our context here in case we are the only caller and we cancel part
			// way through a huge/expensive snapshot. But note that we might cancel
			// while other callers are also waiting. That's why they retry if their
			// context wasn't cancelled.
			idx, evts, err := m.snapFunc(ctx, req)
			return snapshot{idx, evts}, err
		})
		if r.Err == context.Canceled && ctx.Err() == nil {
			// Someone else's context was cancelled, try again
			continue
		}
		if r.Err != nil {
			return snapshot{}, r.Err
		}
		return r.Val.(snapshot), nil
	}
}

func (m *TopicManager) Publish(evts []Event) error {

	return nil
}

func (m *TopicManager) Commit() error {
	return nil
}
*/