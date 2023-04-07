package utils

// ResettableDefer is a way to capture a series of cleanup functions and
// bulk-cancel them. Ideal to use in a long constructor function before the
// overall Close/Stop/Terminate method is ready to use to tear down all of the
// portions properly.
type ResettableDefer struct {
	cleanupFns []func()
}

// Add registers another function to call at Execute time.
func (d *ResettableDefer) Add(f func()) {
	d.cleanupFns = append(d.cleanupFns, f)
}

// Reset clears the pending defer work.
func (d *ResettableDefer) Reset() {
	d.cleanupFns = nil
}

// Execute actually executes the functions registered by Add in the reverse
// order of their call order (like normal defer blocks).
func (d *ResettableDefer) Execute() {
	// Run these in reverse order, like defer blocks.
	for i := len(d.cleanupFns) - 1; i >= 0; i-- {
		d.cleanupFns[i]()
	}
}
