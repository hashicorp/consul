package connect

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"net/url"
)

// CreateCSR returns a CSR to sign the given service along with the PEM-encoded
// private key for this certificate.
func CreateCSR(uri CertURI, privateKey crypto.Signer) (string, error) {
	template := &x509.CertificateRequest{
		URIs:               []*url.URL{uri.URI()},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	// Create the CSR itself
	var csrBuf bytes.Buffer
	bs, err := x509.CreateCertificateRequest(rand.Reader, template, privateKey)
	if err != nil {
		return "", err
	}

	err = pem.Encode(&csrBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: bs})
	if err != nil {
		return "", err
	}

	return csrBuf.String(), nil
}
