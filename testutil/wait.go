package testutil

import (
	"github.com/hashicorp/consul/consul/structs"
	"testing"
	"time"
)

type testFn func() (bool, error)
type errorFn func(error)

func WaitForResult(test testFn, error errorFn) {
	retries := 1000

	for retries > 0 {
		time.Sleep(10 * time.Millisecond)
		retries--

		success, err := test()
		if success {
			return
		}

		if retries == 0 {
			error(err)
		}
	}
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
