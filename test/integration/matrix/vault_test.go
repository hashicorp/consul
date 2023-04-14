package test

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
	"github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

func vaultClient(v TestVaultServer) (*api.Client, error) {
	return api.NewClient(&api.Config{})
}

// Holds a running server and connected client.
// Plus anscillary data useful in testing.
type TestVaultServer struct {
	RootToken string
	Addr      string
	cmd       *exec.Cmd
	client    *api.Client
}

// NewTestVaultServer runs the Vault binary given on the path and returning
// TestVaultServer object which encaptulates the server and a connected client.
func NewTestVaultServer(t *testing.T, path string) TestVaultServer {
	path, err := exec.LookPath(path)
	if err != nil || path == "" {
		t.Fatalf("%q not found on $PATH", path)
	}

	ports := freeport.GetN(t, 2)
	var (
		clientAddr  = fmt.Sprintf("127.0.0.1:%d", ports[0])
		clusterAddr = fmt.Sprintf("127.0.0.1:%d", ports[1])
	)

	const token = "root"

	client, err := api.NewClient(&api.Config{
		Address: "http://" + clientAddr,
	})
	require.NoError(t, err)
	client.SetToken(token)

	args := []string{
		"server",
		"-dev",
		"-dev-root-token-id",
		token,
		"-dev-listen-address",
		clientAddr,
		"-address",
		clusterAddr,
		"-dev-no-store-token",
	}

	cmd := exec.Command(path, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())

	testVault := TestVaultServer{
		RootToken: token,
		Addr:      "http://" + clientAddr,
		cmd:       cmd,
		client:    client,
	}

	testVault.WaitUntilReady(t)

	return testVault
}

func (v TestVaultServer) Client() *api.Client {
	return v.client
}

func (v TestVaultServer) WaitUntilReady(t *testing.T) {
	var version string
	retry.Run(t, func(r *retry.R) {
		resp, err := v.client.Sys().Health()
		if err != nil {
			r.Fatalf("err: %v", err)
		}
		if !resp.Initialized {
			r.Fatalf("vault server is not initialized")
		}
		if resp.Sealed {
			r.Fatalf("vault server is sealed")
		}
		version = resp.Version
	})
	fmt.Fprintf(os.Stderr, "[INFO] agent/connect/ca: testing with vault server version: %s\n", version)
}

func (v TestVaultServer) Stop() error {
	// There was no process
	if v.cmd == nil {
		return nil
	}

	if v.cmd.Process != nil {
		err := v.cmd.Process.Signal(os.Interrupt)
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("failed to kill vault server: %v", err)
		}
	}

	// wait for the process to exit to be sure that the data dir can be
	// deleted on all platforms.
	if err := v.cmd.Wait(); err != nil {
		if strings.Contains(err.Error(), "exec: Wait was already called") {
			return nil
		}
		return err
	}
	return nil
}
