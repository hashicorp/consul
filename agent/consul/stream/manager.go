package stream

/*
import (
	context "context"
	fmt "fmt"
	"log"
	"sync"

	"github.com/hashicorp/go-memdb"
)

// SnapFn is the type of function that will make a snapshot for a given topic.
type SnapFn func(ctx context.Context, req *SubscribeRequest) (uint64, []Event, error)

// Manager implements the core of the pubsub mechanism behind streaming. Events
// are published to the manager by the state store and subscriptionsare managed.
type Manager struct {
	logger *log.Logger

	snapFns map[Topic]SnapFn

	tms    map[Topic]map[string]TopicManager
	tmLock sync.Mutex

	staged     []Event
	stagedLock sync.Mutex
	commitCh   chan commitEvents
}

type commitEvents struct {
	tx   *memdb.Txn
	evts []Event
}

// NewManager creates a manager instance
func NewManager(logger *log.Logger, snapFns map[Topic]SnapFn) *Manager {
	return &Manager{
		logger:  logger,
		snapFns: snapFns,
	}
}

// PreparePublish accepts a set of events to be published. All events must have
// the same index which must be higher than any previously Committed events.
// Events will not be delivered to subscribers until Commit is called. If
// PreparePublish is called a second time before Commit, and the events have a
// higher index, it is assumed the previous set of events were aborted and they
// will be dropped. If the events in a subsequent call have the same index they
// will be staged in addition to those already prepared and all will be
// committed on next Commit.
func (m *Manager) PreparePublish(evts []Event) error {
	m.stagedLock.Lock()
	defer m.stagedLock.Unlock()

	if len(m.staged) > 0 {
		if m.staged[0].Index != evts[0].Index {
			// implicit abort - these events are from a different index, replace
			// current staged events.
			m.staged = evts
		} else {
			// Merge events at same index
			m.staged = append(m.staged, evts...)
		}
	} else {
		m.staged = evts
	}
	return nil
}

// Commit flushes any prepared messages to all relevant topic buffers.
func (m *Manager) Commit(db *memdb.MemDB) error {
	m.stagedLock.Lock()
	defer m.stagedLock.Unlock()

	if len(m.staged) == 0 {
		return nil
	}

	ce := commitEvents{
		tx:   db.Txn(false),
		evts: m.staged,
	}
	m.commitCh <- ce

	return nil
}

func (m *Manager) Subscribe(ctx context.Context, req *SubscribeRequest) (*Subscription, error) {

	// Make sure this is a valid topic
	snapFn, ok := m.snapFns[req.Topic]
	if !ok {
		return nil, fmt.Errorf("no snapshot function registered for topic %s",
			req.Topic.String())
	}

	// Find the TM for this request
	m.tmLock.Lock()
	byKey, ok := m.tms[req.Topic]
	if !ok {
		byKey := make(map[string]TopicManager)
		m.tms[req.Topic] = byKey
	}
	tm, ok := byKey[req.Key]
	if !ok {
		tm = TopicManager{
			snapFunc: snapFn,
		}
		byKey[req.Key] = tm
	}
	m.tmLock.Unlock()

	// Subscribe to the topic manager
	return tm.Subscribe(ctx, req)
}
*/
