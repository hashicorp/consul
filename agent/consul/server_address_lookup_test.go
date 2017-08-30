package consul

import (
	"fmt"
	"testing"
)

func TestServerAddressLookup(t *testing.T) {
	lookup := NewServerAddressLookup()
	addr := "72.0.0.17:8300"
	lookup.AddServer("1", addr)

	got, err := lookup.ServerAddr("1")
	if err != nil {
		t.Fatalf("Unexpected error:%v", err)
	}
	if string(got) != addr {
		t.Fatalf("Expected %v but got %v", addr, got)
	}

	lookup.RemoveServer("1")

	got, err = lookup.ServerAddr("1")
	expectedErr := fmt.Errorf("Could not find address for server id 1")
	if expectedErr.Error() != err.Error() {
		t.Fatalf("Unexpected error, got %v wanted %v", err, expectedErr)
	}

	lookup.RemoveServer("3")
}
