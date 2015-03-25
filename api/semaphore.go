package api

import (
	"encoding/json"
	"fmt"
	"path"
	"sync"
	"time"
)

const (
	// DefaultSemaphoreSessionName is the Session Name we assign if none is provided
	DefaultSemaphoreSessionName = "Consul API Semaphore"

	// DefaultSemaphoreSessionTTL is the default session TTL if no Session is provided
	// when creating a new Semaphore. This is used because we do not have another
	// other check to depend upon.
	DefaultSemaphoreSessionTTL = "15s"

	// DefaultSemaphoreWaitTime is how long we block for at a time to check if semaphore
	// acquisition is possible. This affects the minimum time it takes to cancel
	// a Semaphore acquisition.
	DefaultSemaphoreWaitTime = 15 * time.Second

	// DefaultSemaphoreWatchWaitTime is how long we block for at a time while watching
	// a semaphore. This affects the minimum time it takes to stop watching.
	DefaultSemaphoreWatchWaitTime = 5 * time.Second

	// DefaultSemaphoreRetryTime is how long we wait after a failed lock acquisition
	// before attempting to do the lock again. This is so that once a lock-delay
	// is in affect, we do not hot loop retrying the acquisition.
	DefaultSemaphoreRetryTime = 5 * time.Second

	// DefaultSemaphoreKey is the key used within the prefix to
	// use for coordination between all the contenders.
	DefaultSemaphoreKey = ".lock"

	// SemaphoreFlagValue is a magic flag we set to indicate a key
	// is being used for a semaphore. It is used to detect a potential
	// conflict with a lock.
	SemaphoreFlagValue = 0x9642d0084d1d424d
)

var (
	// ErrSemaphoreHeld is returned if we attempt to double lock
	ErrSemaphoreHeld = fmt.Errorf("Semaphore already held")

	// ErrSemaphoreNotHeld is returned if we attempt to unlock a semaphore
	// that we do not hold.
	ErrSemaphoreNotHeld = fmt.Errorf("Semaphore not held")

	// ErrSemaphoreInUse is returned if we attempt to destroy a semaphore
	// that is in use.
	ErrSemaphoreInUse = fmt.Errorf("Semaphore in use")

	// ErrSemaphoreConflict is returned if the flags on a key
	// used for a semaphore do not match expectation
	ErrSemaphoreConflict = fmt.Errorf("Existing key does not match semaphore use")
)

// Semaphore is used to implement a distributed semaphore
// using the Consul KV primitives.
type Semaphore struct {
	c    *Client
	opts *SemaphoreOptions

	isHeld       bool
	sessionRenew chan struct{}
	lockSession  string
	l            sync.Mutex
}

// SemaphoreOptions is used to parameterize the Semaphore
type SemaphoreOptions struct {
	Prefix      string // Must be set and have write permissions
	Limit       int    // Must be set, and be positive
	Value       []byte // Optional, value to associate with the contender entry
	Session     string // Optional, created if not specified
	SessionName string // Optional, defaults to DefaultLockSessionName
	SessionTTL  string // Optional, defaults to DefaultLockSessionTTL
}

// semaphoreLock is written under the DefaultSemaphoreKey and
// is used to coordinate between all the contenders.
type semaphoreLock struct {
	// Limit is the integer limit of holders. This is used to
	// verify that all the holders agree on the value.
	Limit int

	// Holders is a list of all the semaphore holders and available slots.
	// Its length is always Limit. A session ID in the list marks a slot
	// as held, while an empty string denotes the slot is available.
	Holders semaphoreHolders
}

// semaphoreHolders is just an alias so we can extend []string
type semaphoreHolders []string

// compare against another []string for equality
func (h semaphoreHolders) Equals(a []string) bool {
	if len(a) != len(h) {
		return false
	}
	for i, v := range a {
		if v != h[i] {
			return false
		}
	}
	return true
}

// SemaphorePrefix is used to created a Semaphore which will operate
// at the given KV prefix and uses the given limit for the semaphore.
// The prefix must have write privileges, and the limit must be agreed
// upon by all contenders.
func (c *Client) SemaphorePrefix(prefix string, limit int) (*Semaphore, error) {
	opts := &SemaphoreOptions{
		Prefix: prefix,
		Limit:  limit,
	}
	return c.SemaphoreOpts(opts)
}

