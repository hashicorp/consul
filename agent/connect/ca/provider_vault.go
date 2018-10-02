package ca

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-uuid"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"
)

const VaultCALeafCertRole = "leaf-cert"

var ErrBackendNotMounted = fmt.Errorf("backend not mounted")
var ErrBackendNotInitialized = fmt.Errorf("backend not initialized")

type VaultProvider struct {
	config    *structs.VaultCAProviderConfig
	client    *vaultapi.Client
	isRoot    bool
	clusterId string
}

// Configure sets up the provider using the given configuration.
func (v *VaultProvider) Configure(clusterId string, isRoot bool, rawConfig map[string]interface{}) error {
	config, err := ParseVaultCAConfig(rawConfig)
	if err != nil {
		return err
	}

	clientConf := &vaultapi.Config{
		Address: config.Address,
	}
	client, err := vaultapi.NewClient(clientConf)
	if err != nil {
		return err
	}

	client.SetToken(config.Token)
	v.config = config
	v.client = client
	v.isRoot = isRoot
	v.clusterId = clusterId

	return nil
}

// ActiveRoot returns the active root CA certificate.
func (v *VaultProvider) ActiveRoot() (string, error) {
	return v.getCA(v.config.RootPKIPath)
}

// GenerateRoot mounts and initializes a new root PKI backend if needed.
func (v *VaultProvider) GenerateRoot() error {
	if !v.isRoot {
		return fmt.Errorf("provider is not the root certificate authority")
	}

	// Set up the root PKI backend if necessary.
	_, err := v.ActiveRoot()
	switch err {
	case ErrBackendNotMounted:
		err := v.client.Sys().Mount(v.config.RootPKIPath, &vaultapi.MountInput{
			Type:        "pki",
			Description: "root CA backend for Consul Connect",
			Config: vaultapi.MountConfigInput{
				MaxLeaseTTL: "8760h",
			},
		})

		if err != nil {
			return err
		}

		fallthrough
	case ErrBackendNotInitialized:
		spiffeID := connect.SpiffeIDSigning{ClusterID: v.clusterId, Domain: "consul"}
		uuid, err := uuid.GenerateUUID()
		if err != nil {
			return err
		}
		_, err = v.client.Logical().Write(v.config.RootPKIPath+"root/generate/internal", map[string]interface{}{
			"common_name": fmt.Sprintf("Vault CA Root Authority %s", uuid),
			"uri_sans":    spiffeID.URI().String(),
		})
		if err != nil {
			return err
		}
	default:
		if err != nil {
			return err
		}
	}

	return nil
}

