// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package consul

import (
	"context"
	"net"
)

type contextKeyRemoteAddr struct{}

func ContextWithRemoteAddr(ctx context.Context, addr net.Addr) context.Context {
	return context.WithValue(ctx, contextKeyRemoteAddr{}, addr)
}

func RemoteAddrFromContext(ctx context.Context) (net.Addr, bool) {
	v := ctx.Value(contextKeyRemoteAddr{})
	if v == nil {
		return nil, false
	}
	return v.(net.Addr), true
}
