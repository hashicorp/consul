package ready

// The Readiness interface needs to be implemented by each plugin willing to provide a readiness check.
type Readiness interface {
	// Ready is called by ready to see whether the plugin is ready.
	Ready() bool
}
