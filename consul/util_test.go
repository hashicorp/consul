package consul

import (
	"github.com/hashicorp/serf/serf"
	"regexp"
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
		Tags: map[string]string{
			"role": "consul",
			"dc":   "east-aws",
			"port": "10000",
		},
	}
	valid, parts := isConsulServer(m)
	if !valid || parts.Datacenter != "east-aws" || parts.Port != 10000 {
		t.Fatalf("bad: %v %v", valid, parts)
	}
	if parts.Bootstrap {
		t.Fatalf("unexpected bootstrap")
	}
	m.Tags["bootstrap"] = "1"
	valid, parts = isConsulServer(m)
	if !valid || !parts.Bootstrap {
		t.Fatalf("expected bootstrap")
	}
}

func TestIsConsulNode(t *testing.T) {
	m := serf.Member{
		Tags: map[string]string{
			"role": "node",
			"dc":   "east-aws",
		},
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

func TestGenerateUUID(t *testing.T) {
	prev := generateUUID()
	for i := 0; i < 100; i++ {
		id := generateUUID()
		if prev == id {
			t.Fatalf("Should get a new ID!")
		}

		matched, err := regexp.MatchString(
			"[\\da-f]{8}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{4}-[\\da-f]{12}", id)
		if !matched || err != nil {
			t.Fatalf("expected match %s %v %s", id, matched, err)
		}
	}
}
