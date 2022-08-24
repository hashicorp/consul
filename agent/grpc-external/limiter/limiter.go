// package limiter provides primatives for limiting the number of concurrent
// operations in-flight.
package limiter

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

// Unlimited can be used to allow an unlimited number of concurrent sessions.
const Unlimited uint32 = 0

// ErrCapacityReached is returned when there is no capacity for additional sessions.
var ErrCapacityReached = errors.New("active session limit reached")

// Limiter is a session-based concurrency limiter, it provides the basis of
// gRPC/xDS load balancing.
//
// Stream handlers obtain a session with BeginSession before they begin serving
// resources - if the server has reached capacity ErrCapacityReached is returned,
// otherwise a Session is returned.
//
// It is the session-holder's responsibility to:
//
//	1. Call End on the session when finished.
//	2. Receive on the session's Terminated channel and exit (e.g. close the gRPC
//	   stream) when it is closed.
//
// The maximum number of concurrent sessions is controlled with SetMaxSessions.
// If there are more than the given maximum sessions already in-flight, Limiter
// will terminate randomly-selected sessions at a rate controlled by the
// termLimiter given in NewLimiter.
type Limiter struct {
	termLimiter Waiter

	// max and inFlight are read/written using atomic operations.
	max, inFlight uint32

	// wakeCh is used to trigger the Run loop to start terminating excess sessions.
	wakeCh chan struct{}

	// Everything below here is guarded by mu.
	mu           sync.Mutex
	maxSessionID uint64
	sessionIDs   []uint64
	sessions     map[uint64]*Session
}

// Waiter is responsible for controlling the rate at which excess sessions will
// be terminated.
type Waiter interface {
	Wait(ctx context.Context) error
}

// NewLimiter creates a new Limiter.
//
// termLimiter is used to control the rate at which excess sessions will be
// terminated.
func NewLimiter(termLimiter Waiter) *Limiter {
	return &Limiter{
		termLimiter: termLimiter,
		max:         Unlimited,
		wakeCh:      make(chan struct{}, 1),
		sessionIDs:  make([]uint64, 0),
		sessions:    make(map[uint64]*Session),
	}
}

// Run the Limiter's termination loop, which terminates excess sessions if the
// limit is lowered. It will exit when the given context is canceled or reaches
// its deadline.
func (l *Limiter) Run(ctx context.Context) {
	for {
		select {
		case <-l.wakeCh:
			for {
				if !l.overCapacity() {
					break
				}

				if err := l.termLimiter.Wait(ctx); err != nil {
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
// lower, randomly-selected sessions will be terminated.
func (l *Limiter) SetMaxSessions(max uint32) {
	atomic.StoreUint32(&l.max, max)

	// Send on wakeCh without blocking if the Run loop is busy. wakeCh has a
	// buffer of 1, so no triggers will be missed.
	select {
	case l.wakeCh <- struct{}{}:
	default:
	}
}

// BeginSession begins a new session, or returns ErrCapacityReached if the
// concurrent session limit has been reached.
//
// It is the session-holder's responsibility to:
//
//	1. Call End on the session when finished.
//	2. Receive on the session's Terminated channel and exit (e.g. close the gRPC
//	   stream) when it is closed.
func (l *Limiter) BeginSession() (*Session, error) {
	if !l.hasCapacity() {
		return nil, ErrCapacityReached
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.createSessionLocked(), nil
}

// Note: hasCapacity is *best effort*. As we do not hold l.mu it's possible that:
//
//	- max has changed by the time we compare it to inFlight.
//	- inFlight < max now, but increases before we create a new session.
//
// This is acceptable for our uses, especially because excess sessions will
// eventually get terminated.
func (l *Limiter) hasCapacity() bool {
	max := atomic.LoadUint32(&l.max)
	if max == Unlimited {
		return true
	}

	cur := atomic.LoadUint32(&l.inFlight)
	return max > cur
}

// Note: overCapacity is *best effort*. As we do not hold l.mu it's possible that:
//
//	- max has changed by the time we compare it to inFlight.
//	- inFlight > max now, but decreases before we terminate a session.
func (l *Limiter) overCapacity() bool {
	max := atomic.LoadUint32(&l.max)
	if max == Unlimited {
		return false
	}

	cur := atomic.LoadUint32(&l.inFlight)
	return cur > max
}

func (l *Limiter) terminateSession() {
	l.mu.Lock()
	defer l.mu.Unlock()

	idx := rand.Intn(len(l.sessionIDs))
	id := l.sessionIDs[idx]
	l.sessions[id].terminate()
	l.deleteSessionLocked(idx)
}

func (l *Limiter) createSessionLocked() *Session {
	session := &Session{
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

func (l *Limiter) deleteSessionLocked(idx int) {
	delete(l.sessions, l.sessionIDs[idx])

	l.sessionIDs[idx] = l.sessionIDs[len(l.sessionIDs)-1]
	l.sessionIDs = l.sessionIDs[:len(l.sessionIDs)-1]

	atomic.AddUint32(&l.inFlight, ^uint32(0))
}

// Session allows its holder to perform an operation (e.g. serve a gRPC stream)
// concurrenly with other session-holders. Sessions may be terminated abruptly
// by the Limiter, so it is the responsibility of the holder to receive on the
// Terminated channel and halt the operation when it is closed.
type Session struct {
	l *Limiter

	id     uint64
	termCh chan struct{}

	// done ensures that if both End and terminate are called on a session, only
	// the first call will have effect - this is important as both will attempt to
	// clean up the session's state. It also ensures calling End more than once is
	// a no-op.
	done uint32
}

// End the session.
//
// This MUST be called when the session-holder is done (e.g. the gRPC stream
// is closed).
func (s *Session) End() {
	if !atomic.CompareAndSwapUint32(&s.done, 0, 1) {
		return
	}

	s.l.mu.Lock()
	defer s.l.mu.Unlock()

	idx := -1
	for i, id := range s.l.sessionIDs {
		if id == s.id {
			idx = i
			break
		}
	}

	s.l.deleteSessionLocked(idx)
}

// Terminated is a channel that is closed when the session is terminated.
//
// The session-holder MUST receive on it and exit (e.g. close the gRPC stream)
// when it is closed.
func (s *Session) Terminated() <-chan struct{} { return s.termCh }

func (s *Session) terminate() {
	if atomic.CompareAndSwapUint32(&s.done, 0, 1) {
		close(s.termCh)
	}
}

func init() { rand.Seed(time.Now().UnixNano()) }
