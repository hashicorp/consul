package connect

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"
)

// ParseCert parses the x509 certificate from a PEM-encoded value.
func ParseCert(pemValue string) (*x509.Certificate, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("first PEM-block should be CERTIFICATE type")
	}

	return x509.ParseCertificate(block.Bytes)
}

// CalculateCertFingerprint parses the x509 certificate from a PEM-encoded value
// and calculates the SHA-1 fingerprint.
func CalculateCertFingerprint(pemValue string) (string, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return "", fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE" {
		return "", fmt.Errorf("first PEM-block should be CERTIFICATE type")
	}

	hash := sha1.Sum(block.Bytes)
	return HexString(hash[:]), nil
}

// ParseSigner parses a crypto.Signer from a PEM-encoded key. The private key
// is expected to be the first block in the PEM value.
func ParseSigner(pemValue string) (crypto.Signer, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	switch block.Type {
	case "EC PRIVATE KEY":
		return x509.ParseECPrivateKey(block.Bytes)

	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)

	case "PRIVATE KEY":
		signer, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		pk, ok := signer.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("private key is not a valid format")
		}

		return pk, nil

	default:
		return nil, fmt.Errorf("unknown PEM block type for signing key: %s", block.Type)
	}
}

// ParseCSR parses a CSR from a PEM-encoded value. The certificate request
// must be the the first block in the PEM value.
func ParseCSR(pemValue string) (*x509.CertificateRequest, error) {
	// The _ result below is not an error but the remaining PEM bytes.
	block, _ := pem.Decode([]byte(pemValue))
	if block == nil {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	if block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("first PEM-block should be CERTIFICATE REQUEST type")
	}

	return x509.ParseCertificateRequest(block.Bytes)
}

// KeyId returns a x509 KeyId from the given signing key. The key must be
// an *ecdsa.PublicKey currently, but may support more types in the future.
func KeyId(raw interface{}) ([]byte, error) {
	switch raw.(type) {
	case *ecdsa.PublicKey:
	default:
		return nil, fmt.Errorf("invalid key type: %T", raw)
	}

	// This is not standard; RFC allows any unique identifier as long as they
	// match in subject/authority chains but suggests specific hashing of DER
	// bytes of public key including DER tags.
	bs, err := x509.MarshalPKIXPublicKey(raw)
	if err != nil {
		return nil, err
	}

	// String formatted
	kID := sha256.Sum256(bs)
	return []byte(strings.Replace(fmt.Sprintf("% x", kID), " ", ":", -1)), nil
}

// HexString returns a standard colon-separated hex value for the input
// byte slice. This should be used with cert serial numbers and so on.
func HexString(input []byte) string {
	return strings.Replace(fmt.Sprintf("% x", input), " ", ":", -1)
}