// SemaphoreOpts is used to create a Semaphore with the given options.
// The prefix must have write privileges, and the limit must be agreed
// upon by all contenders. If a Session is not provided, one will be created.
func (c *Client) SemaphoreOpts(opts *SemaphoreOptions) (*Semaphore, error) {
	if opts.Prefix == "" {
		return nil, fmt.Errorf("missing prefix")
	}
	if opts.Limit <= 0 {
		return nil, fmt.Errorf("semaphore limit must be positive")
	}
	if opts.SessionName == "" {
		opts.SessionName = DefaultSemaphoreSessionName
	}
	if opts.SessionTTL == "" {
		opts.SessionTTL = DefaultSemaphoreSessionTTL
	} else {
		if _, err := time.ParseDuration(opts.SessionTTL); err != nil {
			return nil, fmt.Errorf("invalid SessionTTL: %v", err)
		}
	}
	s := &Semaphore{
		c:    c,
		opts: opts,
	}
	return s, nil
}

// Acquire attempts to reserve a slot in the semaphore, blocking until
// success, interrupted via the stopCh or an error is encounted.
// Providing a non-nil stopCh can be used to abort the attempt.
// On success, a channel is returned that represents our slot.
// This channel could be closed at any time due to session invalidation,
// communication errors, operator intervention, etc. It is NOT safe to
// assume that the slot is held until Release() unless the Session is specifically
// created without any associated health checks. By default Consul sessions
// prefer liveness over safety and an application must be able to handle
// the session being lost.
func (s *Semaphore) Acquire(stopCh <-chan struct{}) (<-chan struct{}, error) {
	// Hold the lock as we try to acquire
	s.l.Lock()
	defer s.l.Unlock()

	// Check if we already hold the semaphore
	if s.isHeld {
		return nil, ErrSemaphoreHeld
	}

	// Check if we need to create a session first
	s.lockSession = s.opts.Session
	if s.lockSession == "" {
		if sess, err := s.createSession(); err != nil {
			return nil, fmt.Errorf("failed to create session: %v", err)
		} else {
			s.sessionRenew = make(chan struct{})
			s.lockSession = sess
			session := s.c.Session()
			go session.RenewPeriodic(s.opts.SessionTTL, sess, nil, s.sessionRenew)

			// If we fail to acquire the lock, cleanup the session
			defer func() {
				if !s.isHeld {
					close(s.sessionRenew)
					s.sessionRenew = nil
				}
			}()
		}
	}

	// Create the contender entry
	kv := s.c.KV()
	made, _, err := kv.Acquire(s.contenderEntry(s.lockSession), nil)
	if err != nil || !made {
		return nil, fmt.Errorf("failed to make contender entry: %v", err)
	}

	// Setup the query options
	qOpts := &QueryOptions{
		WaitTime: DefaultSemaphoreWaitTime,
	}

WAIT:
	// Check if we should quit
	select {
	case <-stopCh:
		return nil, nil
	default:
	}

	// Read the prefix
	pairs, meta, err := kv.List(s.opts.Prefix, qOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to read prefix: %v", err)
	}

	// Decode the lock
	lockPair := s.findLock(pairs)
	if lockPair.Flags != SemaphoreFlagValue {
		return nil, ErrSemaphoreConflict
	}
	lock, err := s.decodeLock(lockPair)
	if err != nil {
		return nil, err
	}

	// Verify we agree with the limit
	if lock.Limit != s.opts.Limit {
		return nil, fmt.Errorf("semaphore limit conflict (lock: %d, local: %d)",
			lock.Limit, s.opts.Limit)
	}

	// Prune the dead holders
	s.pruneDeadHolders(lock, pairs)

	// Look for an open slot
	slot := s.findSlot(lock, "")

	// Check if the lock is fully held
	if slot == -1 {
		qOpts.WaitIndex = meta.LastIndex
		goto WAIT
	}

	// Create a new lock with us as a holder
	lock.Holders[slot] = s.lockSession
	newLock, err := s.encodeLock(lock, lockPair.ModifyIndex)
	if err != nil {
		return nil, err
	}

	// Attempt the acquisition
	didSet, _, err := kv.CAS(newLock, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update lock: %v", err)
	}
	if !didSet {
		// Update failed, could have been a race with another contender,
		// retry the operation
		goto WAIT
	}

	// Watch to ensure we maintain ownership of the slot
	lockCh := make(chan struct{})
	go s.monitorLock(s.lockSession, lockCh)

	// Set that we own the lock
	s.isHeld = true

	// Acquired! All done
	return lockCh, nil
}

