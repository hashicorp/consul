package connect

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReloadableTLSConfig(t *testing.T) {
	base := TestTLSConfig(t, "ca1", "web")

	c := NewReloadableTLSConfig(base)

	a := &TestAuther{
		Return: nil,
	}

	// The dynamic config should be the one we loaded, but with the passed auther
	expect := base
	expect.VerifyPeerCertificate = a.Auth
	require.Equal(t, base, c.TLSConfig(a))

	// The server config should return same too for new connections
	serverCfg := c.ServerTLSConfig()
	require.NotNil(t, serverCfg.GetConfigForClient)
	got, err := serverCfg.GetConfigForClient(&tls.ClientHelloInfo{})
	require.Nil(t, err)
	require.Equal(t, base, got)

	// Now change the config as if we just rotated to a new CA
	new := TestTLSConfig(t, "ca2", "web")
	err = c.SetTLSConfig(new)
	require.Nil(t, err)

	// The dynamic config should be the one we loaded (with same auther due to nil)
	require.Equal(t, new, c.TLSConfig(nil))

	// The server config should return same too for new connections
	serverCfg = c.ServerTLSConfig()
	require.NotNil(t, serverCfg.GetConfigForClient)
	got, err = serverCfg.GetConfigForClient(&tls.ClientHelloInfo{})
	require.Nil(t, err)
	require.Equal(t, new, got)
}