// GenerateIntermediateCSR creates a private key and generates a CSR
// for another datacenter's root to sign, overwriting the intermediate backend
// in the process.
func (v *VaultProvider) GenerateIntermediateCSR() (string, error) {
	if v.isRoot {
		return "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	return v.generateIntermediateCSR()
}

func (v *VaultProvider) generateIntermediateCSR() (string, error) {
	mounts, err := v.client.Sys().ListMounts()
	if err != nil {
		return "", err
	}

	// Mount the backend if it isn't mounted already.
	if _, ok := mounts[v.config.IntermediatePKIPath]; !ok {
		err := v.client.Sys().Mount(v.config.IntermediatePKIPath, &vaultapi.MountInput{
			Type:        "pki",
			Description: "intermediate CA backend for Consul Connect",
			Config: vaultapi.MountConfigInput{
				MaxLeaseTTL: "2160h",
			},
		})

		if err != nil {
			return "", err
		}
	}

	// Create the role for issuing leaf certs if it doesn't exist yet
	rolePath := v.config.IntermediatePKIPath + "roles/" + VaultCALeafCertRole
	role, err := v.client.Logical().Read(rolePath)
	if err != nil {
		return "", err
	}
	spiffeID := connect.SpiffeIDSigning{ClusterID: v.clusterId, Domain: "consul"}
	if role == nil {
		_, err := v.client.Logical().Write(rolePath, map[string]interface{}{
			"allow_any_name":   true,
			"allowed_uri_sans": "spiffe://*",
			"key_type":         "any",
			"max_ttl":          v.config.LeafCertTTL.String(),
			"no_store":         true,
			"require_cn":       false,
		})
		if err != nil {
			return "", err
		}
	}

	// Generate a new intermediate CSR for the root to sign.
	data, err := v.client.Logical().Write(v.config.IntermediatePKIPath+"intermediate/generate/internal", map[string]interface{}{
		"common_name": "Vault CA Intermediate Authority",
		"key_bits":    224,
		"key_type":    "ec",
		"uri_sans":    spiffeID.URI().String(),
	})
	if err != nil {
		return "", err
	}
	if data == nil || data.Data["csr"] == "" {
		return "", fmt.Errorf("got empty value when generating intermediate CSR")
	}
	csr, ok := data.Data["csr"].(string)
	if !ok {
		return "", fmt.Errorf("csr result is not a string")
	}

	return csr, nil
}

// SetIntermediate writes the incoming intermediate and root certificates to the
// intermediate backend (as a chain).
func (v *VaultProvider) SetIntermediate(intermediatePEM, rootPEM string) error {
	if v.isRoot {
		return fmt.Errorf("cannot set an intermediate using another root in the primary datacenter")
	}

	_, err := v.client.Logical().Write(v.config.IntermediatePKIPath+"intermediate/set-signed", map[string]interface{}{
		"certificate": fmt.Sprintf("%s\n%s", intermediatePEM, rootPEM),
	})
	if err != nil {
		return err
	}

	return nil
}

// ActiveIntermediate returns the current intermediate certificate.
func (v *VaultProvider) ActiveIntermediate() (string, error) {
	return v.getCA(v.config.IntermediatePKIPath)
}

// getCA returns the raw CA cert for the given endpoint if there is one.
// We have to use the raw NewRequest call here instead of Logical().Read
// because the endpoint only returns the raw PEM contents of the CA cert
// and not the typical format of the secrets endpoints.
func (v *VaultProvider) getCA(path string) (string, error) {
	req := v.client.NewRequest("GET", "/v1/"+path+"/ca/pem")
	resp, err := v.client.RawRequest(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return "", ErrBackendNotMounted
	}
	if err != nil {
		return "", err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	root := string(bytes)
	if root == "" {
		return "", ErrBackendNotInitialized
	}

	return root, nil
}

// GenerateIntermediate mounts the configured intermediate PKI backend if
// necessary, then generates and signs a new CA CSR using the root PKI backend
// and updates the intermediate backend to use that new certificate.
func (v *VaultProvider) GenerateIntermediate() (string, error) {
	csr, err := v.generateIntermediateCSR()
	if err != nil {
		return "", err
	}

	// Sign the CSR with the root backend.
	intermediate, err := v.client.Logical().Write(v.config.RootPKIPath+"root/sign-intermediate", map[string]interface{}{
		"csr":    csr,
		"format": "pem_bundle",
	})
	if err != nil {
		return "", err
	}
	if intermediate == nil || intermediate.Data["certificate"] == "" {
		return "", fmt.Errorf("got empty value when generating intermediate certificate")
	}

	// Set the intermediate backend to use the new certificate.
	_, err = v.client.Logical().Write(v.config.IntermediatePKIPath+"intermediate/set-signed", map[string]interface{}{
		"certificate": intermediate.Data["certificate"],
	})
	if err != nil {
		return "", err
	}

	return v.ActiveIntermediate()
}

// Sign calls the configured role in the intermediate PKI backend to issue
// a new leaf certificate based on the provided CSR, with the issuing
// intermediate CA cert attached.
func (v *VaultProvider) Sign(csr *x509.CertificateRequest) (string, error) {
	var pemBuf bytes.Buffer
	if err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw}); err != nil {
		return "", err
	}

	// Use the leaf cert role to sign a new cert for this CSR.
	response, err := v.client.Logical().Write(v.config.IntermediatePKIPath+"sign/"+VaultCALeafCertRole, map[string]interface{}{
		"csr": pemBuf.String(),
		"ttl": v.config.LeafCertTTL.String(),
	})
	if err != nil {
		return "", fmt.Errorf("error issuing cert: %v", err)
	}
	if response == nil || response.Data["certificate"] == "" || response.Data["issuing_ca"] == "" {
		return "", fmt.Errorf("certificate info returned from Vault was blank")
	}

	cert, ok := response.Data["certificate"].(string)
	if !ok {
		return "", fmt.Errorf("certificate was not a string")
	}
	ca, ok := response.Data["issuing_ca"].(string)
	if !ok {
		return "", fmt.Errorf("issuing_ca was not a string")
	}

	return fmt.Sprintf("%s\n%s", cert, ca), nil
}

