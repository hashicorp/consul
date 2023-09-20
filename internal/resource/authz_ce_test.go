// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build !consulent
// +build !consulent

package resource

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func TestAuthorizerContext_CE(t *testing.T) {
	t.Run("no peer", func(t *testing.T) {
		require.Equal(t,
			&acl.AuthorizerContext{},
			AuthorizerContext(&pbresource.Tenancy{
				Partition: "foo",
				Namespace: "bar",
			}),
		)
	})

	t.Run("with local peer", func(t *testing.T) {
		require.Equal(t,
			&acl.AuthorizerContext{},
			AuthorizerContext(&pbresource.Tenancy{
				Partition: "foo",
				Namespace: "bar",
				PeerName:  "local",
			}),
		)
	})

	t.Run("with non-local peer", func(t *testing.T) {
		require.Equal(t,
			&acl.AuthorizerContext{
				Peer: "remote",
			},
			AuthorizerContext(&pbresource.Tenancy{
				Partition: "foo",
				Namespace: "bar",
				PeerName:  "remote",
			}),
		)
	})
}
