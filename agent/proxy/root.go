package proxy

import (
	"os"
)

// isRoot returns true if the process is executing as root.
func isRoot() bool {
	if testRootValue != nil {
		return *testRootValue
	}

	return os.Geteuid() == 0
}

// testSetRootValue is a test helper for setting the root value.
func testSetRootValue(v bool) func() {
	testRootValue = &v
	return func() { testRootValue = nil }
}

// testRootValue should be set to a non-nil value to return it as a stub
// from isRoot. This should only be used in tests.
var testRootValue *bool
