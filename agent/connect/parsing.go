// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package connect

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
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

// ParseLeafCerts parses all of the x509 certificates from a PEM-encoded value
// under the assumption that the first cert is a leaf (non-CA) cert and the
// rest are intermediate CA certs.
//
// If no certificates are found this returns an error.
func ParseLeafCerts(pemValue string) (*x509.Certificate, *x509.CertPool, error) {
	certs, err := parseCerts(pemValue)
	if err != nil {
		return nil, nil, err
	}

	leaf := certs[0]
	if leaf.IsCA {
		return nil, nil, fmt.Errorf("first PEM-block should be a leaf cert")
	}

	intermediates := x509.NewCertPool()
	for _, cert := range certs[1:] {
		if !cert.IsCA {
			return nil, nil, fmt.Errorf("found an unexpected leaf cert after the first PEM-block")
		}
		intermediates.AddCert(cert)
	}

	return leaf, intermediates, nil
}

// CertSubjects can be used in debugging to return the subject of each
// certificate in the PEM bundle. Each subject is separated by a newline.
func CertSubjects(pem string) string {
	certs, err := parseCerts(pem)
	if err != nil {
		return err.Error()
	}
	var buf strings.Builder
	for _, cert := range certs {
		buf.WriteString(cert.Subject.String())
		buf.WriteString("\n")
	}
	return buf.String()
}

// ParseCerts parses the all x509 certificates from a PEM-encoded value.
// The first returned cert is a leaf cert and any other ones are intermediates.
//
// If no certificates are found this returns an error.
func parseCerts(pemValue string) ([]*x509.Certificate, error) {
	var out []*x509.Certificate

	rest := []byte(pemValue)
	for {
		// The _ result below is not an error but the remaining PEM bytes.
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = remaining

		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("PEM-block should be CERTIFICATE type")
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, cert)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("no PEM-encoded data found")
	}

	return out, nil
}

// CalculateCertFingerprint calculates the SHA-1 fingerprint from the cert bytes.
func CalculateCertFingerprint(cert []byte) string {
	hash := sha1.Sum(cert)
	return HexString(hash[:])
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
// must be the first block in the PEM value.
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
	case *rsa.PublicKey:
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

	kID := sha256.Sum256(bs)
	return kID[:], nil
}

// EncodeSerialNumber encodes the given serial number as a colon-hex encoded
// string.
func EncodeSerialNumber(serial *big.Int) string {
	return HexString(serial.Bytes())
}

// EncodeSigningKeyID encodes the given AuthorityKeyId or SubjectKeyId into a
// colon-hex encoded string suitable for using as a SigningKeyID value.
func EncodeSigningKeyID(keyID []byte) string { return HexString(keyID) }

// HexString returns a standard colon-separated hex value for the input
// byte slice. This should be used with cert serial numbers and so on.
func HexString(input []byte) string {
	return strings.Replace(fmt.Sprintf("% x", input), " ", ":", -1)
}

// IsHexString returns true if the input is the output of HexString(). Meant
// for use in tests.
func IsHexString(input []byte) bool {
	s := string(input)
	if strings.Count(s, ":") < 5 { // 5 is arbitrary
		return false
	}

	s = strings.ReplaceAll(s, ":", "")
	_, err := hex.DecodeString(s)
	return err == nil
}

// KeyInfoFromCert returns the key type and key bit length for the key used by
// the certificate.
func KeyInfoFromCert(cert *x509.Certificate) (keyType string, keyBits int, err error) {
	switch k := cert.PublicKey.(type) {
	case *ecdsa.PublicKey:
		return "ec", k.Curve.Params().BitSize, nil
	case *rsa.PublicKey:
		return "rsa", k.N.BitLen(), nil
	default:
		return "", 0, fmt.Errorf("unsupported key type")
	}
}
