package connect

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	uuid "github.com/hashicorp/go-uuid"
	"github.com/mitchellh/mapstructure"
)

// CAProvider is the interface for Consul to interact with
// an external CA that provides leaf certificate signing for
// given SpiffeIDServices.
type CAProvider interface {
	SetConfiguration(raw map[string]interface{}) error
	ActiveRoot() (*structs.CARoot, error)
	ActiveIntermediate() (*structs.CARoot, error)
	RotateIntermediate() error
	Sign(*SpiffeIDService, *x509.CertificateRequest) (*structs.IssuedCert, error)
}

type ConsulCAProviderConfig struct {
	PrivateKey     string
	RootCert       string
	RotationPeriod time.Duration
}

type ConsulCAProvider struct {
	config *ConsulCAProviderConfig

	// todo(kyhavlov): store these directly in the state store
	// and pass a reference to the state to this provider instead of
	// having these values here
	privateKey string
	caRoot     *structs.CARoot
	caIndex    uint64
	sync.RWMutex
}

func NewConsulCAProvider(rawConfig map[string]interface{}) (*ConsulCAProvider, error) {
	provider := &ConsulCAProvider{}
	provider.SetConfiguration(rawConfig)

	return provider, nil
}

func (c *ConsulCAProvider) SetConfiguration(raw map[string]interface{}) error {
	conf, err := decodeConfig(raw)
	if err != nil {
		return err
	}

	c.config = conf
	return nil
}

func decodeConfig(raw map[string]interface{}) (*ConsulCAProviderConfig, error) {
	var config *ConsulCAProviderConfig
	if err := mapstructure.WeakDecode(raw, &config); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	return config, nil
}

func (c *ConsulCAProvider) ActiveRoot() (*structs.CARoot, error) {
	if c.privateKey == "" {
		pk, err := generatePrivateKey()
		if err != nil {
			return nil, err
		}
		c.privateKey = pk
	}

	if c.caRoot == nil {
		ca, err := c.generateCA()
		if err != nil {
			return nil, err
		}
		c.caRoot = ca
	}

	return c.caRoot, nil
}

func (c *ConsulCAProvider) ActiveIntermediate() (*structs.CARoot, error) {
	return c.ActiveRoot()
}

func (c *ConsulCAProvider) RotateIntermediate() error {
	ca, err := c.generateCA()
	if err != nil {
		return err
	}
	c.caRoot = ca

	return nil
}

// Sign returns a new certificate valid for the given SpiffeIDService
// using the current CA.
func (c *ConsulCAProvider) Sign(serviceId *SpiffeIDService, csr *x509.CertificateRequest) (*structs.IssuedCert, error) {
	// The serial number for the cert.
	// todo(kyhavlov): increment this based on raft index once the provider uses
	// the state store directly
	sn, err := rand.Int(rand.Reader, (&big.Int{}).Exp(big.NewInt(2), big.NewInt(159), nil))
	if err != nil {
		return nil, fmt.Errorf("error generating serial number: %s", err)
	}

	// Create the keyId for the cert from the signing public key.
	signer, err := ParseSigner(c.privateKey)
	if err != nil {
		return nil, err
	}
	if signer == nil {
		return nil, fmt.Errorf("error signing cert: Consul CA not initialized yet")
	}
	keyId, err := KeyId(signer.Public())
	if err != nil {
		return nil, err
	}

	// Parse the CA cert
	caCert, err := ParseCert(c.caRoot.RootCert)
	if err != nil {
		return nil, fmt.Errorf("error parsing CA cert: %s", err)
	}

	// Cert template for generation
	template := x509.Certificate{
		SerialNumber:          sn,
		Subject:               pkix.Name{CommonName: serviceId.Service},
		URIs:                  csr.URIs,
		Signature:             csr.Signature,
		SignatureAlgorithm:    csr.SignatureAlgorithm,
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
		PublicKey:             csr.PublicKey,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageDataEncipherment |
			x509.KeyUsageKeyAgreement |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
			x509.ExtKeyUsageServerAuth,
		},
		NotAfter:       time.Now().Add(3 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: keyId,
		SubjectKeyId:   keyId,
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, signer.Public(), signer)
	if err != nil {
		return nil, fmt.Errorf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return nil, fmt.Errorf("error encoding private key: %s", err)
	}

	// Set the response
	return &structs.IssuedCert{
		SerialNumber: HexString(template.SerialNumber.Bytes()),
		CertPEM:      buf.String(),
		Service:      serviceId.Service,
		ServiceURI:   template.URIs[0].String(),
		ValidAfter:   template.NotBefore,
		ValidBefore:  template.NotAfter,
	}, nil
}

// generatePrivateKey returns a new private key
func generatePrivateKey() (string, error) {
	var pk *ecdsa.PrivateKey

	// If we have no key, then create a new one.
	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("error generating private key: %s", err)
	}

	bs, err := x509.MarshalECPrivateKey(pk)
	if err != nil {
		return "", fmt.Errorf("error generating private key: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "EC PRIVATE KEY", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), nil
}

// generateCA makes a new root CA using the given private key
func (c *ConsulCAProvider) generateCA() (*structs.CARoot, error) {
	privKey, err := ParseSigner(c.privateKey)
	if err != nil {
		return nil, err
	}

	name := fmt.Sprintf("Consul CA %d", atomic.AddUint64(&c.caIndex, 1))

	// The serial number for the cert
	sn, err := testSerialNumber()
	if err != nil {
		return nil, err
	}

	// The URI (SPIFFE compatible) for the cert
	id := &SpiffeIDSigning{ClusterID: testClusterID, Domain: "consul"}
	keyId, err := KeyId(privKey.Public())
	if err != nil {
		return nil, err
	}

	// Create the CA cert
	template := x509.Certificate{
		SerialNumber: sn,
		Subject:      pkix.Name{CommonName: name},
		URIs:         []*url.URL{id.URI()},
		PermittedDNSDomainsCritical: true,
		PermittedDNSDomains:         []string{id.URI().Hostname()},
		BasicConstraintsValid:       true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
		IsCA:           true,
		NotAfter:       time.Now().Add(10 * 365 * 24 * time.Hour),
		NotBefore:      time.Now(),
		AuthorityKeyId: keyId,
		SubjectKeyId:   keyId,
	}

	bs, err := x509.CreateCertificate(
		rand.Reader, &template, &template, privKey.Public(), privKey)
	if err != nil {
		return nil, fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return nil, fmt.Errorf("error encoding private key: %s", err)
	}

	// Generate an ID for the new intermediate
	rootId, err := uuid.GenerateUUID()
	if err != nil {
		return nil, err
	}

	return &structs.CARoot{
		ID:         rootId,
		Name:       name,
		RootCert:   buf.String(),
		SigningKey: c.privateKey,
		Active:     true,
	}, nil
}
