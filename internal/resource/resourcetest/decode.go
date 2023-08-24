// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func MustDecode[T proto.Message](t *testing.T, res *pbresource.Resource) *resource.DecodedResource[T] {
	dec, err := resource.Decode[T](res)
	require.NoError(t, err)
	return dec
}
