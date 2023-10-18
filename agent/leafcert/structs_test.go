package leafcert

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectCALeafRequest_Key(t *testing.T) {
	key := func(r ConnectCALeafRequest) string {
		return r.Key()
	}
	t.Run("service", func(t *testing.T) {
		t.Run("name", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Service: "web"})
			r2 := key(ConnectCALeafRequest{Service: "api"})
			require.True(t, strings.HasPrefix(r1, "service:"), "Key %s does not start with service:", r1)
			require.True(t, strings.HasPrefix(r2, "service:"), "Key %s does not start with service:", r2)
			require.NotEqual(t, r1, r2, "Cache keys for different services should not be equal")
		})
		t.Run("dns-san", func(t *testing.T) {
			r3 := key(ConnectCALeafRequest{Service: "foo", DNSSAN: []string{"a.com"}})
			r4 := key(ConnectCALeafRequest{Service: "foo", DNSSAN: []string{"b.com"}})
			require.NotEqual(t, r3, r4, "Cache keys for different DNSSAN should not be equal")
		})
		t.Run("ip-san", func(t *testing.T) {
			r5 := key(ConnectCALeafRequest{Service: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
			r6 := key(ConnectCALeafRequest{Service: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
			require.NotEqual(t, r5, r6, "Cache keys for different IPSAN should not be equal")
		})
	})
	t.Run("agent", func(t *testing.T) {
		t.Run("name", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Agent: "abc"})
			require.True(t, strings.HasPrefix(r1, "agent:"), "Key %s does not start with agent:", r1)
		})
		t.Run("dns-san ignored", func(t *testing.T) {
			r3 := key(ConnectCALeafRequest{Agent: "foo", DNSSAN: []string{"a.com"}})
			r4 := key(ConnectCALeafRequest{Agent: "foo", DNSSAN: []string{"b.com"}})
			require.Equal(t, r3, r4, "DNSSAN is ignored for agent type")
		})
		t.Run("ip-san ignored", func(t *testing.T) {
			r5 := key(ConnectCALeafRequest{Agent: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
			r6 := key(ConnectCALeafRequest{Agent: "foo", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
			require.Equal(t, r5, r6, "IPSAN is ignored for agent type")
		})
	})
	t.Run("kind", func(t *testing.T) {
		t.Run("invalid", func(t *testing.T) {
			r1 := key(ConnectCALeafRequest{Kind: "terminating-gateway"})
			require.Empty(t, r1)
		})
		t.Run("mesh-gateway", func(t *testing.T) {
			t.Run("normal", func(t *testing.T) {
				r1 := key(ConnectCALeafRequest{Kind: "mesh-gateway"})
				require.True(t, strings.HasPrefix(r1, "kind:"), "Key %s does not start with kind:", r1)
			})
			t.Run("dns-san", func(t *testing.T) {
				r3 := key(ConnectCALeafRequest{Kind: "mesh-gateway", DNSSAN: []string{"a.com"}})
				r4 := key(ConnectCALeafRequest{Kind: "mesh-gateway", DNSSAN: []string{"b.com"}})
				require.NotEqual(t, r3, r4, "Cache keys for different DNSSAN should not be equal")
			})
			t.Run("ip-san", func(t *testing.T) {
				r5 := key(ConnectCALeafRequest{Kind: "mesh-gateway", IPSAN: []net.IP{net.ParseIP("192.168.4.139")}})
				r6 := key(ConnectCALeafRequest{Kind: "mesh-gateway", IPSAN: []net.IP{net.ParseIP("192.168.4.140")}})
				require.NotEqual(t, r5, r6, "Cache keys for different IPSAN should not be equal")
			})
		})
	})
	t.Run("server", func(t *testing.T) {
		r1 := key(ConnectCALeafRequest{
			Server:     true,
			Datacenter: "us-east",
		})
		require.True(t, strings.HasPrefix(r1, "server:"), "Key %s does not start with server:", r1)
	})
}
