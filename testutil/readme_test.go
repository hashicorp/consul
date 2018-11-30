// This test should match the example in README.md.  If you make
// changes here, please make the same changes there as well.
package testutil_test

import (
	"testing"

	"github.com/hashicorp/consul/testutil"
)

func TestFoo_bar(t *testing.T) {
	// Create a test Consul server
	srv1, err := testutil.NewTestServer()
	if err != nil {
		t.Fatal(err)
	}
	defer srv1.Stop()

	// Create a secondary server, passing in configuration
	// to avoid bootstrapping as we are forming a cluster.
	srv2, err := testutil.NewTestServerConfigT(t, func(c *testutil.TestServerConfig) {
		c.Bootstrap = false
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv2.Stop()

	// Join the servers together
	srv1.JoinLAN(t, srv2.LANAddr)

	// Create a test key/value pair
	srv1.SetKV(t, "foo", []byte("bar"))

	// Create lots of test key/value pairs
	srv1.PopulateKV(t, map[string][]byte{
		"bar": []byte("123"),
		"baz": []byte("456"),
	})

	// Create a service
	srv1.AddService(t, "redis", testutil.HealthPassing, []string{"master"})

	// Create a service that will be accessed in target source code
	srv1.AddAddressableService(t, "redis", testutil.HealthPassing, "127.0.0.1", 6379, []string{"master"})

	// Create a service check
	srv1.AddCheck(t, "service:redis", "redis", testutil.HealthPassing)

	// Create a node check
	srv1.AddCheck(t, "mem", "", testutil.HealthCritical)

	// The HTTPAddr field contains the address of the Consul
	// API on the new test server instance.
	println(srv1.HTTPAddr)

	// All functions also have a wrapper method to limit the passing of "t"
	wrap := srv1.Wrap(t)
	wrap.SetKV("foo", []byte("bar"))
}
