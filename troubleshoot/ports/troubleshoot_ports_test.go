package ports

import (
	"fmt"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/stretchr/testify/require"
	"strconv"
	"strings"
	"testing"
)

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
	results := TroubleShootCustomPorts("127.0.0.1", strings.Join([]string{"8777", "8888"}, ","))

	expectedResults := []string{
		fmt.Sprintf("TCP: Port 8777 on 127.0.0.1 is closed, unreachable, or the connection timed out.\n"),
		fmt.Sprintf("TCP: Port 8888 on 127.0.0.1 is closed, unreachable, or the connection timed out.\n"),
	}
	for _, res := range expectedResults {
		require.Contains(t, results, res)
	}
}
