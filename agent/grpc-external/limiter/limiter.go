// package limiter provides primatives for limiting the number of concurrent
// operations in-flight.
package limiter

import (
	"context"
	"errors"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"

	"golang.org/x/time/rate"
)

// Unlimited can be used to allow an unlimited number of concurrent sessions.
const Unlimited uint32 = 0

// ErrCapacityReached is returned when there is no capacity for additional sessions.
var ErrCapacityReached = errors.New("active session limit reached")

// SessionLimiter is a session-based concurrency limiter, it provides the basis
// of gRPC/xDS load balancing.
//
// Stream handlers obtain a session with BeginSession before they begin serving
// resources - if the server has reached capacity ErrCapacityReached is returned,
// otherwise a Session is returned.
//
// It is the session-holder's responsibility to:
//
//  1. Call End on the session when finished.
//  2. Receive on the session's Terminated channel and exit (e.g. close the gRPC
//     stream) when it is closed.
//
// The maximum number of concurrent sessions is controlled with SetMaxSessions.
// If there are more than the given maximum sessions already in-flight,
// SessionLimiter will drain randomly-selected sessions at a rate controlled
// by SetDrainRateLimit.
type SessionLimiter struct {
	drainLimiter *rate.Limiter

	// max and inFlight are read/written using atomic operations.
	max, inFlight uint32

	// wakeCh is used to trigger the Run loop to start draining excess sessions.
	wakeCh chan struct{}

	// Everything below here is guarded by mu.
	mu           sync.Mutex
	maxSessionID uint64
	sessionIDs   []uint64 // sessionIDs must be sorted so we can binary search it.
	sessions     map[uint64]*session
}

// NewSessionLimiter creates a new SessionLimiter.
func NewSessionLimiter() *SessionLimiter {
	return &SessionLimiter{
		drainLimiter: rate.NewLimiter(rate.Inf, 1),
		max:          Unlimited,
		wakeCh:       make(chan struct{}, 1),
		sessionIDs:   make([]uint64, 0),
		sessions:     make(map[uint64]*session),
	}
}

// Run the SessionLimiter's drain loop, which terminates excess sessions if the
// limit is lowered. It will exit when the given context is canceled or reaches
// its deadline.
func (l *SessionLimiter) Run(ctx context.Context) {
	for {
		select {
		case <-l.wakeCh:
			for {
				if !l.overCapacity() {
					break
				}

				if err := l.drainLimiter.Wait(ctx); err != nil {
					break
				}

				if !l.overCapacity() {
					break
				}

				l.terminateSession()
			}
		case <-ctx.Done():
			return
		}
	}
}

// SetMaxSessions controls the maximum number of concurrent sessions. If it is
// lower, randomly-selected sessions will be drained.
func (l *SessionLimiter) SetMaxSessions(max uint32) {
	atomic.StoreUint32(&l.max, max)

	// Send on wakeCh without blocking if the Run loop is busy. wakeCh has a
	// buffer of 1, so no triggers will be missed.
	select {
	case l.wakeCh <- struct{}{}:
	default:
	}
}

// SetDrainRateLimit controls the rate at which excess sessions will be drained.
func (l *SessionLimiter) SetDrainRateLimit(limit rate.Limit) {
	l.drainLimiter.SetLimit(limit)
}

// BeginSession begins a new session, or returns ErrCapacityReached if the
// concurrent session limit has been reached.
//
// It is the session-holder's responsibility to:
//
//  1. Call End on the session when finished.
//  2. Receive on the session's Terminated channel and exit (e.g. close the gRPC
//     stream) when it is closed.
func (l *SessionLimiter) BeginSession() (Session, error) {
	if !l.hasCapacity() {
		return nil, ErrCapacityReached
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.createSessionLocked(), nil
}

// Note: hasCapacity is *best effort*. As we do not hold l.mu it's possible that:
//
//   - max has changed by the time we compare it to inFlight.
//   - inFlight < max now, but increases before we create a new session.
//
// This is acceptable for our uses, especially because excess sessions will
// eventually be drained.
func (l *SessionLimiter) hasCapacity() bool {
	max := atomic.LoadUint32(&l.max)
	if max == Unlimited {
		return true
	}

	cur := atomic.LoadUint32(&l.inFlight)
	return max > cur
}

// Note: overCapacity is *best effort*. As we do not hold l.mu it's possible that:
//
//   - max has changed by the time we compare it to inFlight.
//   - inFlight > max now, but decreases before we terminate a session.
func (l *SessionLimiter) overCapacity() bool {
	max := atomic.LoadUint32(&l.max)
	if max == Unlimited {
		return false
	}

	cur := atomic.LoadUint32(&l.inFlight)
	return cur > max
}

func (l *SessionLimiter) terminateSession() {
	l.mu.Lock()
	defer l.mu.Unlock()

	idx := rand.Intn(len(l.sessionIDs))
	id := l.sessionIDs[idx]
	l.sessions[id].terminate()
	l.deleteSessionLocked(idx, id)
}

func (l *SessionLimiter) createSessionLocked() *session {
	session := &session{
		l:      l,
		id:     l.maxSessionID,
		termCh: make(chan struct{}),
	}

	l.maxSessionID++
	l.sessionIDs = append(l.sessionIDs, session.id)
	l.sessions[session.id] = session

	atomic.AddUint32(&l.inFlight, 1)

	return session
}

func (l *SessionLimiter) deleteSessionLocked(idx int, id uint64) {
	delete(l.sessions, id)

	// Note: it's important that we preserve the order here (which most allocation
	// free deletion tricks don't) because we binary search the slice.
	l.sessionIDs = append(l.sessionIDs[:idx], l.sessionIDs[idx+1:]...)

	atomic.AddUint32(&l.inFlight, ^uint32(0))
}

func (l *SessionLimiter) deleteSessionWithID(id uint64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	idx := sort.Search(len(l.sessionIDs), func(i int) bool {
		return l.sessionIDs[i] >= id
	})

	if idx == len(l.sessionIDs) || l.sessionIDs[idx] != id {
		// It's possible that we weren't able to find the id because the session has
		// already been deleted. This could be because the session-holder called End
		// more than once, or because the session was drained. In either case there's
		// nothing more to do.
		return
	}

	l.deleteSessionLocked(idx, id)
}

// SessionTerminatedChan is a channel that will be closed to notify session-
// holders that a session has been terminated.
type SessionTerminatedChan <-chan struct{}

// Session allows its holder to perform an operation (e.g. serve a gRPC stream)
// concurrenly with other session-holders. Sessions may be terminated abruptly
// by the SessionLimiter, so it is the responsibility of the holder to receive
// on the Terminated channel and halt the operation when it is closed.
type Session interface {
	// End the session.
	//
	// This MUST be called when the session-holder is done (e.g. the gRPC stream
	// is closed).
	End()

	// Terminated is a channel that is closed when the session is terminated.
	//
	// The session-holder MUST receive on it and exit (e.g. close the gRPC stream)
	// when it is closed.
	Terminated() SessionTerminatedChan
}

type session struct {
	l *SessionLimiter

	id     uint64
	termCh chan struct{}
}

func (s *session) End() { s.l.deleteSessionWithID(s.id) }

func (s *session) Terminated() SessionTerminatedChan { return s.termCh }

func (s *session) terminate() { close(s.termCh) }
