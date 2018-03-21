package consul

import (
	"sync"
)

// testEndpointsOnce ensures that endpoints for testing are registered once.
var testEndpointsOnce sync.Once

// TestEndpoints registers RPC endpoints specifically for testing. These
// endpoints enable some internal data access that we normally disallow, but
// are useful for modifying server state.
//
// To use this, modify TestMain to call this function prior to running tests.
//
// These should NEVER be registered outside of tests.
//
// NOTE(mitchellh): This was created so that the downstream agent tests can
// modify internal Connect CA state. When the CA plugin work comes in with
// a more complete CA API, this may no longer be necessary and we can remove it.
// That would be ideal.
func TestEndpoint() {
	testEndpointsOnce.Do(func() {
		registerEndpoint(func(s *Server) interface{} { return &Test{s} })
	})
}
