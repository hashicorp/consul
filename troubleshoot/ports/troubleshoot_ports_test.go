// Copyright IBM Corp. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package ports

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
)

// getFreePort binds to :0 on loopback to obtain a port the OS guarantees is
// free at the moment of the call, then immediately closes the listener so the
// port is available to use. Using OS-allocated ports avoids collisions with
// well-known ports (e.g. 8888) that may already be bound on CI runners.
func getFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "getFreePort: failed to bind")
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close(), "getFreePort: failed to close listener")
	return port
}

func TestTroubleShootCustom_Ports(t *testing.T) {
	// Create a test Consul server
	srv1, err := testutil.NewTestServerConfigT(t, nil)
	if err != nil {
		t.Fatal(err)
	}

	results := TroubleShootCustomPorts("127.0.0.1", strings.Join([]string{
		strconv.Itoa(srv1.Config.Ports.HTTP),
		strconv.Itoa(srv1.Config.Ports.DNS),
		strconv.Itoa(srv1.Config.Ports.HTTPS),
		strconv.Itoa(srv1.Config.Ports.GRPC),
		strconv.Itoa(srv1.Config.Ports.SerfLan),
		strconv.Itoa(srv1.Config.Ports.SerfWan),
		strconv.Itoa(srv1.Config.Ports.Server)}, ","))
	expectedResults := []string{
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.HTTP)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.GRPC)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.HTTPS)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.SerfLan)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.SerfWan)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.DNS)),
		fmt.Sprintf("TCP: Port %s on 127.0.0.1 is open.\n", strconv.Itoa(srv1.Config.Ports.Server)),
	}
	for _, res := range expectedResults {
		require.Contains(t, results, res)
	}
	defer srv1.Stop()
}

func TestTroubleShootCustom_Ports_Not_Reachable(t *testing.T) {
	// Use OS-allocated ports that are guaranteed free rather than hardcoded
	// values like 8888 which may already be bound on the CI runner.
	port1 := getFreePort(t)
	port2 := getFreePort(t)

	results := TroubleShootCustomPorts("127.0.0.1", strings.Join([]string{
		strconv.Itoa(port1),
		strconv.Itoa(port2),
	}, ","))

	expectedResults := []string{
		fmt.Sprintf("TCP: Port %d on 127.0.0.1 is closed, unreachable, or the connection timed out.\n", port1),
		fmt.Sprintf("TCP: Port %d on 127.0.0.1 is closed, unreachable, or the connection timed out.\n", port2),
	}
	for _, res := range expectedResults {
		require.Contains(t, results, res)
	}
}
