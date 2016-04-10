// +build etcd

package test

import "testing"

// This test starts two coredns servers (and needs etcd). Configure a stubzones in both (that will loop) and
// will then test if we detect this loop.
func TestEtcdStubForwarding(t *testing.T) {
	// TODO(miek)
}
