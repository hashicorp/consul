package erratic

import "sync/atomic"

// Ready returns true if the number of received queries is in the range [3, 5). All other values return false.
// To aid in testing we want to this flip between ready and not ready.
func (e *Erratic) Ready() bool {
	q := atomic.LoadUint64(&e.q)
	if q >= 3 && q < 5 {
		return true
	}
	return false
}