// SignIntermediate returns a signed CA certificate with a path length constraint
// of 0 to ensure that the certificate cannot be used to generate further CA certs.
func (v *VaultProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	var pemBuf bytes.Buffer
	err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw})
	if err != nil {
		return "", err
	}

	// Sign the CSR with the root backend.
	data, err := v.client.Logical().Write(v.config.RootPKIPath+"root/sign-intermediate", map[string]interface{}{
		"csr":             pemBuf.String(),
		"format":          "pem_bundle",
		"max_path_length": 0,
	})
	if err != nil {
		return "", err
	}
	if data == nil || data.Data["certificate"] == "" {
		return "", fmt.Errorf("got empty value when generating intermediate certificate")
	}

	intermediate, ok := data.Data["certificate"].(string)
	if !ok {
		return "", fmt.Errorf("signed intermediate result is not a string")
	}

	return intermediate, nil
}

// CrossSignCA takes a CA certificate and cross-signs it to form a trust chain
// back to our active root.
func (v *VaultProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	var pemBuf bytes.Buffer
	err := pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if err != nil {
		return "", err
	}

	// Have the root PKI backend sign this cert.
	response, err := v.client.Logical().Write(v.config.RootPKIPath+"root/sign-self-issued", map[string]interface{}{
		"certificate": pemBuf.String(),
	})
	if err != nil {
		return "", fmt.Errorf("error having Vault cross-sign cert: %v", err)
	}
	if response == nil || response.Data["certificate"] == "" {
		return "", fmt.Errorf("certificate info returned from Vault was blank")
	}

	xcCert, ok := response.Data["certificate"].(string)
	if !ok {
		return "", fmt.Errorf("certificate was not a string")
	}

	return xcCert, nil
}

// Cleanup unmounts the configured intermediate PKI backend. It's fine to tear
// this down and recreate it on small config changes because the intermediate
// certs get bundled with the leaf certs, so there's no cost to the CA changing.
func (v *VaultProvider) Cleanup() error {
	return v.client.Sys().Unmount(v.config.IntermediatePKIPath)
}

func ParseVaultCAConfig(raw map[string]interface{}) (*structs.VaultCAProviderConfig, error) {
	config := structs.VaultCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
	}

	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook:       structs.ParseDurationFunc(),
		Result:           &config,
		WeaklyTypedInput: true,
	}

	decoder, err := mapstructure.NewDecoder(decodeConf)
	if err != nil {
		return nil, err
	}

	if err := decoder.Decode(raw); err != nil {
		return nil, fmt.Errorf("error decoding config: %s", err)
	}

	if config.Token == "" {
		return nil, fmt.Errorf("must provide a Vault token")
	}

	if config.RootPKIPath == "" {
		return nil, fmt.Errorf("must provide a valid path to a root PKI backend")
	}
	if !strings.HasSuffix(config.RootPKIPath, "/") {
		config.RootPKIPath += "/"
	}

	if config.IntermediatePKIPath == "" {
		return nil, fmt.Errorf("must provide a valid path for the intermediate PKI backend")
	}
	if !strings.HasSuffix(config.IntermediatePKIPath, "/") {
		config.IntermediatePKIPath += "/"
	}

	if err := config.CommonCAProviderConfig.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}
