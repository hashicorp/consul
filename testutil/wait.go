package testutil

import (
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
)

type testFn func() (bool, error)
type errorFn func(error)

const (
	baseWait = 1 * time.Millisecond
	maxWait  = 100 * time.Millisecond
)

func WaitForResult(try testFn, fail errorFn) {
	var err error
	wait := baseWait
	for retries := 100; retries > 0; retries-- {
		var success bool
		success, err = try()
		if success {
			return
		}

		time.Sleep(wait)
		wait *= 2
		if wait > maxWait {
			wait = maxWait
		}
	}
	fail(err)
}

type rpcFn func(string, interface{}, interface{}) error

func WaitForLeader(t *testing.T, rpc rpcFn, dc string) structs.IndexedNodes {
	var out structs.IndexedNodes
	WaitForResult(func() (bool, error) {
		args := &structs.DCSpecificRequest{
			Datacenter: dc,
		}
		err := rpc("Catalog.ListNodes", args, &out)
		return out.QueryMeta.KnownLeader && out.Index > 0, err
	}, func(err error) {
		t.Fatalf("failed to find leader: %v", err)
	})
	return out
}
