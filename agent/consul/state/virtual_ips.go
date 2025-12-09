// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package state

import (
	"fmt"
	"math/big"
	"net"
	"sync"
)

const (
	// DefaultVirtualIPv4CIDR matches the historical 240.0.0.0/4 range used for auto-assigned
	// virtual IPs.
	DefaultVirtualIPv4CIDR = "240.0.0.0/4"
	// DefaultVirtualIPv6CIDR matches the historical 2000::/3 range used for auto-assigned
	// virtual IPs when dual-stack is enabled.
	DefaultVirtualIPv6CIDR = "2000::/3"
)

type virtualIPAllocatorConfig struct {
	startingIPv4  net.IP
	maxOffsetIPv4 net.IP
	startingIPv6  net.IP
	maxOffsetIPv6 net.IP
}

var (
	virtualIPConfigMu sync.RWMutex
	virtualIPConfig   = mustBuildVirtualIPConfig(DefaultVirtualIPv4CIDR, DefaultVirtualIPv6CIDR)
)

func mustBuildVirtualIPConfig(v4CIDR, v6CIDR string) virtualIPAllocatorConfig {
	cfg, err := buildVirtualIPConfig(v4CIDR, v6CIDR)
	if err != nil {
		panic(err)
	}
	return cfg
}

// SetVirtualIPConfig configures the allocator ranges for IPv4 and IPv6 virtual IPs. Empty strings
// fall back to defaults. It is expected to be called during server startup before any allocations
// occur.
func SetVirtualIPConfig(v4CIDR, v6CIDR string) error {
	cfg, err := buildVirtualIPConfig(v4CIDR, v6CIDR)
	if err != nil {
		return err
	}

	virtualIPConfigMu.Lock()
	virtualIPConfig = cfg
	virtualIPConfigMu.Unlock()
	return nil
}

// ValidateVirtualIPCIDRs checks that the provided CIDRs can be used for virtual IP allocation.
// Empty values are treated as defaults.
func ValidateVirtualIPCIDRs(v4CIDR, v6CIDR string) error {
	_, err := buildVirtualIPConfig(v4CIDR, v6CIDR)
	return err
}

func currentVirtualIPConfig() virtualIPAllocatorConfig {
	virtualIPConfigMu.RLock()
	cfg := virtualIPConfig
	virtualIPConfigMu.RUnlock()
	return cfg
}

func buildVirtualIPConfig(v4CIDR, v6CIDR string) (virtualIPAllocatorConfig, error) {
	cfg := virtualIPAllocatorConfig{}

	if v4CIDR == "" {
		v4CIDR = DefaultVirtualIPv4CIDR
	}
	if v6CIDR == "" {
		v6CIDR = DefaultVirtualIPv6CIDR
	}

	startV4, maxOffsetV4, err := parseVirtualIPCIDR(v4CIDR, net.IPv4len)
	if err != nil {
		return cfg, fmt.Errorf("invalid virtual_ip_cidr_v4: %w", err)
	}
	startV6, maxOffsetV6, err := parseVirtualIPCIDR(v6CIDR, net.IPv6len)
	if err != nil {
		return cfg, fmt.Errorf("invalid virtual_ip_cidr_v6: %w", err)
	}

	cfg.startingIPv4 = startV4
	cfg.maxOffsetIPv4 = maxOffsetV4
	cfg.startingIPv6 = startV6
	cfg.maxOffsetIPv6 = maxOffsetV6
	return cfg, nil
}

// parseVirtualIPCIDR returns the base network address and the maximum offset allowed (host space
// minus the broadcast address) for the given cidr. expectedLen should be net.IPv4len or
// net.IPv6len to ensure family matches.
func parseVirtualIPCIDR(cidr string, expectedLen int) (net.IP, net.IP, error) {
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, nil, err
	}

	ones, bits := ipNet.Mask.Size()
	if bits != expectedLen*8 {
		return nil, nil, fmt.Errorf("cidr %q must be IPv%d", cidr, expectedLen*8)
	}
	hostBits := bits - ones
	// Require at least 4 addresses (hostBits >= 2) to stay consistent with historical range that
	// reserved the broadcast address but allowed the network address.
	if hostBits < 2 {
		return nil, nil, fmt.Errorf("cidr %q must allow at least four addresses", cidr)
	}

	base := ip.Mask(ipNet.Mask)
	if expectedLen == net.IPv4len {
		base = base.To4()
		if base == nil {
			return nil, nil, fmt.Errorf("cidr %q must be IPv4", cidr)
		}
		hostCount := uint64(1) << uint(hostBits)
		// Leave room for the broadcast address to mirror prior behavior.
		maxOffset := hostCount - 2
		return base, net.IPv4(byte(maxOffset>>24), byte(maxOffset>>16), byte(maxOffset>>8), byte(maxOffset)), nil
	}

	// IPv6
	base = base.To16()
	if base == nil {
		return nil, nil, fmt.Errorf("cidr %q must be IPv6", cidr)
	}

	hostCount := big.NewInt(0).Lsh(big.NewInt(1), uint(hostBits))
	hostCount.Sub(hostCount, big.NewInt(2))
	maxOffset := hostCount.Bytes()

	// Left-pad to 16 bytes.
	if len(maxOffset) < net.IPv6len {
		padded := make([]byte, net.IPv6len)
		copy(padded[net.IPv6len-len(maxOffset):], maxOffset)
		maxOffset = padded
	}

	return base, net.IP(maxOffset), nil
}

func (cfg virtualIPAllocatorConfig) maxOffsetFor(ip net.IP) net.IP {
	if ip.To4() != nil {
		return cfg.maxOffsetIPv4
	}
	if ip.To16() != nil {
		return cfg.maxOffsetIPv6
	}
	return nil
}
