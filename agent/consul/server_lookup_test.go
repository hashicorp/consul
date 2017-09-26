package consul

import (
	"fmt"
	"testing"

	"github.com/hashicorp/consul/agent/metadata"
	"github.com/hashicorp/raft"
)

type testAddr struct {
	addr string
}

func (ta *testAddr) Network() string {
	return "tcp"
}

func (ta *testAddr) String() string {
	return ta.addr
}

func TestServerLookup(t *testing.T) {
	lookup := NewServerLookup()
	addr := "72.0.0.17:8300"
	id := "1"

	svr := &metadata.Server{ID: id, Addr: &testAddr{addr}}
	lookup.AddServer(svr)

	got, err := lookup.ServerAddr(raft.ServerID(id))
	if err != nil {
		t.Fatalf("Unexpected error:%v", err)
	}
	if string(got) != addr {
		t.Fatalf("Expected %v but got %v", addr, got)
	}

	server := lookup.Server(raft.ServerAddress(addr))
	if server == nil {
		t.Fatalf("Expected lookup to return true")
	}
	if server.Addr.String() != addr {
		t.Fatalf("Expected lookup to return address %v but got %v", addr, server.Addr)
	}

	lookup.RemoveServer(svr)

	got, err = lookup.ServerAddr("1")
	expectedErr := fmt.Errorf("Could not find address for server id 1")
	if expectedErr.Error() != err.Error() {
		t.Fatalf("Unexpected error, got %v wanted %v", err, expectedErr)
	}

	svr2 := &metadata.Server{ID: "2", Addr: &testAddr{"123.4.5.6"}}
	lookup.RemoveServer(svr2)

}
