package consul

import (
	"github.com/hashicorp/serf/serf"
	"testing"
)

func TestStrContains(t *testing.T) {
	l := []string{"a", "b", "c"}
	if !strContains(l, "b") {
		t.Fatalf("should contain")
	}
	if strContains(l, "d") {
		t.Fatalf("should not contain")
	}
}

func TestIsPrivateIP(t *testing.T) {
	if !isPrivateIP("192.168.1.1") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("172.16.45.100") {
		t.Fatalf("bad")
	}
	if !isPrivateIP("10.1.2.3") {
		t.Fatalf("bad")
	}
	if isPrivateIP("8.8.8.8") {
		t.Fatalf("bad")
	}
	if isPrivateIP("127.0.0.1") {
		t.Fatalf("bad")
	}
}

func TestIsConsulServer(t *testing.T) {
	m := serf.Member{
		Role: "consul:east-aws:10000",
	}
	valid, dc, port := isConsulServer(m)
	if !valid || dc != "east-aws" || port != 10000 {
		t.Fatalf("bad: %v %v %v", valid, dc, port)
	}
}

func TestIsConsulNode(t *testing.T) {
	m := serf.Member{
		Role: "node:east-aws",
	}
	valid, dc := isConsulNode(m)
	if !valid || dc != "east-aws" {
		t.Fatalf("bad: %v %v %v", valid, dc)
	}
}

func TestByteConversion(t *testing.T) {
	var val uint64 = 2 << 50
	raw := uint64ToBytes(val)
	if bytesToUint64(raw) != val {
		t.Fatalf("no match")
	}
}