// Release is used to voluntarily give up our semaphore slot. It is
// an error to call this if the semaphore has not been acquired.
func (s *Semaphore) Release() error {
	// Hold the lock as we try to release
	s.l.Lock()
	defer s.l.Unlock()

	// Ensure the lock is actually held
	if !s.isHeld {
		return ErrSemaphoreNotHeld
	}

	// Set that we no longer own the lock
	s.isHeld = false

	// Stop the session renew
	if s.sessionRenew != nil {
		defer func() {
			close(s.sessionRenew)
			s.sessionRenew = nil
		}()
	}

	// Get and clear the lock session
	lockSession := s.lockSession
	s.lockSession = ""

	// Remove ourselves as a lock holder
	kv := s.c.KV()
	key := path.Join(s.opts.Prefix, DefaultSemaphoreKey)
READ:
	pair, _, err := kv.Get(key, nil)
	if err != nil {
		return err
	}
	if pair == nil {
		pair = &KVPair{}
	}
	lock, err := s.decodeLock(pair)
	if err != nil {
		return err
	}

	// Create a new lock without us as a holder
	if slot := s.findSlot(lock, lockSession); slot != -1 {
		lock.Holders[slot] = ""
		newLock, err := s.encodeLock(lock, pair.ModifyIndex)
		if err != nil {
			return err
		}

		// Swap the locks
		didSet, _, err := kv.CAS(newLock, nil)
		if err != nil {
			return fmt.Errorf("failed to update lock: %v", err)
		}
		if !didSet {
			goto READ
		}
	}

	// Destroy the contender entry
	contenderKey := path.Join(s.opts.Prefix, lockSession)
	if _, err := kv.Delete(contenderKey, nil); err != nil {
		return err
	}
	return nil
}

// Destroy is used to cleanup the semaphore entry. It is not necessary
// to invoke. It will fail if the semaphore is in use.
func (s *Semaphore) Destroy() error {
	// Hold the lock as we try to acquire
	s.l.Lock()
	defer s.l.Unlock()

	// Check if we already hold the semaphore
	if s.isHeld {
		return ErrSemaphoreHeld
	}

	// List for the semaphore
	kv := s.c.KV()
	pairs, _, err := kv.List(s.opts.Prefix, nil)
	if err != nil {
		return fmt.Errorf("failed to read prefix: %v", err)
	}

	// Find the lock pair, bail if it doesn't exist
	lockPair := s.findLock(pairs)
	if lockPair.ModifyIndex == 0 {
		return nil
	}
	if lockPair.Flags != SemaphoreFlagValue {
		return ErrSemaphoreConflict
	}

	// Decode the lock
	lock, err := s.decodeLock(lockPair)
	if err != nil {
		return err
	}

	// Prune the dead holders
	s.pruneDeadHolders(lock, pairs)

	// Check if there are any holders
	for _, holder := range lock.Holders {
		if holder != "" {
			return ErrSemaphoreInUse
		}
	}

	// Attempt the delete
	didRemove, _, err := kv.DeleteCAS(lockPair, nil)
	if err != nil {
		return fmt.Errorf("failed to remove semaphore: %v", err)
	}
	if !didRemove {
		return ErrSemaphoreInUse
	}
	return nil
}

// Watch continuously monitors a semaphore and provides updates on its
// result channel whenever there are changes to the semaphore's holders.
// The updates are in the form of a [][]byte whose length is equal to the
// Limit of the semaphore. Each entry contains the SemaphoreOptions.Value
// for the contender who holds that slot in the semaphore. An unheld slot
// is represented by an empty []byte.
// Watching will continue until an error occurs or stopCh is closed.
func (s *Semaphore) Watch(stopCh <-chan struct{}) (<-chan [][]byte, <-chan error) {
	resultCh := make(chan [][]byte)
	errCh := make(chan error)

	go func() {
		defer func() {
			close(resultCh)
			close(errCh)
		}()

		// Setup the query options
		qOpts := &QueryOptions{
			WaitTime: DefaultSemaphoreWatchWaitTime,
		}

		// Our last-known state of lock.Holders
		var holders semaphoreHolders

		for {
			// Check if we should quit
			select {
			case <-stopCh:
				return
			default:
			}

			// Read the prefix
			kv := s.c.KV()
			pairs, meta, err := kv.List(s.opts.Prefix, qOpts)
			if err != nil {
				errCh <- err
				return
			}

			// If nothing changed, continue watching
			if qOpts.WaitIndex == meta.LastIndex {
				continue
			}
			qOpts.WaitIndex = meta.LastIndex

			// Find the lock
			lockPair := s.findLock(pairs)
			if lockPair.Flags != SemaphoreFlagValue {
				errCh <- ErrSemaphoreConflict
				return
			}

			// Decode the lock
			lock, err := s.decodeLock(lockPair)
			if err != nil {
				errCh <- err
				return
			}

			// Prune dead holders, get Values for alive holders
			alive := s.pruneDeadHolders(lock, pairs)

			// If the lock holders didn't change, continue watching
			if holders.Equals(lock.Holders) {
				continue
			}
			holders = lock.Holders

			// Gather Values for alive holders
			values := make([][]byte, lock.Limit)
			for i, holder := range lock.Holders {
				if holder != "" {
					values[i] = alive[holder]
				}
			}
			resultCh <- values
		}
	}()

	return resultCh, errCh
}

