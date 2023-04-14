package integration_test

import testclientserver "github.com/hashicorp/consul/sdk/testutil/clientserver"

// assert that we fulfill the interface
var _ TestServerI = &testclientserver.TestServerAdapter{}

// TODO: name, location
type TestServerI interface {
	// TODO: not sure we really need this; just use t.Cleanup, so maybe a no-op
	Stop() error
}
