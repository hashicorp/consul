package connect

import (
	"fmt"
	"testing"
	"time"

	"crypto/x509"
	"encoding/pem"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/stretchr/testify/require"
)

type KeyConfig struct {
	keyType string
	keyBits int
}

var goodParams, badParams []KeyConfig

func init() {
	goodParams = []KeyConfig{
		{keyType: "rsa", keyBits: 2048},
		{keyType: "rsa", keyBits: 4096},
		{keyType: "ec", keyBits: 224},
		{keyType: "ec", keyBits: 256},
		{keyType: "ec", keyBits: 384},
		{keyType: "ec", keyBits: 521},
	}
	badParams = []KeyConfig{
		{keyType: "rsa", keyBits: 0},
		{keyType: "rsa", keyBits: 1024},
		{keyType: "rsa", keyBits: 24601},
		{keyType: "ec", keyBits: 0},
		{keyType: "ec", keyBits: 512},
		{keyType: "ec", keyBits: 321},
		{keyType: "ecdsa", keyBits: 256}, // test for "ecdsa" instead of "ec"
		{keyType: "aes", keyBits: 128},
	}
}

func makeConfig(kc KeyConfig) structs.CommonCAProviderConfig {
	return structs.CommonCAProviderConfig{
		LeafCertTTL:    3 * 24 * time.Hour,
		PrivateKeyType: kc.keyType,
		PrivateKeyBits: kc.keyBits,
	}
}

func testGenerateRSAKey(t *testing.T, bits int) {
	r := require.New(t)
	_, rsaBlock, err := GeneratePrivateKeyWithConfig("rsa", bits)
	r.NoError(err)
	r.Contains(rsaBlock, "RSA PRIVATE KEY")

	rsaBytes, _ := pem.Decode([]byte(rsaBlock))
	r.NotNil(rsaBytes)

	rsaKey, err := x509.ParsePKCS1PrivateKey(rsaBytes.Bytes)
	r.NoError(err)
	r.NoError(rsaKey.Validate())
	r.Equal(bits/8, rsaKey.Size()) // note: returned size is in bytes. 2048/8==256
}

func testGenerateECDSAKey(t *testing.T, bits int) {
	r := require.New(t)
	_, pemBlock, err := GeneratePrivateKeyWithConfig("ec", bits)
	r.NoError(err)
	r.Contains(pemBlock, "EC PRIVATE KEY")

	block, _ := pem.Decode([]byte(pemBlock))
	r.NotNil(block)

	pk, err := x509.ParseECPrivateKey(block.Bytes)
	r.NoError(err)
	r.Equal(bits, pk.Curve.Params().BitSize)
}

// Tests to make sure we are able to generate every type of private key supported by the x509 lib.
func TestGenerateKeys(t *testing.T) {
	t.Parallel()
	for _, params := range goodParams {
		t.Run(fmt.Sprintf("TestGenerateKeys-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				switch params.keyType {
				case "rsa":
					testGenerateRSAKey(t, params.keyBits)
				case "ec":
					testGenerateECDSAKey(t, params.keyBits)
				default:
					t.Fatalf("unkown key type: %s", params.keyType)
				}
			})
	}
}

// Tests a variety of valid private key configs to make sure they're accepted.
func TestValidateGoodConfigs(t *testing.T) {
	t.Parallel()
	for _, params := range goodParams {
		config := makeConfig(params)
		t.Run(fmt.Sprintf("TestValidateGoodConfigs-%s-%d", params.keyType, params.keyBits),
			func(t *testing.T) {
				require.New(t).NoError(config.Validate(), "unexpected error: type=%s bits=%d",
					params.keyType, params.keyBits)
			})

	}
}

// Tests a variety of invalid private key configs to make sure they're caught.
func TestValidateBadConfigs(t *testing.T) {
	t.Parallel()
	for _, params := range badParams {
		config := makeConfig(params)
		t.Run(fmt.Sprintf("TestValidateBadConfigs-%s-%d", params.keyType, params.keyBits), func(t *testing.T) {
			require.New(t).Error(config.Validate(), "expected error: type=%s bits=%d",
				params.keyType, params.keyBits)
		})
	}
}

// Tests the ability of a CA to sign a CSR using a different key type. If the key types differ, the test should fail.
func TestSignatureMismatches(t *testing.T) {
	t.Parallel()
	r := require.New(t)
	for _, p1 := range goodParams {
		for _, p2 := range goodParams {
			if p1 == p2 {
				continue
			}
			t.Run(fmt.Sprintf("TestMismatches-%s%d-%s%d", p1.keyType, p1.keyBits, p2.keyType, p2.keyBits), func(t *testing.T) {
				ca := TestCAWithKeyType(t, nil, p1.keyType, p1.keyBits)
				r.Equal(p1.keyType, ca.PrivateKeyType)
				r.Equal(p1.keyBits, ca.PrivateKeyBits)
				certPEM, keyPEM, err := testLeaf(t, "foobar.service.consul", ca, p2.keyType, p2.keyBits)
				if p1.keyType == p2.keyType {
					r.NoError(err)
					_, err := ParseCert(certPEM)
					r.NoError(err)
					_, err = ParseSigner(keyPEM)
					r.NoError(err)
				} else {
					r.Error(err)
				}
			})
		}
	}
}
