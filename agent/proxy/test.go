package proxy

// defaultTestProxy is the test proxy that is instantiated for proxies with
// an execution mode of ProxyExecModeTest.
var defaultTestProxy = testProxy{}

// testProxy is a Proxy implementation that stores state in-memory and
// is only used for unit testing. It is in a non _test.go file because the
// factory for initializing it is exported (newProxy).
type testProxy struct {
	Start uint32
	Stop  uint32
}
