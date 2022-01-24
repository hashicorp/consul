package ca

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-testing-interface"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/sdk/freeport"
	"github.com/hashicorp/consul/sdk/testutil/retry"
)

// KeyTestCases is a list of the important CA key types that we should test
// against when signing. For now leaf keys are always EC P256 but CA can be EC
// (any NIST curve) or RSA (2048, 4096). Providers must be able to complete all
// signing operations with both types that includes:
//   - Sign must be able to sign EC P256 leaf with all these types of CA key
//   - CrossSignCA must be able to sign all these types of new CA key with all
//     these types of old CA key.
//   - SignIntermediate muse bt able to sign all the types of secondary
//     intermediate CA key with all these types of primary CA key
var KeyTestCases = []struct {
	Desc    string
	KeyType string
	KeyBits int
}{
	{
		Desc:    "Default Key Type (EC 256)",
		KeyType: connect.DefaultPrivateKeyType,
		KeyBits: connect.DefaultPrivateKeyBits,
	},
	{
		Desc:    "RSA 2048",
		KeyType: "rsa",
		KeyBits: 2048,
	},
}

// CASigningKeyTypes is a struct with params for tests that sign one CA CSR with
// another CA key.
type CASigningKeyTypes struct {
	Desc           string
	SigningKeyType string
	SigningKeyBits int
	CSRKeyType     string
	CSRKeyBits     int
}

// CASigningKeyTypeCases returns the cross-product of the important supported CA
// key types for generating table tests for CA signing tests (CrossSignCA and
// SignIntermediate).
func CASigningKeyTypeCases() []CASigningKeyTypes {
	cases := make([]CASigningKeyTypes, 0, len(KeyTestCases)*len(KeyTestCases))
	for _, outer := range KeyTestCases {
		for _, inner := range KeyTestCases {
			cases = append(cases, CASigningKeyTypes{
				Desc: fmt.Sprintf("%s-%d signing %s-%d", outer.KeyType, outer.KeyBits,
					inner.KeyType, inner.KeyBits),
				SigningKeyType: outer.KeyType,
				SigningKeyBits: outer.KeyBits,
				CSRKeyType:     inner.KeyType,
				CSRKeyBits:     inner.KeyBits,
			})
		}
	}
	return cases
}

// TestConsulProvider creates a new ConsulProvider, taking care to stub out it's
// Logger so that logging calls don't panic. If logging output is important
func TestConsulProvider(t testing.T, d ConsulProviderStateDelegate) *ConsulProvider {
	logger := hclog.New(&hclog.LoggerOptions{Output: ioutil.Discard})
	provider := &ConsulProvider{Delegate: d, logger: logger}
	return provider
}

// SkipIfVaultNotPresent skips the test if the vault binary is not in PATH.
//
// These tests may be skipped in CI. They are run as part of a separate
// integration test suite.
func SkipIfVaultNotPresent(t testing.T) {
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		t.Skipf("%q not found on $PATH - download and install to run this test", vaultBinaryName)
	}
}

func NewTestVaultServer(t testing.T) *TestVaultServer {
	testVault, err := runTestVault(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}

	testVault.WaitUntilReady(t)

	return testVault
}

func runTestVault(t testing.T) (*TestVaultServer, error) {
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		return nil, fmt.Errorf("%q not found on $PATH", vaultBinaryName)
	}

	ports := freeport.GetN(t, 2)
	var (
		clientAddr  = fmt.Sprintf("127.0.0.1:%d", ports[0])
		clusterAddr = fmt.Sprintf("127.0.0.1:%d", ports[1])
	)

	const token = "root"

	client, err := vaultapi.NewClient(&vaultapi.Config{
		Address: "http://" + clientAddr,
	})
	if err != nil {
		return nil, err
	}
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
	}

	cmd := exec.Command(vaultBinaryName, args...)
	cmd.Stdout = ioutil.Discard
	cmd.Stderr = ioutil.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	testVault := &TestVaultServer{
		RootToken: token,
		Addr:      "http://" + clientAddr,
		cmd:       cmd,
		client:    client,
	}
	t.Cleanup(func() {
		if err := testVault.Stop(); err != nil {
			t.Logf("failed to stop vault server: %v", err)
		}
	})

	return testVault, nil
}

type TestVaultServer struct {
	RootToken string
	Addr      string
	cmd       *exec.Cmd
	client    *vaultapi.Client
}

var printedVaultVersion sync.Once

func (v *TestVaultServer) Client() *vaultapi.Client {
	return v.client
}

func (v *TestVaultServer) WaitUntilReady(t testing.T) {
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
	printedVaultVersion.Do(func() {
		fmt.Fprintf(os.Stderr, "[INFO] agent/connect/ca: testing with vault server version: %s\n", version)
	})
}

func (v *TestVaultServer) Stop() error {
	// There was no process
	if v.cmd == nil {
		return nil
	}

	if v.cmd.Process != nil {
		if err := v.cmd.Process.Signal(os.Interrupt); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("failed to kill vault server: %v", err)
		}
	}

	// wait for the process to exit to be sure that the data dir can be
	// deleted on all platforms.
	if err := v.cmd.Wait(); err != nil {
		return err
	}
	return nil
}

func requireTrailingNewline(t testing.T, leafPEM string) {
	t.Helper()
	if len(leafPEM) == 0 {
		t.Fatalf("cert is empty")
	}
	if '\n' != rune(leafPEM[len(leafPEM)-1]) {
		t.Fatalf("cert do not end with a new line")
	}
}
