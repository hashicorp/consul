// Package retry provides support for repeating operations in tests.
//
// A sample retry operation looks like this:
//
//   func TestX(t *testing.T) {
//       for r := retry.OneSec(); r.NextOr(t.FailNow); {
//           if err := foo(); err != nil {
//               t.Log("f: ", err)
//               continue
//           }
//           break
//       }
//   }
//
// A sample retry operation which exits a test with a message
// looks like this:
//
//   func TestX(t *testing.T) {
//       for r := retry.OneSec(); r.NextOr(func(){ t.Fatal("foo failed") }); {
//           if err := foo(); err != nil {
//               t.Log("f: ", err)
//               continue
//           }
//           break
//       }
//   }
package retry

import "time"

// OneSec repeats an operation for one second and waits 25ms in between.
func OneSec() *Timer {
	return &Timer{Timeout: time.Second, Wait: 25 * time.Millisecond}
}

// ThreeTimes repeats an operation three times and waits 25ms in between.
func ThreeTimes() *Counter {
	return &Counter{Count: 3, Wait: 25 * time.Millisecond}
}

// Retryer provides an interface for repeating operations
// until they succeed or an exit condition is met.
type Retryer interface {
	// NextOr returns true if the operation should be repeated.
	// Otherwise, it calls fail and returns false.
	NextOr(fail func()) bool
}

// Counter repeats an operation a given number of
// times and waits between subsequent operations.
type Counter struct {
	Count int
	Wait  time.Duration

	count int
}

func (r *Counter) NextOr(fail func()) bool {
	if r.count == r.Count {
		fail()
		return false
	}
	if r.count > 0 {
		time.Sleep(r.Wait)
	}
	r.count++
	return true
}

// Timer repeats an operation for a given amount
// of time and waits between subsequent operations.
type Timer struct {
	Timeout time.Duration
	Wait    time.Duration

	// stop is the timeout deadline.
	// Set on the first invocation of Next().
	stop time.Time
}

func (r *Timer) NextOr(fail func()) bool {
	if r.stop.IsZero() {
		r.stop = time.Now().Add(r.Timeout)
		return true
	}
	if time.Now().After(r.stop) {
		fail()
		return false
	}
	time.Sleep(r.Wait)
	return true
}
