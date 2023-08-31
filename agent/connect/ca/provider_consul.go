// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

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
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

var (
	// NotBefore will be CertificateTimeDriftBuffer in the past to account for
	// time drift between different servers.
	CertificateTimeDriftBuffer = time.Minute

	ErrNotInitialized = errors.New("provider not initialized")
)

type ConsulProvider struct {
	Delegate ConsulProviderStateDelegate

	config    *structs.ConsulCAProviderConfig
	id        string
	clusterID string
	isPrimary bool
	spiffeID  *connect.SpiffeIDSigning
	logger    hclog.Logger

	// testState is only used to test Consul leader's handling of providers that
	// need to persist state. Consul provider actually manages it's state directly
	// in the FSM since it is highly sensitive not (root private keys) not just
	// metadata for lookups. We could make a whole mock provider to keep this out
	// of Consul but that would still need to be configurable through real config
	// and is a lot more boilerplate to test this for equivalent functionality.
	testState map[string]string

	sync.RWMutex
}

var _ Provider = (*ConsulProvider)(nil)

// NewConsulProvider returns a new ConsulProvider that is ready to be used.
func NewConsulProvider(delegate ConsulProviderStateDelegate, logger hclog.Logger) *ConsulProvider {
	return &ConsulProvider{Delegate: delegate, logger: logger}
}

type ConsulProviderStateDelegate interface {
	ProviderState(id string) (*structs.CAConsulProviderState, error)
	ApplyCARequest(*structs.CARequest) (interface{}, error)
}

func hexStringHash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return connect.HexString(hash[:])
}

// Configure sets up the provider using the given configuration.
func (c *ConsulProvider) Configure(cfg ProviderConfig) error {
	// Parse the raw config and update our ID.
	config, err := ParseConsulCAConfig(cfg.RawConfig)
	if err != nil {
		return err
	}
	c.config = config
	c.id = hexStringHash(fmt.Sprintf("%s,%s,%s,%d,%v", config.PrivateKey, config.RootCert, config.PrivateKeyType, config.PrivateKeyBits, cfg.IsPrimary))
	c.clusterID = cfg.ClusterID
	c.isPrimary = cfg.IsPrimary
	c.spiffeID = connect.SpiffeIDSigningForCluster(c.clusterID)

	// Passthrough test state for state handling tests. See testState doc.
	c.parseTestState(cfg.RawConfig, cfg.State)

	// Exit early if the state store has an entry for this provider's config.
	providerState, err := c.Delegate.ProviderState(c.id)
	if err != nil {
		return err
	}

	if providerState != nil {
		return nil
	}

	oldIDs := []string{
		hexStringHash(fmt.Sprintf("%s,%s,%v", config.PrivateKey, config.RootCert, cfg.IsPrimary)),
		fmt.Sprintf("%s,%s", config.PrivateKey, config.RootCert),
	}

	// Check if there are any entries with old ID schemes.
	for _, oldID := range oldIDs {
		providerState, err = c.Delegate.ProviderState(oldID)
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
			if _, err := c.Delegate.ApplyCARequest(createReq); err != nil {
				return err
			}

			deleteReq := &structs.CARequest{
				Op:            structs.CAOpDeleteProviderState,
				ProviderState: providerState,
			}
			if _, err := c.Delegate.ApplyCARequest(deleteReq); err != nil {
				return err
			}

			return nil
		}
	}

	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &structs.CAConsulProviderState{ID: c.id},
	}
	if _, err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	c.logger.Debug("consul CA provider configured", "id", c.id, "is_primary", c.isPrimary)

	return nil
}

// State implements Provider. Consul actually does store all it's state in raft
// but it manages it independently through a separate table already so this is a
// no-op. This method just passes through testState which allows tests to verify
// state handling behavior without needing to plumb a full test mock provider
// right through Consul server code.
func (c *ConsulProvider) State() (map[string]string, error) {
	return c.testState, nil
}

