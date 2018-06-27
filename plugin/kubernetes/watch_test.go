package kubernetes

import (
	"testing"

	"github.com/coredns/coredns/plugin/pkg/watch"
)

func TestIsWatchable(t *testing.T) {
	k := &Kubernetes{}
	var i interface{} = k
	if _, ok := i.(watch.Watchable); !ok {
		t.Error("Kubernetes should implement watch.Watchable and does not")
	}
}
