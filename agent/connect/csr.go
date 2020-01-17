package connect

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"net"
	"net/url"
)

// SigAlgoForKey returns the preferred x509.SignatureAlgorithm for a given key
// based on it's type. If the key type is not supported we return
// ECDSAWithSHA256 on the basis that it will fail anyway and we've already type
// checked keys by the time we call this in general.
func SigAlgoForKey(key crypto.Signer) x509.SignatureAlgorithm {
	if _, ok := key.(*rsa.PrivateKey); ok {
		return x509.SHA256WithRSA
	}
	// We default to ECDSA but don't bother detecting invalid key types as we do
	// that in lots of other places and it will fail anyway if we try to sign with
	// an incompatible type.
	return x509.ECDSAWithSHA256
}

// SigAlgoForKeyType returns the preferred x509.SignatureAlgorithm for a given
// key type string from configuration or an existing cert. If the key type is
// not supported we return ECDSAWithSHA256 on the basis that it will fail anyway
// and we've already type checked config by the time we call this in general.
func SigAlgoForKeyType(keyType string) x509.SignatureAlgorithm {
	switch keyType {
	case "rsa":
		return x509.SHA256WithRSA
	case "ec":
		fallthrough
	default:
		return x509.ECDSAWithSHA256
	}
}

// CreateCSR returns a CSR to sign the given service with SAN entries
// along with the PEM-encoded private key for this certificate.
func CreateCSR(uri CertURI, commonName string, privateKey crypto.Signer,
	dnsNames []string, ipAddresses []net.IP, extensions ...pkix.Extension) (string, error) {
	template := &x509.CertificateRequest{
		URIs:               []*url.URL{uri.URI()},
		SignatureAlgorithm: SigAlgoForKey(privateKey),
		ExtraExtensions:    extensions,
		Subject:            pkix.Name{CommonName: commonName},
		DNSNames:           dnsNames,
		IPAddresses:        ipAddresses,
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
func CreateCACSR(uri CertURI, commonName string, privateKey crypto.Signer) (string, error) {
	ext, err := CreateCAExtension()
	if err != nil {
		return "", err
	}

	return CreateCSR(uri, commonName, privateKey, nil, nil, ext)
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
