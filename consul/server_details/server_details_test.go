package server_details_test

import (
	"net"
	"testing"

	"github.com/hashicorp/consul/consul/server_details"
	"github.com/hashicorp/serf/serf"
)

func TestIsConsulServer(t *testing.T) {
	m := serf.Member{
		Name: "foo",
		Addr: net.IP([]byte{127, 0, 0, 1}),
		Tags: map[string]string{
			"role": "consul",
			"dc":   "east-aws",
			"port": "10000",
			"vsn":  "1",
		},
	}
	ok, parts := server_details.IsConsulServer(m)
	if !ok || parts.Datacenter != "east-aws" || parts.Port != 10000 {
		t.Fatalf("bad: %v %v", ok, parts)
	}
	if parts.Name != "foo" {
		t.Fatalf("bad: %v", parts)
	}
	if parts.Bootstrap {
		t.Fatalf("unexpected bootstrap")
	}
	if parts.Expect != 0 {
		t.Fatalf("bad: %v", parts.Expect)
	}
	m.Tags["bootstrap"] = "1"
	m.Tags["disabled"] = "1"
	ok, parts = server_details.IsConsulServer(m)
	if !ok {
		t.Fatalf("expected a valid consul server")
	}
	if !parts.Bootstrap {
		t.Fatalf("expected bootstrap")
	}
	if parts.Addr.String() != "127.0.0.1:10000" {
		t.Fatalf("bad addr: %v", parts.Addr)
	}
	if parts.Version != 1 {
		t.Fatalf("bad: %v", parts)
	}
	m.Tags["expect"] = "3"
	delete(m.Tags, "bootstrap")
	delete(m.Tags, "disabled")
	ok, parts = server_details.IsConsulServer(m)
	if !ok || parts.Expect != 3 {
		t.Fatalf("bad: %v", parts.Expect)
	}
	if parts.Bootstrap {
		t.Fatalf("unexpected bootstrap")
	}
}
