package testutil

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hashicorp/consul/consul/structs"
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

type ClosableResponseRecorder struct {
	*httptest.ResponseRecorder
	Notifier chan bool
	writes   *[][]byte
}

func (c ClosableResponseRecorder) Write(buf []byte) (int, error) {
	*c.writes = append(*c.writes, buf)
	return len(buf), nil
}

func (c ClosableResponseRecorder) CloseNotify() <-chan bool {
	return c.Notifier
}

func NewClosableResponseWriter(writeBuffer *[][]byte) *ClosableResponseRecorder {
	return &ClosableResponseRecorder{httptest.NewRecorder(), make(chan bool, 1), writeBuffer}
}
