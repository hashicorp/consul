package plugin

import "context"

// Done is a non-blocking function that returns true if the context has been canceled.
func Done(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
