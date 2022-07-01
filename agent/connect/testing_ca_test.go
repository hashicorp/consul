package connect

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var mustAlwaysRun = os.Getenv("CI") == "true"

func skipIfMissingOpenSSL(t *testing.T) {
	openSSLBinaryName := "openssl"
	_, err := exec.LookPath(openSSLBinaryName)
	if err != nil {
		if mustAlwaysRun {
			t.Fatalf("%q not found on $PATH", openSSLBinaryName)
		}
		t.Skipf("%q not found on $PATH", openSSLBinaryName)
	}
}

// Test that the TestCA and TestLeaf functions generate valid certificates.
func testCAAndLeaf(t *testing.T, keyType string, keyBits int) {
	skipIfMissingOpenSSL(t)

	// Create the certs
	ca := TestCAWithKeyType(t, nil, keyType, keyBits)
	leaf, _ := TestLeaf(t, "web", ca)

	// Create a temporary directory for storing the certs
	td, err := os.MkdirTemp("", "consul")
	require.NoError(t, err)
	defer os.RemoveAll(td)

	// Write the cert
	require.NoError(t, os.WriteFile(filepath.Join(td, "ca.pem"), []byte(ca.RootCert), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(td, "leaf.pem"), []byte(leaf[:]), 0644))

	// Use OpenSSL to verify so we have an external, known-working process
	// that can verify this outside of our own implementations.
	cmd := exec.Command(
		"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf.pem")
	cmd.Dir = td
	output, err := cmd.Output()
	t.Log("STDOUT:", string(output))
	if ee, ok := err.(*exec.ExitError); ok {
		t.Log("STDERR:", string(ee.Stderr))
	}
	require.NoError(t, err)
}

// Test cross-signing.
func testCAAndLeaf_xc(t *testing.T, keyType string, keyBits int) {
	skipIfMissingOpenSSL(t)

	// Create the certs
	ca1 := TestCAWithKeyType(t, nil, keyType, keyBits)
	ca2 := TestCAWithKeyType(t, ca1, keyType, keyBits)
	leaf1, _ := TestLeaf(t, "web", ca1)
	leaf2, _ := TestLeaf(t, "web", ca2)

	// Create a temporary directory for storing the certs
	td, err := os.MkdirTemp("", "consul")
	assert.Nil(t, err)
	defer os.RemoveAll(td)

	// Write the cert
	xcbundle := []byte(ca1.RootCert)
	xcbundle = append(xcbundle, '\n')
	xcbundle = append(xcbundle, []byte(ca2.SigningCert)...)
	assert.Nil(t, os.WriteFile(filepath.Join(td, "ca.pem"), xcbundle, 0644))
	assert.Nil(t, os.WriteFile(filepath.Join(td, "leaf1.pem"), []byte(leaf1), 0644))
	assert.Nil(t, os.WriteFile(filepath.Join(td, "leaf2.pem"), []byte(leaf2), 0644))

	// OpenSSL verify the cross-signed leaf (leaf2)
	{
		cmd := exec.Command(
			"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf2.pem")
		cmd.Dir = td
		output, err := cmd.Output()
		t.Log(string(output))
		assert.Nil(t, err)
	}

	// OpenSSL verify the old leaf (leaf1)
	{
		cmd := exec.Command(
			"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf1.pem")
		cmd.Dir = td
		output, err := cmd.Output()
		t.Log(string(output))
		assert.Nil(t, err)
	}
}

func TestTestCAAndLeaf(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, params := range goodParams {
		t.Run(fmt.Sprintf("TestTestCAAndLeaf-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				testCAAndLeaf(t, params.keyType, params.keyBits)
			})
	}
}

func TestTestCAAndLeaf_xc(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	t.Parallel()
	for _, params := range goodParams {
		t.Run(fmt.Sprintf("TestTestCAAndLeaf_xc-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				testCAAndLeaf_xc(t, params.keyType, params.keyBits)
			})
	}
}
