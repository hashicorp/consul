package ca

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
)

var ErrNotInitialized = errors.New("provider not initialized")

type ConsulProvider struct {
	Delegate ConsulProviderStateDelegate

	config *structs.ConsulCAProviderConfig
	id     string
	isRoot bool
	sync.RWMutex
}

type ConsulProviderStateDelegate interface {
	State() *state.Store
	ApplyCARequest(*structs.CARequest) error
}

// Configure sets up the provider using the given configuration.
func (c *ConsulProvider) Configure(clusterID string, isRoot bool, rawConfig map[string]interface{}) error {
	// Parse the raw config and update our ID.
	config, err := ParseConsulCAConfig(rawConfig)
	if err != nil {
		return err
	}
	c.config = config
	c.isRoot = isRoot
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s,%s,%v", config.PrivateKey, config.RootCert, isRoot)))
	c.id = strings.Replace(fmt.Sprintf("% x", hash), " ", ":", -1)

	// Exit early if the state store has an entry for this provider's config.
	_, providerState, err := c.Delegate.State().CAProviderState(c.id)
	if err != nil {
		return err
	}

	if providerState != nil {
		return nil
	}

	// Check if there's an entry with the old ID scheme.
	oldID := fmt.Sprintf("%s,%s", config.PrivateKey, config.RootCert)
	_, providerState, err = c.Delegate.State().CAProviderState(oldID)
	if err != nil {
		return err
	}

	// Found an entry with the old ID, so update it to the new ID and
	// delete the old entry.
	if providerState != nil {
		newState := *providerState
		newState.ID = c.id
		createReq := &structs.CARequest{
			Op:            structs.CAOpSetProviderState,
			ProviderState: &newState,
		}
		if err := c.Delegate.ApplyCARequest(createReq); err != nil {
			return err
		}

		deleteReq := &structs.CARequest{
			Op:            structs.CAOpDeleteProviderState,
			ProviderState: providerState,
		}
		if err := c.Delegate.ApplyCARequest(deleteReq); err != nil {
			return err
		}

		return nil
	}

	// Write the provider state to the state store.
	newState := structs.CAConsulProviderState{
		ID: c.id,
	}

	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	if err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

// ActiveRoot returns the active root CA certificate.
func (c *ConsulProvider) ActiveRoot() (string, error) {
	state := c.Delegate.State()
	_, providerState, err := state.CAProviderState(c.id)
	if err != nil {
		return "", err
	}

	return providerState.RootCert, nil
}

// GenerateRoot initializes a new root certificate and private key
// if needed.
func (c *ConsulProvider) GenerateRoot() error {
	state := c.Delegate.State()
	idx, providerState, err := state.CAProviderState(c.id)
	if err != nil {
		return err
	}

	if providerState == nil {
		return ErrNotInitialized
	}
	if !c.isRoot {
		return fmt.Errorf("provider is not the root certificate authority")
	}
	if providerState.RootCert != "" {
		return nil
	}

	// Generate a private key if needed
	newState := *providerState
	if c.config.PrivateKey == "" {
		_, pk, err := connect.GeneratePrivateKey()
		if err != nil {
			return err
		}
		newState.PrivateKey = pk
	} else {
		newState.PrivateKey = c.config.PrivateKey
	}

	// Generate the root CA if necessary
	if c.config.RootCert == "" {
		ca, err := c.generateCA(newState.PrivateKey, idx+1)
		if err != nil {
			return fmt.Errorf("error generating CA: %v", err)
		}
		newState.RootCert = ca
	} else {
		newState.RootCert = c.config.RootCert
	}

	// Write the provider state
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	if err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

// We aren't maintaining separate root/intermediate CAs for the builtin
// provider, so just return the root.
func (c *ConsulProvider) ActiveIntermediate() (string, error) {
	return c.ActiveRoot()
}

// We aren't maintaining separate root/intermediate CAs for the builtin
// provider, so just return the root.
func (c *ConsulProvider) GenerateIntermediate() (string, error) {
	return c.ActiveIntermediate()
}

// Remove the state store entry for this provider instance.
func (c *ConsulProvider) Cleanup() error {
	args := &structs.CARequest{
		Op:            structs.CAOpDeleteProviderState,
		ProviderState: &structs.CAConsulProviderState{ID: c.id},
	}
	if err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

// Sign returns a new certificate valid for the given SpiffeIDService
// using the current CA.
func (c *ConsulProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	// Lock during the signing so we don't use the same index twice
	// for different cert serial numbers.
	c.Lock()
	defer c.Unlock()

	// Get the provider state
	state := c.Delegate.State()
	idx, providerState, err := state.CAProviderState(c.id)
	if err != nil {
		return "", err
	}

	// Create the keyId for the cert from the signing private key.
	signer, err := connect.ParseSigner(providerState.PrivateKey)
	if err != nil {
		return "", err
	}
	if signer == nil {
		return "", fmt.Errorf("error signing cert: Consul CA not initialized yet")
	}
	keyId, err := connect.KeyId(signer.Public())
	if err != nil {
		return "", err
	}

	// Parse the SPIFFE ID
	spiffeId, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return "", err
	}
	serviceId, ok := spiffeId.(*connect.SpiffeIDService)
	if !ok {
		return "", fmt.Errorf("SPIFFE ID in CSR must be a service ID")
	}

	// Parse the CA cert
	caCert, err := connect.ParseCert(providerState.RootCert)
	if err != nil {
		return "", fmt.Errorf("error parsing CA cert: %s", err)
	}

	// Cert template for generation
	sn := &big.Int{}
	sn.SetUint64(idx + 1)
	// Sign the certificate valid from 1 minute in the past, this helps it be
	// accepted right away even when nodes are not in close time sync accross the
	// cluster. A minute is more than enough for typical DC clock drift.
	effectiveNow := time.Now().Add(-1 * time.Minute)
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
		NotAfter:       effectiveNow.Add(c.config.LeafCertTTL),
		NotBefore:      effectiveNow,
		AuthorityKeyId: keyId,
		SubjectKeyId:   keyId,
	}

	// Create the certificate, PEM encode it and return that value.
	var buf bytes.Buffer
	bs, err := x509.CreateCertificate(
		rand.Reader, &template, caCert, csr.PublicKey, signer)
	if err != nil {
		return "", fmt.Errorf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding certificate: %s", err)
	}

	err = c.incrementProviderIndex(providerState)
	if err != nil {
		return "", err
	}

	// Set the response
	return buf.String(), nil
}

// CrossSignCA returns the given CA cert signed by the current active root.
func (c *ConsulProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	c.Lock()
	defer c.Unlock()

	// Get the provider state
	state := c.Delegate.State()
	idx, providerState, err := state.CAProviderState(c.id)
	if err != nil {
		return "", err
	}

	privKey, err := connect.ParseSigner(providerState.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("error parsing private key %q: %s", providerState.PrivateKey, err)
	}

	rootCA, err := connect.ParseCert(providerState.RootCert)
	if err != nil {
		return "", err
	}

	keyId, err := connect.KeyId(privKey.Public())
	if err != nil {
		return "", err
	}

	// Create the cross-signing template from the existing root CA
	serialNum := &big.Int{}
	serialNum.SetUint64(idx + 1)
	template := *cert
	template.SerialNumber = serialNum
	template.SignatureAlgorithm = rootCA.SignatureAlgorithm
	template.AuthorityKeyId = keyId

	// Sign the certificate valid from 1 minute in the past, this helps it be
	// accepted right away even when nodes are not in close time sync accross the
	// cluster. A minute is more than enough for typical DC clock drift.
	effectiveNow := time.Now().Add(-1 * time.Minute)
	template.NotBefore = effectiveNow
	// This cross-signed cert is only needed during rotation, and only while old
	// leaf certs are still in use. They expire within 3 days currently so 7 is
	// safe. TODO(banks): make this be based on leaf expiry time when that is
	// configurable.
	template.NotAfter = effectiveNow.Add(7 * 24 * time.Hour)

	bs, err := x509.CreateCertificate(
		rand.Reader, &template, rootCA, cert.PublicKey, privKey)
	if err != nil {
		return "", fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	err = c.incrementProviderIndex(providerState)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

// incrementProviderIndex does a write to increment the provider state store table index
// used for serial numbers when generating certificates.
func (c *ConsulProvider) incrementProviderIndex(providerState *structs.CAConsulProviderState) error {
	newState := *providerState
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	if err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

// generateCA makes a new root CA using the current private key
func (c *ConsulProvider) generateCA(privateKey string, sn uint64) (string, error) {
	state := c.Delegate.State()
	_, config, err := state.CAConfig()
	if err != nil {
		return "", err
	}

	privKey, err := connect.ParseSigner(privateKey)
	if err != nil {
		return "", fmt.Errorf("error parsing private key %q: %s", privateKey, err)
	}

	name := fmt.Sprintf("Consul CA %d", sn)

	// The URI (SPIFFE compatible) for the cert
	id := connect.SpiffeIDSigningForCluster(config)
	keyId, err := connect.KeyId(privKey.Public())
	if err != nil {
		return "", err
	}

	// Create the CA cert
	serialNum := &big.Int{}
	serialNum.SetUint64(sn)
	template := x509.Certificate{
		SerialNumber:          serialNum,
		Subject:               pkix.Name{CommonName: name},
		URIs:                  []*url.URL{id.URI()},
		BasicConstraintsValid: true,
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
		return "", fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), nil
}
