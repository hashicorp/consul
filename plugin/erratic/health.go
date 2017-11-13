package erratic

import (
	"sync/atomic"
)

// Health implements the health.Healther interface.
func (e *Erratic) Health() bool {
	q := atomic.LoadUint64(&e.q)
	if e.drop > 0 && q%e.drop == 0 {
		return false
	}
	return true
}
