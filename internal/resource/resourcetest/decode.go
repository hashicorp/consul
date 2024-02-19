// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package resourcetest

import (
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/hashicorp/consul/internal/resource"
	"github.com/hashicorp/consul/proto-public/pbresource"
)

func MustDecode[Tp proto.Message](t T, res *pbresource.Resource) *resource.DecodedResource[Tp] {
	dec, err := resource.Decode[Tp](res)
	require.NoError(t, err)
	return dec
}

func MustDecodeList[Tp proto.Message](t T, resources []*pbresource.Resource) []*resource.DecodedResource[Tp] {
	dec, err := resource.DecodeList[Tp](resources)
	require.NoError(t, err)
	return dec
}
