package ae

// Trigger implements a non-blocking event notifier. Events can be
// triggered without blocking and notifications happen only when the
// previous event was consumed.
type Trigger struct {
	ch chan struct{}
}

func NewTrigger() *Trigger {
	return &Trigger{make(chan struct{}, 1)}
}

func (t Trigger) Trigger() {
	select {
	case t.ch <- struct{}{}:
	default:
	}
}

// wait returns a channel that blocks until Trigger is called.
func (t Trigger) wait() <-chan struct{} {
	return t.ch
}
