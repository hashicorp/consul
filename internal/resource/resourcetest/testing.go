package resourcetest

// T represents the subset of testing.T methods that will be used
// by the various functionality in this package
type T interface {
	Helper()
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	FailNow()
}

type CleanupT interface {
	T
	Cleanup(func())
}
