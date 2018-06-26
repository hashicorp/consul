package proxy

import (
	"testing"
)

func TestNoop_impl(t *testing.T) {
	var _ Proxy = new(Noop)
}
