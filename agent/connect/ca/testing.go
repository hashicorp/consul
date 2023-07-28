// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package ca

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/go-testing-interface"
	"github.com/stretchr/testify/require"

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
var KeyTestCases = []KeyTestCase{
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

type KeyTestCase struct {
	Desc    string
	KeyType string
	KeyBits int
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
	logger := hclog.New(&hclog.LoggerOptions{Output: io.Discard})
	provider := &ConsulProvider{Delegate: d, logger: logger}
	return provider
}

// SkipIfVaultNotPresent skips the test if the vault binary is not in PATH.
//
// These tests may be skipped in CI. They are run as part of a separate
// integration test suite.
func SkipIfVaultNotPresent(t testing.T) {
	// Try to safeguard against tests that will never run in CI.
	// This substring should match the pattern used by the
	// test-connect-ca-providers CI job.
	if !strings.Contains(t.Name(), "Vault") {
		t.Fatalf("test name must contain Vault, otherwise CI will never run it")
	}

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
	vaultBinaryName := os.Getenv("VAULT_BINARY_NAME")
	if vaultBinaryName == "" {
		vaultBinaryName = "vault"
	}

	path, err := exec.LookPath(vaultBinaryName)
	if err != nil || path == "" {
		t.Fatalf("%q not found on $PATH", vaultBinaryName)
	}

	ports := freeport.RetryMustGetN(t, retry.Minute(), 2)
	var (
		clientAddr  = fmt.Sprintf("127.0.0.1:%d", ports[0])
		clusterAddr = fmt.Sprintf("127.0.0.1:%d", ports[1])
	)

	const token = "root"

	client, err := vaultapi.NewClient(&vaultapi.Config{
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
		// We pass '-dev-no-store-token' to avoid having multiple vaults oddly
		// interact and fail like this:
		//
		//   Error initializing Dev mode: rename /.vault-token.tmp /.vault-token: no such file or directory
		//
		"-dev-no-store-token",
	}

	cmd := exec.Command(vaultBinaryName, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	require.NoError(t, cmd.Start())

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

	testVault.WaitUntilReady(t)

	return testVault
}

type TestVaultServer struct {
	RootToken string
	Addr      string
	cmd       *exec.Cmd
	client    *vaultapi.Client
}

var printedVaultVersion sync.Once
var vaultTestVersion string

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
		vaultTestVersion = version
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
		if strings.Contains(err.Error(), "exec: Wait was already called") {
			return nil
		}
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

// The zero value implies unprivileged.
type VaultTokenAttributes struct {
	RootPath, IntermediatePath string

	ConsulManaged bool
	VaultManaged  bool
	WithSudo      bool

	CustomRules string
}

func (a *VaultTokenAttributes) DisplayName() string {
	switch {
	case a == nil:
		return "unprivileged"
	case a.CustomRules != "":
		return "custom"
	case a.ConsulManaged:
		return "consul-managed"
	case a.VaultManaged:
		return "vault-managed"
	default:
		return "unprivileged"
	}
}

func (a *VaultTokenAttributes) Rules(t testing.T) string {
	switch {
	case a == nil:
		return ""

	case a.CustomRules != "":
		return a.CustomRules

	case a.RootPath == "":
		t.Fatal("missing required RootPath")
		return "" // dead code

	case a.IntermediatePath == "":
		t.Fatal("missing required IntermediatePath")
		return "" // dead code

	case a.ConsulManaged:
		// Consul Managed PKI Mounts
		rules := fmt.Sprintf(`
path "sys/mounts" {
  capabilities = [ "read" ]
}

path "sys/mounts/%[1]s" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "sys/mounts/%[2]s" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

# Needed for Consul 1.11+
path "sys/mounts/%[2]s/tune" {
  capabilities = [ "update" ]
}

# vault token renewal
path "auth/token/renew-self" {
  capabilities = [ "update" ]
}
path "auth/token/lookup-self" {
  capabilities = [ "read" ]
}

path "%[1]s/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}

path "%[2]s/*" {
  capabilities = [ "create", "read", "update", "delete", "list" ]
}
`, a.RootPath, a.IntermediatePath)

		if a.WithSudo {
			rules += fmt.Sprintf(`

path "%[1]s/root/sign-self-issued" {
  capabilities = [ "sudo", "update" ]
}
`, a.RootPath)
		}

		return rules

	case a.VaultManaged:
		// Vault-managed PKI root.
		t.Fatal("TODO: implement this and use it in tests")
		return ""

	default:
		// zero value
		return ""
	}
}

func CreateVaultTokenWithAttrs(t testing.T, client *vaultapi.Client, attr *VaultTokenAttributes) string {
	policyName, err := uuid.GenerateUUID()
	require.NoError(t, err)

	rules := attr.Rules(t)

	token := createVaultTokenAndPolicy(t, client, policyName, rules)
	// t.Logf("created vault token with scope %q: %s", attr.DisplayName(), token)
	return token
}

func createVaultTokenAndPolicy(t testing.T, client *vaultapi.Client, policyName, policyRules string) string {
	require.NoError(t, client.Sys().PutPolicy(policyName, policyRules))

	renew := true
	tok, err := client.Auth().Token().Create(&vaultapi.TokenCreateRequest{
		Policies:  []string{policyName},
		Renewable: &renew,
	})
	require.NoError(t, err)
	return tok.Auth.ClientToken
}
