package connect

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
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

// testClusterID is the Consul cluster ID for testing.
//
// NOTE(mitchellh): This might have to change some other constant for
// real testing once we integrate the Cluster ID into the core. For now it
// is unchecked.
const testClusterID = "11111111-2222-3333-4444-555555555555"

// testCACounter is just an atomically incremented counter for creating
// unique names for the CA certs.
var testCACounter uint64 = 0

// TestCA creates a test CA certificate and signing key and returns it
// in the CARoot structure format. The returned CA will be set as Active = true.
//
// If xc is non-nil, then the returned certificate will have a signing cert
// that is cross-signed with the previous cert, and this will be set as
// SigningCert.
func TestCA(t testing.T, xc *structs.CARoot) *structs.CARoot {
	var result structs.CARoot
	result.ID = testUUID(t)
	result.Active = true
	result.Name = fmt.Sprintf("Test CA %d", atomic.AddUint64(&testCACounter, 1))

	// Create the private key we'll use for this CA cert.
	signer := testPrivateKey(t, &result)

	// The serial number for the cert
	sn, err := testSerialNumber()
	if err != nil {
		t.Fatalf("error generating serial number: %s", err)
	}

	// The URI (SPIFFE compatible) for the cert
	uri, err := url.Parse(fmt.Sprintf("spiffe://%s.consul", testClusterID))
	if err != nil {
		t.Fatalf("error parsing CA URI: %s", err)
	}

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{CommonName: result.Name},
		URIs:         []*url.URL{uri},
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{uri.Hostname()},
		BasicConstraintsValid:       true,
		KeyUsage:                    x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                        true,
		NotAfter:                    time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:                   time.Now(),
		AuthorityKeyId:              testKeyID(t, signer.Public()),
		SubjectKeyId:                testKeyID(t, signer.Public()),
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

		// Set the authority key to be the previous one
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

// TestLeaf returns a valid leaf certificate for the named service with
// the given CA Root.
func TestLeaf(t testing.T, service string, root *structs.CARoot) string {
	// Parse the CA cert and signing key from the root
	cert := root.SigningCert
	if cert == "" {
		cert = root.RootCert
	}
	caCert, err := ParseCert(cert)
	if err != nil {
		t.Fatalf("error parsing CA cert: %s", err)
	}
	signer, err := ParseSigner(root.SigningKey)
	if err != nil {
		t.Fatalf("error parsing signing key: %s", err)
	}

	// Build the SPIFFE ID
	spiffeId := &SpiffeIDService{
		Host:       fmt.Sprintf("%s.consul", testClusterID),
		Namespace:  "default",
		Datacenter: "dc01",
		Service:    service,
	}

	// The serial number for the cert
	sn, err := testSerialNumber()
	if err != nil {
		t.Fatalf("error generating serial number: %s", err)
	}

	// Cert template for generation
	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: service},
		URIs:                  []*url.URL{spiffeId.URI()},
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDataEncipherment | x509.KeyUsageKeyAgreement,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		NotAfter:       time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: testKeyID(t, signer.Public()),
		SubjectKeyId:   testKeyID(t, signer.Public()),
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, signer.Public(), signer)
	if err != nil {
		t.Fatalf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding private key: %s", err)
	}

	return buf.String()
}

// TestCSR returns a CSR to sign the given service.
func TestCSR(t testing.T, id SpiffeID) string {
	template := &x509.CertificateRequest{
		URIs:               []*url.URL{id.URI()},
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}

	// Create the private key we'll use
	signer := testPrivateKey(t, nil)

	// Create the CSR itself
	bs, err := x509.CreateCertificateRequest(rand.Reader, template, signer)
	if err != nil {
		t.Fatalf("error creating CSR: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: bs})
	if err != nil {
		t.Fatalf("error encoding CSR: %s", err)
	}

	return buf.String()
}

// testKeyID returns a KeyID from the given public key. The "raw" must be
// an *ecdsa.PublicKey, but is an interface type to suppot crypto.Signer.Public
// values.
func testKeyID(t testing.T, raw interface{}) []byte {
	pub, ok := raw.(*ecdsa.PublicKey)
	if !ok {
		t.Fatalf("raw is type %T, expected *ecdsa.PublicKey", raw)
	}

	// This is not standard; RFC allows any unique identifier as long as they
	// match in subject/authority chains but suggests specific hashing of DER
	// bytes of public key including DER tags. I can't be bothered to do esp.
	// since ECDSA keys don't have a handy way to marshal the publick key alone.
	h := sha256.New()
	h.Write(pub.X.Bytes())
	h.Write(pub.Y.Bytes())
	return h.Sum([]byte{})
}

// testMemoizePK is the private key that we memoize once we generate it
// once so that our tests don't rely on too much system entropy.
var testMemoizePK atomic.Value

// testPrivateKey creates an ECDSA based private key.
func testPrivateKey(t testing.T, ca *structs.CARoot) crypto.Signer {
	// If we already generated a private key, use that
	var pk *ecdsa.PrivateKey
	if v := testMemoizePK.Load(); v != nil {
		pk = v.(*ecdsa.PrivateKey)
	}

	// If we have no key, then create a new one.
	if pk == nil {
		var err error
		pk, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("error generating private key: %s", err)
		}
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
	if ca != nil {
		ca.SigningKey = buf.String()
	}

	// Memoize the key
	testMemoizePK.Store(pk)

	return pk
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
