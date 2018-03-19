package connect

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
)

// ParseCert parses the x509 certificate from a PEM-encoded value.
func ParseCert(pemValue string) (*x509.Certificate, error) {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("first PEM-block should be CERTIFICATE type")
	}

	return x509.ParseCertificate(block.Bytes)
}

// ParseSigner parses a crypto.Signer from a PEM-encoded key. The private key
// is expected to be the first block in the PEM value.
func ParseSigner(pemValue string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)

	default:
		return nil, fmt.Errorf("unknown PEM block type for signing key: %s", block.Type)
	}
}

// SerialNumber generates a serial number suitable for a certificate.
//
// This function is taken directly from the Vault implementation.
func SerialNumber() (*big.Int, error) {
	return rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
}