// createSession is used to create a new managed session
func (s *Semaphore) createSession() (string, error) {
	session := s.c.Session()
	se := &SessionEntry{
		Name:     s.opts.SessionName,
		TTL:      s.opts.SessionTTL,
		Behavior: SessionBehaviorDelete,
	}
	id, _, err := session.Create(se, nil)
	if err != nil {
		return "", err
	}
	return id, nil
}

// contenderEntry returns a formatted KVPair for the contender
func (s *Semaphore) contenderEntry(session string) *KVPair {
	return &KVPair{
		Key:     path.Join(s.opts.Prefix, session),
		Value:   s.opts.Value,
		Session: session,
		Flags:   SemaphoreFlagValue,
	}
}

// findLock is used to find the KV Pair which is used for coordination
func (s *Semaphore) findLock(pairs KVPairs) *KVPair {
	key := path.Join(s.opts.Prefix, DefaultSemaphoreKey)
	for _, pair := range pairs {
		if pair.Key == key {
			return pair
		}
	}
	return &KVPair{Flags: SemaphoreFlagValue}
}

// decodeLock is used to decode a semaphoreLock from an
// entry in Consul
func (s *Semaphore) decodeLock(pair *KVPair) (*semaphoreLock, error) {
	// Handle if there is no lock
	if pair == nil || pair.Value == nil {
		return &semaphoreLock{
			Limit:   s.opts.Limit,
			Holders: make([]string, s.opts.Limit),
		}, nil
	}

	l := &semaphoreLock{}
	if err := json.Unmarshal(pair.Value, l); err != nil {
		return nil, fmt.Errorf("lock decoding failed: %v", err)
	}
	return l, nil
}

// encodeLock is used to encode a semaphoreLock into a KVPair
// that can be PUT
func (s *Semaphore) encodeLock(l *semaphoreLock, oldIndex uint64) (*KVPair, error) {
	enc, err := json.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("lock encoding failed: %v", err)
	}
	pair := &KVPair{
		Key:         path.Join(s.opts.Prefix, DefaultSemaphoreKey),
		Value:       enc,
		Flags:       SemaphoreFlagValue,
		ModifyIndex: oldIndex,
	}
	return pair, nil
}

// findSlot is used to locate an index position in lock.Holders.
// By passing session == "", findSlot will return the first available
// slot, or -1 if none are available.
func (s *Semaphore) findSlot(lock *semaphoreLock, session string) int {
	for i, holder := range lock.Holders {
		if holder == session {
			return i
		}
	}
	return -1
}

// pruneDeadHolders is used to remove all the dead lock holders.
// The Value from each contender pair is returned for all living lock holders.
func (s *Semaphore) pruneDeadHolders(lock *semaphoreLock, pairs KVPairs) map[string][]byte {
	// Gather all the live holders
	alive := make(map[string][]byte, len(pairs))
	for _, pair := range pairs {
		if pair.Session != "" {
			alive[pair.Session] = pair.Value
		}
	}

	// Remove any holders that are dead
	for i, holder := range lock.Holders {
		if holder != "" {
			if _, ok := alive[holder]; !ok {
				lock.Holders[i] = ""
			}
		}
	}

	return alive
}

// monitorLock is a long running routine to monitor a semaphore ownership
// It closes the stopCh if we lose our slot.
func (s *Semaphore) monitorLock(session string, stopCh chan struct{}) {
	defer close(stopCh)
	kv := s.c.KV()
	opts := &QueryOptions{RequireConsistent: true}
WAIT:
	pairs, meta, err := kv.List(s.opts.Prefix, opts)
	if err != nil {
		return
	}
	lockPair := s.findLock(pairs)
	lock, err := s.decodeLock(lockPair)
	if err != nil {
		return
	}
	s.pruneDeadHolders(lock, pairs)
	if slot := s.findSlot(lock, session); slot != -1 {
		opts.WaitIndex = meta.LastIndex
		goto WAIT
	}
}
