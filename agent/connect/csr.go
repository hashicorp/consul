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
	"fmt"
	"net"
	"net/url"
	"strings"
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
func CreateCSR(uri CertURI, privateKey crypto.Signer,
	dnsNames []string, ipAddresses []net.IP, extensions ...pkix.Extension) (string, error) {

	// Drop everything after the ':' from the name when constructing the DNS SANs.
	uniqueNames := make(map[string]struct{})
	formattedDNSNames := make([]string, 0)
	for _, host := range dnsNames {
		hostSegments := strings.Split(host, ":")
		if len(hostSegments) == 0 || hostSegments[0] == "" {
			continue
		}

		formattedHost := hostSegments[0]
		if _, ok := uniqueNames[formattedHost]; !ok {
			formattedDNSNames = append(formattedDNSNames, formattedHost)
			uniqueNames[formattedHost] = struct{}{}
		}
	}

	template := &x509.CertificateRequest{
		URIs:               []*url.URL{uri.URI()},
		SignatureAlgorithm: SigAlgoForKey(privateKey),
		ExtraExtensions:    extensions,
		DNSNames:           formattedDNSNames,
		IPAddresses:        ipAddresses,
	}
	HackSANExtensionForCSR(template)

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

	return CreateCSR(uri, privateKey, nil, nil, ext)
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

// InvalidCSRError returns an error with the given fmt.Sprintf-formatted message
// indicating certificate signing failed because the user supplied an invalid CSR.
//
// See: IsInvalidCSRError.
func InvalidCSRError(format string, args ...interface{}) error {
	return invalidCSRError{fmt.Sprintf(format, args...)}
}

// IsInvalidCSRError returns whether the given error indicates that certificate
// signing failed because the user supplied an invalid CSR.
func IsInvalidCSRError(err error) bool {
	_, ok := err.(invalidCSRError)
	return ok
}

type invalidCSRError struct {
	s string
}

func (e invalidCSRError) Error() string { return e.s }
