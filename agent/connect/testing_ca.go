package connect

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-uuid"
	"github.com/mitchellh/go-testing-interface"
)

// TestClusterID is the Consul cluster ID for testing.
const TestClusterID = "11111111-2222-3333-4444-555555555555"

// testCACounter is just an atomically incremented counter for creating
// unique names for the CA certs.
var testCACounter uint64

// TestCA creates a test CA certificate and signing key and returns it
// in the CARoot structure format. The returned CA will be set as Active = true.
//
// If xc is non-nil, then the returned certificate will have a signing cert
// that is cross-signed with the previous cert, and this will be set as
// SigningCert.
func TestCA(t testing.T, xc *structs.CARoot) *structs.CARoot {
	var result structs.CARoot
	result.Active = true
	result.Name = fmt.Sprintf("Test CA %d", atomic.AddUint64(&testCACounter, 1))

	// Create the private key we'll use for this CA cert.
	signer, keyPEM := testPrivateKey(t)
	result.SigningKey = keyPEM

	// The serial number for the cert
	sn, err := testSerialNumber()
	if err != nil {
		t.Fatalf("error generating serial number: %s", err)
	}

	// The URI (SPIFFE compatible) for the cert
	id := &SpiffeIDSigning{ClusterID: TestClusterID, Domain: "consul"}

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{CommonName: result.Name},
		URIs:         []*url.URL{id.URI()},
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
		IsCA:           true,
		NotAfter:       time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: testKeyID(t, signer.Public()),
		SubjectKeyId:   testKeyID(t, signer.Public()),
	}

	bs, err := x509.CreateCertificate(
		rand.Reader, &template, &template, signer.Public(), signer)
	if err != nil {
		t.Fatalf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding private key: %s", err)
	}
	result.RootCert = buf.String()
	result.ID, err = CalculateCertFingerprint(result.RootCert)
	if err != nil {
		t.Fatalf("error generating CA ID fingerprint: %s", err)
	}

	// If there is a prior CA to cross-sign with, then we need to create that
	// and set it as the signing cert.
	if xc != nil {
		xccert, err := ParseCert(xc.RootCert)
		if err != nil {
			t.Fatalf("error parsing CA cert: %s", err)
		}
		xcsigner, err := ParseSigner(xc.SigningKey)
		if err != nil {
			t.Fatalf("error parsing signing key: %s", err)
		}

		// Set the authority key to be the previous one.
		// NOTE(mitchellh): From Paul Banks:  if we have to cross-sign a cert
		// that came from outside (e.g. vault) we can't rely on them using the
		// same KeyID hashing algo we do so we'd need to actually copy this
		// from the xc cert's subjectKeyIdentifier extension.
		template.AuthorityKeyId = testKeyID(t, xcsigner.Public())

		// Create the new certificate where the parent is the previous
		// CA, the public key is the new public key, and the signing private
		// key is the old private key.
		bs, err := x509.CreateCertificate(
			rand.Reader, &template, xccert, signer.Public(), xcsigner)
		if err != nil {
			t.Fatalf("error generating CA certificate: %s", err)
		}

		var buf bytes.Buffer
		err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
		if err != nil {
			t.Fatalf("error encoding private key: %s", err)
		}
		result.SigningCert = buf.String()
	}

	return &result
}

// TestLeaf returns a valid leaf certificate and it's private key for the named
// service with the given CA Root.
func TestLeaf(t testing.T, service string, root *structs.CARoot) (string, string) {
	// Parse the CA cert and signing key from the root
	cert := root.SigningCert
	if cert == "" {
		cert = root.RootCert
	}
	caCert, err := ParseCert(cert)
	if err != nil {
		t.Fatalf("error parsing CA cert: %s", err)
	}
	caSigner, err := ParseSigner(root.SigningKey)
	if err != nil {
		t.Fatalf("error parsing signing key: %s", err)
	}

	// Build the SPIFFE ID
	spiffeId := &SpiffeIDService{
		Host:       fmt.Sprintf("%s.consul", TestClusterID),
		Namespace:  "default",
		Datacenter: "dc1",
		Service:    service,
	}

	// The serial number for the cert
	sn, err := testSerialNumber()
	if err != nil {
		t.Fatalf("error generating serial number: %s", err)
	}

	// Generate fresh private key
	pkSigner, pkPEM, err := GeneratePrivateKey()
	if err != nil {
		t.Fatalf("failed to generate private key: %s", err)
	}

	// Cert template for generation
	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: service},
		URIs:                  []*url.URL{spiffeId.URI()},
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyAgreement |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		NotAfter:       time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: testKeyID(t, caSigner.Public()),
		SubjectKeyId:   testKeyID(t, pkSigner.Public()),
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, pkSigner.Public(), caSigner)
	if err != nil {
		t.Fatalf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding private key: %s", err)
	}

	return buf.String(), pkPEM
}

