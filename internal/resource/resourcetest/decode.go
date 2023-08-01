// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package resourcetest

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func MustDecode[V any, PV interface {
	proto.Message
	*V
}](t *testing.T, res *pbresource.Resource) *resource.DecodedResource[V, PV] {
	dec, err := resource.Decode[V, PV](res)
	require.NoError(t, err)
	return dec
}
