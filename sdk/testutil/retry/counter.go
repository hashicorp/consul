package retry

import "time"

// Counter repeats an operation a given number of
// times and waits between subsequent operations.
type Counter struct {
	Count int
	Wait  time.Duration

	count int
}

func (r *Counter) Continue() bool {
	if r.count == r.Count {
		return false
	}
	if r.count > 0 {
		time.Sleep(r.Wait)
	}
	r.count++
	return true
}
