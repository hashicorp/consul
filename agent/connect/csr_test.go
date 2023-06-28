// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateCSR_FormatDNSSANs(t *testing.T) {
	pk, _, err := GeneratePrivateKey()
	require.NoError(t, err)
	spiffeID := &SpiffeIDService{
		Host:       "7528f42f-92e5-4db4-b84c-3405c3ca91e6",
		Service:    "srv1",
		Datacenter: "dc1",
	}
	csr, err := CreateCSR(spiffeID, pk, []string{
		"foo.example.com",
		"foo.example.com:8080",
		"bar.example.com",
		"*.example.com",
		":8080",
		"",
	}, nil)
	require.NoError(t, err)

	req, err := ParseCSR(csr)
	require.NoError(t, err)
	require.Len(t, req.URIs, 1)
	require.Equal(t, spiffeID.URI(), req.URIs[0])
	require.Equal(t, []string{
		"foo.example.com",
		"bar.example.com",
		"*.example.com",
	}, req.DNSNames)
}
