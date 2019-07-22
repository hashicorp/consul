package connect

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"net/url"
)

// CreateCSR returns a CSR to sign the given service along with the PEM-encoded
// private key for this certificate.
func CreateCSR(commonName string, uri CertURI, privateKey crypto.Signer,
	extensions ...pkix.Extension) (string, error) {

	signAlgo := x509.SHA256WithRSA
	_, ok := privateKey.(*ecdsa.PrivateKey)
	if ok {
		signAlgo = x509.ECDSAWithSHA256
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: commonName,
		},
		URIs:               []*url.URL{uri.URI()},
		SignatureAlgorithm: signAlgo,
		ExtraExtensions:    extensions,
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

// CreateCSR returns a CA CSR to sign the given service along with the PEM-encoded
// private key for this certificate.
func CreateCACSR(uri CertURI, privateKey crypto.Signer) (string, error) {
	ext, err := CreateCAExtension()
	if err != nil {
		return "", err
	}

	return CreateCSR("Consul CA", uri, privateKey, ext)
}

// CreateCAExtension creates a pkix.Extension for the x509 Basic Constraints
// IsCA field ()
func CreateCAExtension() (pkix.Extension, error) {
	type basicConstraints struct {
		IsCA       bool `asn1:"optional"`
		MaxPathLen int  `asn1:"optional"`
	}
	basicCon := basicConstraints{IsCA: true, MaxPathLen: 0}
	bitstr, err := asn1.Marshal(basicCon)
	if err != nil {
		return pkix.Extension{}, err
	}

	return pkix.Extension{
		Id:       []int{2, 5, 29, 19}, // from x509 package
		Critical: true,
		Value:    bitstr,
	}, nil
}
