package consul

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
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/mitchellh/mapstructure"
)

type ConsulCAProvider struct {
	config *structs.ConsulCAProviderConfig

	id  string
	srv *Server
	sync.RWMutex
}

// NewConsulCAProvider returns a new instance of the Consul CA provider,
// bootstrapping its state in the state store necessary
func NewConsulCAProvider(rawConfig map[string]interface{}, srv *Server) (*ConsulCAProvider, error) {
	conf, err := decodeConfig(rawConfig)
	if err != nil {
		return nil, err
	}
	provider := &ConsulCAProvider{
		config: conf,
		srv:    srv,
		id:     fmt.Sprintf("%s,%s", conf.PrivateKey, conf.RootCert),
	}

	// Check if this configuration of the provider has already been
	// initialized in the state store.
	state := srv.fsm.State()
	_, providerState, err := state.CAProviderState(provider.id)
	if err != nil {
		return nil, err
	}

	// Exit early if the state store has already been populated for this config.
	if providerState != nil {
		return provider, nil
	}

	newState := structs.CAConsulProviderState{
		ID: provider.id,
	}

	// Write the initial provider state to get the index to use for the
	// CA serial number.
	{
		args := &structs.CARequest{
			Op:            structs.CAOpSetProviderState,
			ProviderState: &newState,
		}
		resp, err := srv.raftApply(structs.ConnectCARequestType, args)
		if err != nil {
			return nil, err
		}
		if respErr, ok := resp.(error); ok {
			return nil, respErr
		}
	}

	idx, _, err := state.CAProviderState(provider.id)
	if err != nil {
		return nil, err
	}

	// Generate a private key if needed
	if conf.PrivateKey == "" {
		pk, err := generatePrivateKey()
		if err != nil {
			return nil, err
		}
		newState.PrivateKey = pk
	} else {
		newState.PrivateKey = conf.PrivateKey
	}

	// Generate the root CA if necessary
	if conf.RootCert == "" {
		ca, err := provider.generateCA(newState.PrivateKey, idx+1)
		if err != nil {
			return nil, fmt.Errorf("error generating CA: %v", err)
		}
		newState.RootCert = ca
	} else {
		newState.RootCert = conf.RootCert
	}

	// Write the provider state
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	resp, err := srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return nil, err
	}
	if respErr, ok := resp.(error); ok {
		return nil, respErr
	}

	return provider, nil
}

func decodeConfig(raw map[string]interface{}) (*structs.ConsulCAProviderConfig, error) {
	var config *structs.ConsulCAProviderConfig
	if err := mapstructure.WeakDecode(raw, &config); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	if config.PrivateKey == "" && config.RootCert != "" {
		return nil, fmt.Errorf("must provide a private key when providing a root cert")
	}

	return config, nil
}

// Return the active root CA and generate a new one if needed
func (c *ConsulCAProvider) ActiveRoot() (string, error) {
	state := c.srv.fsm.State()
	_, providerState, err := state.CAProviderState(c.id)
	if err != nil {
		return "", err
	}

	return providerState.RootCert, nil
}

// We aren't maintaining separate root/intermediate CAs for the builtin
// provider, so just return the root.
func (c *ConsulCAProvider) ActiveIntermediate() (string, error) {
	return c.ActiveRoot()
}

// We aren't maintaining separate root/intermediate CAs for the builtin
// provider, so just generate a CSR for the active root.
func (c *ConsulCAProvider) GenerateIntermediate() (string, error) {
	ca, err := c.ActiveIntermediate()
	if err != nil {
		return "", err
	}

	// todo(kyhavlov): make a new intermediate here

	return ca, err
}

// Remove the state store entry for this provider instance.
func (c *ConsulCAProvider) Cleanup() error {
	args := &structs.CARequest{
		Op:            structs.CAOpDeleteProviderState,
		ProviderState: &structs.CAConsulProviderState{ID: c.id},
	}
	resp, err := c.srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

// Sign returns a new certificate valid for the given SpiffeIDService
// using the current CA.
func (c *ConsulCAProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	// Lock during the signing so we don't use the same index twice
	// for different cert serial numbers.
	c.Lock()
	defer c.Unlock()

	// Get the provider state
	state := c.srv.fsm.State()
	_, providerState, err := state.CAProviderState(c.id)
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
	sn.SetUint64(providerState.LeafIndex + 1)
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
		return "", fmt.Errorf("error generating certificate: %s", err)
	}
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	// Increment the leaf cert index
	newState := *providerState
	newState.LeafIndex += 1
	args := &structs.CARequest{
		Op:            structs.CAOpSetProviderState,
		ProviderState: &newState,
	}
	resp, err := c.srv.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return "", err
	}
	if respErr, ok := resp.(error); ok {
		return "", respErr
	}

	// Set the response
	return buf.String(), nil
}

// CrossSignCA returns the given intermediate CA cert signed by the current active root.
func (c *ConsulCAProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	c.Lock()
	defer c.Unlock()

	// Get the provider state
	state := c.srv.fsm.State()
	_, providerState, err := state.CAProviderState(c.id)
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
	serialNum.SetUint64(providerState.LeafIndex + 1)
	template := *cert
	template.SerialNumber = serialNum
	template.Subject = rootCA.Subject
	template.SignatureAlgorithm = rootCA.SignatureAlgorithm
	template.SubjectKeyId = keyId
	template.AuthorityKeyId = keyId

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

// generateCA makes a new root CA using the current private key
func (c *ConsulCAProvider) generateCA(privateKey string, sn uint64) (string, error) {
	state := c.srv.fsm.State()
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
	id := &connect.SpiffeIDSigning{ClusterID: config.ClusterID, Domain: "consul"}
	keyId, err := connect.KeyId(privKey.Public())
	if err != nil {
		return "", err
	}

	// Create the CA cert
	serialNum := &big.Int{}
	serialNum.SetUint64(sn)
	template := x509.Certificate{
		SerialNumber: serialNum,
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
		return "", fmt.Errorf("error generating CA certificate: %s", err)
	}

	var buf bytes.Buffer
	err = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: bs})
	if err != nil {
		return "", fmt.Errorf("error encoding private key: %s", err)
	}

	return buf.String(), nil
}