// TestCSR returns a CSR to sign the given service along with the PEM-encoded
// private key for this certificate.
func TestCSR(t testing.T, uri CertURI) (string, string) {
	template := &x509.CertificateRequest{
		URIs:               []*url.URL{uri.URI()},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	// Create the private key we'll use
	signer, pkPEM := testPrivateKey(t)

	// Create the CSR itself
	var csrBuf bytes.Buffer
	bs, err := x509.CreateCertificateRequest(rand.Reader, template, signer)
	if err != nil {
		t.Fatalf("error creating CSR: %s", err)
	}

	err = pem.Encode(&csrBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding CSR: %s", err)
	}

	return csrBuf.String(), pkPEM
}

// testKeyID returns a KeyID from the given public key. This just calls
// KeyId but handles errors for tests.
func testKeyID(t testing.T, raw interface{}) []byte {
	result, err := KeyId(raw)
	if err != nil {
		t.Fatalf("KeyId error: %s", err)
	}

	return result
}

// testPrivateKey creates an ECDSA based private key. Both a crypto.Signer and
// the key in PEM form are returned.
//
// NOTE(banks): this was memoized to save entropy during tests but it turns out
// crypto/rand will never block and always reads from /dev/urandom on unix OSes
// which does not consume entropy.
//
// If we find by profiling it's taking a lot of cycles we could optimise/cache
// again but we at least need to use different keys for each distinct CA (when
// multiple CAs are generated at once e.g. to test cross-signing) and a
// different one again for the leafs otherwise we risk tests that have false
// positives since signatures from different logical cert's keys are
// indistinguishable, but worse we build validation chains using AuthorityKeyID
// which will be the same for multiple CAs/Leafs. Also note that our UUID
// generator also reads from crypto rand and is called far more often during
// tests than this will be.
func testPrivateKey(t testing.T) (crypto.Signer, string) {
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("error generating private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		t.Fatalf("error generating private key: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding private key: %s", err)
	}

	return pk, buf.String()
}

// testSerialNumber generates a serial number suitable for a certificate.
// For testing, this just sets it to a random number.
//
// This function is taken directly from the Vault implementation.
func testSerialNumber() (*big.Int, error) {
	return rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
}

// testUUID generates a UUID for testing.
func testUUID(t testing.T) string {
	ret, err := uuid.GenerateUUID()
	if err != nil {
		t.Fatalf("Unable to generate a UUID, %s", err)
	}

	return ret
}

// TestAgentRPC is an interface that an RPC client must implement. This is a
// helper interface that is implemented by the agent delegate so that test
// helpers can make RPCs without introducing an import cycle on `agent`.
type TestAgentRPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// TestCAConfigSet sets a CARoot returned by TestCA into the TestAgent state. It
// requires that TestAgent had connect enabled in it's config. If ca is nil, a
// new CA is created.
//
// It returns the CARoot passed or created.
//
// Note that we have to use an interface for the TestAgent.RPC method since we
// can't introduce an import cycle by importing `agent.TestAgent` here directly.
// It also means this will work in a few other places we mock that method.
func TestCAConfigSet(t testing.T, a TestAgentRPC,
	ca *structs.CARoot) *structs.CARoot {
	t.Helper()

	if ca == nil {
		ca = TestCA(t, nil)
	}
	newConfig := &structs.CAConfiguration{
		Provider: "consul",
		Config: map[string]interface{}{
			"PrivateKey":     ca.SigningKey,
			"RootCert":       ca.RootCert,
			"RotationPeriod": 180 * 24 * time.Hour,
		},
	}
	args := &structs.CARequest{
		Datacenter: "dc1",
		Config:     newConfig,
	}
	var reply interface{}

	err := a.RPC("ConnectCA.ConfigurationSet", args, &reply)
	if err != nil {
		t.Fatalf("failed to set test CA config: %s", err)
	}
	return ca
}
