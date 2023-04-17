package controller

// Lease is used to ensure controllers are run as singletons (i.e. one leader-
// elected instance per cluster).
//
// Currently, this is just an abstraction over Raft leadership. In the future,
// we'll build a backend-agnostic leasing system into the Resource Service which
// will allow us to balance controllers between many servers.
type Lease interface {
	// Held returns whether we are the current lease-holders.
	Held() bool

	// Watch returns a channel on which you can receive notifications whenever the
	// lease is acquired or lost.
	Watch() <-chan struct{}
}
