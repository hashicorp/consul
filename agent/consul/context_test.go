// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"net"
	"net/netip"
	"testing"
)

func TestRemoteAddrFromContext_Found(t *testing.T) {
	in := net.TCPAddrFromAddrPort(netip.MustParseAddrPort("1.2.3.4:8080"))
	ctx := ContextWithRemoteAddr(context.Background(), in)
	out, ok := RemoteAddrFromContext(ctx)
	if !ok {
		t.Fatalf("cannot get remote addr from context")
	}
	if in != out {
		t.Fatalf("expected %s but got %s instead", in, out)
	}
}

func TestRemoteAddrFromContext_NotFound(t *testing.T) {
	out, ok := RemoteAddrFromContext(context.Background())
	if ok || out != nil {
		t.Fatalf("expected remote addr %s to not be in context", out)
	}
}
