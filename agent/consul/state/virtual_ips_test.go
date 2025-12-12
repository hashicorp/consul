// Copyright IBM Corp. 2014, 2025
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseVirtualIPCIDRIPv4(t *testing.T) {
	base, max, err := parseVirtualIPCIDR("10.0.0.0/29", net.IPv4len)
	require.NoError(t, err)
	require.Equal(t, net.IPv4(10, 0, 0, 0).To4(), base)
	require.Equal(t, net.IPv4(0, 0, 0, 6), max)
}

func TestParseVirtualIPCIDRIPv6(t *testing.T) {
	base, max, err := parseVirtualIPCIDR("fd00::/125", net.IPv6len)
	require.NoError(t, err)
	require.Equal(t, net.ParseIP("fd00::").To16(), base)
	require.Equal(t, net.ParseIP("::6").To16(), max)
}

func TestParseVirtualIPCIDRTooSmall(t *testing.T) {
	_, _, err := parseVirtualIPCIDR("10.0.0.0/31", net.IPv4len)
	require.Error(t, err)
}

func TestSetVirtualIPConfigOverrides(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, SetVirtualIPConfig("", ""))
	})

	require.NoError(t, SetVirtualIPConfig("10.0.0.0/29", "fd00::/125"))
	cfg := currentVirtualIPConfig()

	// Validate starting points and max offsets are derived from the new ranges.
	v4Base, err := addIPv4Offset(cfg.startingIPv4, net.IPv4zero)
	require.NoError(t, err)
	require.Equal(t, "10.0.0.0", v4Base.String())
	require.Equal(t, net.IPv4(0, 0, 0, 6), cfg.maxOffsetIPv4)

	v6Base, err := addIPv6Offset(cfg.startingIPv6, net.ParseIP("::"))
	require.NoError(t, err)
	require.Equal(t, "fd00::", v6Base.String())
	require.Equal(t, net.ParseIP("::6").To16(), cfg.maxOffsetIPv6)
}
