package connect

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// hasOpenSSL is used to determine if the openssl CLI exists for unit tests.
var hasOpenSSL bool

func init() {
	goodParams = []KeyConfig{
		{keyType: "rsa", keyBits: 2048},
		{keyType: "rsa", keyBits: 4096},
		{keyType: "ec", keyBits: 224},
		{keyType: "ec", keyBits: 256},
		{keyType: "ec", keyBits: 384},
		{keyType: "ec", keyBits: 521},
	}

	_, err := exec.LookPath("openssl")
	hasOpenSSL = err == nil
}

// Test that the TestCA and TestLeaf functions generate valid certificates.
func testCAAndLeaf(t *testing.T, keyType string, keyBits int) {
	if !hasOpenSSL {
		t.Skip("openssl not found")
		return
	}

	assert := assert.New(t)

	// Create the certs
	ca := TestCAWithKeyType(t, nil, keyType, keyBits)
	leaf, _ := TestLeaf(t, "web", ca)

	// Create a temporary directory for storing the certs
	td, err := ioutil.TempDir("", "consul")
	assert.Nil(err)
	defer os.RemoveAll(td)

	// Write the cert
	assert.Nil(ioutil.WriteFile(filepath.Join(td, "ca.pem"), []byte(ca.RootCert), 0644))
	assert.Nil(ioutil.WriteFile(filepath.Join(td, "leaf.pem"), []byte(leaf), 0644))

	// Use OpenSSL to verify so we have an external, known-working process
	// that can verify this outside of our own implementations.
	cmd := exec.Command(
		"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf.pem")
	cmd.Dir = td
	output, err := cmd.Output()
	t.Log(string(output))
	assert.Nil(err)
}

// Test cross-signing.
func testCAAndLeaf_xc(t *testing.T, keyType string, keyBits int) {
	if !hasOpenSSL {
		t.Skip("openssl not found")
		return
	}

	assert := assert.New(t)

	// Create the certs
	ca1 := TestCAWithKeyType(t, nil, keyType, keyBits)
	ca2 := TestCAWithKeyType(t, ca1, keyType, keyBits)
	leaf1, _ := TestLeaf(t, "web", ca1)
	leaf2, _ := TestLeaf(t, "web", ca2)

	// Create a temporary directory for storing the certs
	td, err := ioutil.TempDir("", "consul")
	assert.Nil(err)
	defer os.RemoveAll(td)

	// Write the cert
	xcbundle := []byte(ca1.RootCert)
	xcbundle = append(xcbundle, '\n')
	xcbundle = append(xcbundle, []byte(ca2.SigningCert)...)
	assert.Nil(ioutil.WriteFile(filepath.Join(td, "ca.pem"), xcbundle, 0644))
	assert.Nil(ioutil.WriteFile(filepath.Join(td, "leaf1.pem"), []byte(leaf1), 0644))
	assert.Nil(ioutil.WriteFile(filepath.Join(td, "leaf2.pem"), []byte(leaf2), 0644))

	// OpenSSL verify the cross-signed leaf (leaf2)
	{
		cmd := exec.Command(
			"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf2.pem")
		cmd.Dir = td
		output, err := cmd.Output()
		t.Log(string(output))
		assert.Nil(err)
	}

	// OpenSSL verify the old leaf (leaf1)
	{
		cmd := exec.Command(
			"openssl", "verify", "-verbose", "-CAfile", "ca.pem", "leaf1.pem")
		cmd.Dir = td
		output, err := cmd.Output()
		t.Log(string(output))
		assert.Nil(err)
	}
}

func TestTestCAAndLeaf(t *testing.T) {
	t.Parallel()
	for _, params := range goodParams {
		t.Run(fmt.Sprintf("TestTestCAAndLeaf-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				testCAAndLeaf(t, params.keyType, params.keyBits)
			})
	}
}

func TestTestCAAndLeaf_xc(t *testing.T) {
	t.Parallel()
	for _, params := range goodParams {
		t.Run(fmt.Sprintf("TestTestCAAndLeaf_xc-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				testCAAndLeaf_xc(t, params.keyType, params.keyBits)
			})
	}
}