// GenerateCAChain initializes a new root certificate and private key if needed.
func (c *ConsulProvider) GenerateCAChain() (string, error) {
	providerState, err := c.getState()
	if err != nil {
		return "", err
	}

	if !c.isPrimary {
		return "", fmt.Errorf("provider is not the root certificate authority")
	}
	if providerState.RootCert != "" {
		return providerState.RootCert, nil
	}

	// Generate a private key if needed
	newState := *providerState
	if c.config.PrivateKey == "" {
		_, pk, err := connect.GeneratePrivateKeyWithConfig(c.config.PrivateKeyType, c.config.PrivateKeyBits)
		if err != nil {
			return "", err
		}
		newState.PrivateKey = pk
	} else {
		newState.PrivateKey = c.config.PrivateKey
	}

	// Generate the root CA if necessary
	if c.config.RootCert == "" {
		nextSerial, err := c.incrementAndGetNextSerialNumber()
		if err != nil {
			return "", fmt.Errorf("error computing next serial number: %v", err)
		}

		ca, err := c.generateCA(newState.PrivateKey, nextSerial, c.config.RootCertTTL)
		if err != nil {
			return "", fmt.Errorf("error generating CA: %v", err)
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
	if _, err := c.Delegate.ApplyCARequest(args); err != nil {
		return "", err
	}

	return newState.RootCert, nil
}

// GenerateIntermediateCSR creates a private key and generates a CSR
// for another datacenter's root to sign.
func (c *ConsulProvider) GenerateIntermediateCSR() (string, string, error) {
	providerState, err := c.getState()
	if err != nil {
		return "", "", err
	}

	if c.isPrimary {
		return "", "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	// Create a new private key and CSR.
	signer, pk, err := connect.GeneratePrivateKeyWithConfig(c.config.PrivateKeyType, c.config.PrivateKeyBits)
	if err != nil {
		return "", "", err
	}

	csr, err := connect.CreateCACSR(c.spiffeID, signer)
	if err != nil {
		return "", "", err
	}

	// Write the new provider state to the store.
	newState := *providerState
	newState.PrivateKey = pk
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	if _, err := c.Delegate.ApplyCARequest(args); err != nil {
		return "", "", err
	}

	return csr, "", nil
}

// SetIntermediate validates that the given intermediate is for the right private key
// and writes the given intermediate and root certificates to the state.
func (c *ConsulProvider) SetIntermediate(intermediatePEM, rootPEM, _ string) error {
	providerState, err := c.getState()
	if err != nil {
		return err
	}

	if c.isPrimary {
		return fmt.Errorf("cannot set an intermediate using another root in the primary datacenter")
	}

	if err = validateSetIntermediate(intermediatePEM, rootPEM, c.spiffeID); err != nil {
		return err
	}
	if err := validateIntermediateSignedByPrivateKey(intermediatePEM, providerState.PrivateKey); err != nil {
		return err
	}

	// Update the state
	newState := *providerState
	newState.IntermediateCert = intermediatePEM
	newState.RootCert = rootPEM
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	if _, err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

func (c *ConsulProvider) ActiveLeafSigningCert() (string, error) {
	providerState, err := c.getState()
	if err != nil {
		return "", err
	}

	if c.isPrimary {
		return providerState.RootCert, nil
	}
	return providerState.IntermediateCert, nil
}

// Remove the state store entry for this provider instance.
func (c *ConsulProvider) Cleanup(_ bool, _ map[string]interface{}) error {
	// This method only gets called for final cleanup. Therefore we don't
	// need to worry about the case where a ca config update is made to
	// change the cert ttls but leaving the private key and root cert the
	// same. Changing those would change the id field on the provider.
	args := &structs.CARequest{
		Op:            structs.CAOpDeleteProviderState,
		ProviderState: &structs.CAConsulProviderState{ID: c.id},
	}
	if _, err := c.Delegate.ApplyCARequest(args); err != nil {
		return err
	}

	return nil
}

// Sign returns a new certificate valid for the given SpiffeIDService
// using the current CA.
func (c *ConsulProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	connect.HackSANExtensionForCSR(csr)

	// Lock during the signing so we don't use the same index twice
	// for different cert serial numbers.
	c.Lock()
	defer c.Unlock()

	// Get the provider state
	providerState, err := c.getState()
	if err != nil {
		return "", err
	}
	if providerState.PrivateKey == "" {
		return "", ErrNotInitialized
	}

	// Create the keyId for the cert from the signing private key.
	signer, err := connect.ParseSigner(providerState.PrivateKey)
	if err != nil {
		return "", err
	}
	if signer == nil {
		return "", ErrNotInitialized
	}
	keyId, err := connect.KeyId(signer.Public())
	if err != nil {
		return "", err
	}

	// Create the subjectKeyId for the cert from the csr public key.
	subjectKeyID, err := connect.KeyId(csr.PublicKey)
	if err != nil {
		return "", err
	}

	// Parse the CA cert
	certPEM, err := c.ActiveLeafSigningCert()
	if err != nil {
		return "", err
	}
	caCert, err := connect.ParseCert(certPEM)
	if err != nil {
		return "", fmt.Errorf("error parsing CA cert: %s", err)
	}

	nextSerial, err := c.incrementAndGetNextSerialNumber()
	if err != nil {
		return "", fmt.Errorf("error computing next serial number: %v", err)
	}

	// Cert template for generation
	sn := &big.Int{}
	sn.SetUint64(nextSerial)
	// Sign the certificate valid from 1 minute in the past, this helps it be
	// accepted right away even when nodes are not in close time sync across the
	// cluster. A minute is more than enough for typical DC clock drift.
	effectiveNow := time.Now().Add(-1 * time.Minute)
	template := x509.Certificate{
		SerialNumber: sn,
		URIs:         csr.URIs,
		Signature:    csr.Signature,
		// We use the correct signature algorithm for the CA key we are signing with
		// regardless of the algorithm used to sign the CSR signature above since
		// the leaf might use a different key type.
		SignatureAlgorithm:    connect.SigAlgoForKey(signer),
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
		SubjectKeyId:   subjectKeyID,
		DNSNames:       csr.DNSNames,
		IPAddresses:    csr.IPAddresses,
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

	// Set the response
	return buf.String(), nil
}

// SignIntermediate will validate the CSR to ensure the trust domain in the
// URI SAN matches the local one and that basic constraints for a CA certificate
// are met. It should return a signed CA certificate with a path length constraint
// of 0 to ensure that the certificate cannot be used to generate further CA certs.
func (c *ConsulProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	providerState, err := c.getState()
	if err != nil {
		return "", err
	}

	err = validateSignIntermediate(csr, c.spiffeID)
	if err != nil {
		return "", err
	}

	// Get the signing private key.
	signer, err := connect.ParseSigner(providerState.PrivateKey)
	if err != nil {
		return "", err
	}
	subjectKeyID, err := connect.KeyId(csr.PublicKey)
	if err != nil {
		return "", err
	}

	// Parse the CA cert
	caCert, err := connect.ParseCert(providerState.RootCert)
	if err != nil {
		return "", fmt.Errorf("error parsing CA cert: %s", err)
	}

	nextSerial, err := c.incrementAndGetNextSerialNumber()
	if err != nil {
		return "", fmt.Errorf("error computing next serial number: %v", err)
	}

	// Cert template for generation
	sn := &big.Int{}
	sn.SetUint64(nextSerial)
	// Sign the certificate valid from 1 minute in the past, this helps it be
	// accepted right away even when nodes are not in close time sync across the
	// cluster. A minute is more than enough for typical DC clock drift.
	effectiveNow := time.Now().Add(-1 * CertificateTimeDriftBuffer)
	template := x509.Certificate{
		SerialNumber:          sn,
		DNSNames:              csr.DNSNames,
		EmailAddresses:        csr.EmailAddresses,
		IPAddresses:           csr.IPAddresses,
		URIs:                  csr.URIs,
		ExtraExtensions:       csr.ExtraExtensions,
		Subject:               csr.Subject,
		Signature:             csr.Signature,
		SignatureAlgorithm:    connect.SigAlgoForKey(signer),
		PublicKeyAlgorithm:    csr.PublicKeyAlgorithm,
		PublicKey:             csr.PublicKey,
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
		IsCA:           true,
		MaxPathLenZero: true,
		NotAfter:       effectiveNow.Add(c.config.IntermediateCertTTL),
		NotBefore:      effectiveNow,
		SubjectKeyId:   subjectKeyID,
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

	// Set the response
	return buf.String(), nil
}

// CrossSignCA returns the given CA cert signed by the current active root.
func (c *ConsulProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	c.Lock()
	defer c.Unlock()

	if c.config.DisableCrossSigning {
		return "", errors.New("cross-signing disabled")
	}

	// Get the provider state
	providerState, err := c.getState()
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

	nextSerial, err := c.incrementAndGetNextSerialNumber()
	if err != nil {
		return "", fmt.Errorf("error computing next serial number: %v", err)
	}

	// Create the cross-signing template from the existing root CA
	serialNum := &big.Int{}
	serialNum.SetUint64(nextSerial)
	template := *cert
	template.SerialNumber = serialNum
	template.SignatureAlgorithm = rootCA.SignatureAlgorithm
	template.AuthorityKeyId = keyId

	// Sign the certificate valid from 1 minute in the past, this helps it be
	// accepted right away even when nodes are not in close time sync across the
	// cluster. A minute is more than enough for typical DC clock drift.
	effectiveNow := time.Now().Add(-1 * time.Minute)
	template.NotBefore = effectiveNow
	// This cross-signed cert is only needed during rotation, and only while old
	// leaf certs are still in use. They expire within 3 days currently so 7 is
	// safe. TODO(banks): make this be based on leaf expiry time when that is
	// configurable.
	template.NotAfter = effectiveNow.AddDate(0, 0, 7)

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

	return buf.String(), nil
}

// SupportsCrossSigning implements Provider
func (c *ConsulProvider) SupportsCrossSigning() (bool, error) {
	return !c.config.DisableCrossSigning, nil
}

// getState returns the current provider state from the state delegate, and returns
// ErrNotInitialized if no entry is found.
func (c *ConsulProvider) getState() (*structs.CAConsulProviderState, error) {
	providerState, err := c.Delegate.ProviderState(c.id)
	if err != nil {
		return nil, err
	}

	if providerState == nil {
		return nil, ErrNotInitialized
	}

	return providerState, nil
}

func (c *ConsulProvider) incrementAndGetNextSerialNumber() (uint64, error) {
	args := &structs.CARequest{
		Op: structs.CAOpIncrementProviderSerialNumber,
	}

	raw, err := c.Delegate.ApplyCARequest(args)
	if err != nil {
		return 0, err
	}

	return raw.(uint64), nil
}

// generateCA makes a new root CA using the current private key
func (c *ConsulProvider) generateCA(privateKey string, sn uint64, rootCertTTL time.Duration) (string, error) {
	privKey, err := connect.ParseSigner(privateKey)
	if err != nil {
		return "", fmt.Errorf("error parsing private key %q: %s", privateKey, err)
	}

	// The URI (SPIFFE compatible) for the cert
	id := connect.SpiffeIDSigningForCluster(c.clusterID)
	keyId, err := connect.KeyId(privKey.Public())
	if err != nil {
		return "", err
	}

	// Create the CA cert
	uid, err := connect.CompactUID()
	if err != nil {
		return "", err
	}
	cn := connect.CACN("consul", uid, c.clusterID, c.isPrimary)
	serialNum := &big.Int{}
	serialNum.SetUint64(sn)
	template := x509.Certificate{
		SerialNumber:          serialNum,
		Subject:               pkix.Name{CommonName: cn},
		URIs:                  []*url.URL{id.URI()},
		BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign |
			x509.KeyUsageCRLSign |
			x509.KeyUsageDigitalSignature,
		IsCA:           true,
		NotAfter:       time.Now().Add(rootCertTTL),
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

func (c *ConsulProvider) parseTestState(rawConfig map[string]interface{}, state map[string]string) {
	c.testState = nil
	if rawTestState, ok := rawConfig["test_state"]; ok {
		if ts, ok := rawTestState.(map[string]string); ok {
			c.testState = ts
			return
		}

		// Secondary's config takes a trip through the state store before Configure
		// is called and RPC calls that msgpack encode also have the same effect. It
		// means we end up with map[string]string encoded as map[string]interface{}.
		// We just handle that case. There is no struct error handling because this
		// is test-only code (undocumented config key) and we'd rather not leave a
		// way to error CA setup and leave cluster unavailable in prod by
		// accidentally setting a bad test_state config.
		if ts, ok := rawTestState.(map[string]interface{}); ok {
			c.testState = make(map[string]string)
			for k, v := range ts {
				if s, ok := v.(string); ok {
					c.testState[k] = s
				}
			}
		}
	}
	// If config didn't explicitly specify test_state to return, but there is some
	// actual state from a previous provider. Just use that since that is expected
	// behavior that providers with state would preserve the state they are passed
	// in the common case.
	if len(state) > 0 && c.testState == nil {
		c.testState = state
	}
}
