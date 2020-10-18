package testutil

// TestingTB is an interface that describes the implementation of the testing object.
// Using an interface that describes testing.TB instead of the actual implementation
// makes testutil usable in a wider variety of contexts (e.g. use with ginkgo : https://godoc.org/github.com/onsi/ginkgo#GinkgoT)
type TestingTB interface {
	Cleanup(func())
	Failed() bool
	Logf(format string, args ...interface{})
	Name() string
}
