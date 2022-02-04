package ca

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/decode"
)

const (
	VaultCALeafCertRole = "leaf-cert"

	VaultAuthMethodTypeAliCloud     = "alicloud"
	VaultAuthMethodTypeAppRole      = "approle"
	VaultAuthMethodTypeAWS          = "aws"
	VaultAuthMethodTypeAzure        = "azure"
	VaultAuthMethodTypeCloudFoundry = "cf"
	VaultAuthMethodTypeGitHub       = "github"
	VaultAuthMethodTypeGCP          = "gcp"
	VaultAuthMethodTypeJWT          = "jwt"
	VaultAuthMethodTypeKerberos     = "kerberos"
	VaultAuthMethodTypeKubernetes   = "kubernetes"
	VaultAuthMethodTypeLDAP         = "ldap"
	VaultAuthMethodTypeOCI          = "oci"
	VaultAuthMethodTypeOkta         = "okta"
	VaultAuthMethodTypeRadius       = "radius"
	VaultAuthMethodTypeTLS          = "cert"
	VaultAuthMethodTypeToken        = "token"
	VaultAuthMethodTypeUserpass     = "userpass"

	defaultK8SServiceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

var ErrBackendNotMounted = fmt.Errorf("backend not mounted")
var ErrBackendNotInitialized = fmt.Errorf("backend not initialized")

type VaultProvider struct {
	config *structs.VaultCAProviderConfig
	client *vaultapi.Client

	shutdown func()

	isPrimary                    bool
	clusterID                    string
	spiffeID                     *connect.SpiffeIDSigning
	setupIntermediatePKIPathDone bool
	logger                       hclog.Logger
}

func NewVaultProvider(logger hclog.Logger) *VaultProvider {
	return &VaultProvider{
		shutdown: func() {},
		logger:   logger,
	}
}

func vaultTLSConfig(config *structs.VaultCAProviderConfig) *vaultapi.TLSConfig {
	return &vaultapi.TLSConfig{
		CACert:        config.CAFile,
		CAPath:        config.CAPath,
		ClientCert:    config.CertFile,
		ClientKey:     config.KeyFile,
		Insecure:      config.TLSSkipVerify,
		TLSServerName: config.TLSServerName,
	}
}

// Configure sets up the provider using the given configuration.
func (v *VaultProvider) Configure(cfg ProviderConfig) error {
	config, err := ParseVaultCAConfig(cfg.RawConfig)
	if err != nil {
		return err
	}

	clientConf := &vaultapi.Config{
		Address: config.Address,
	}
	err = clientConf.ConfigureTLS(vaultTLSConfig(config))
	if err != nil {
		return err
	}
	client, err := vaultapi.NewClient(clientConf)
	if err != nil {
		return err
	}

	if config.AuthMethod != nil {
		loginResp, err := vaultLogin(client, config.AuthMethod)
		if err != nil {
			return err
		}
		config.Token = loginResp.Auth.ClientToken
	}
	client.SetToken(config.Token)

	// We don't want to set the namespace if it's empty to prevent potential
	// unknown behavior (what does Vault do with an empty namespace). The Vault
	// client also makes sure the inputs are not empty strings so let's do the
	// same.
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
	}
	v.config = config
	v.client = client
	v.isPrimary = cfg.IsPrimary
	v.clusterID = cfg.ClusterID
	v.spiffeID = connect.SpiffeIDSigningForCluster(v.clusterID)

	// Look up the token to see if we can auto-renew its lease.
	secret, err := client.Auth().Token().LookupSelf()
	if err != nil {
		return err
	} else if secret == nil {
		return fmt.Errorf("could not look up Vault provider token: not found")
	}
	var token struct {
		Renewable bool
		TTL       int
	}
	if err := mapstructure.Decode(secret.Data, &token); err != nil {
		return err
	}

	// Set up a renewer to renew the token automatically, if supported.
	if token.Renewable || config.AuthMethod != nil {
		lifetimeWatcher, err := client.NewLifetimeWatcher(&vaultapi.LifetimeWatcherInput{
			Secret: &vaultapi.Secret{
				Auth: &vaultapi.SecretAuth{
					ClientToken:   config.Token,
					Renewable:     token.Renewable,
					LeaseDuration: secret.LeaseDuration,
				},
			},
			Increment:     token.TTL,
			RenewBehavior: vaultapi.RenewBehaviorIgnoreErrors,
		})
		if err != nil {
			return fmt.Errorf("error beginning Vault provider token renewal: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		v.shutdown = cancel
		go v.renewToken(ctx, lifetimeWatcher)
	}

	return nil
}

func (v *VaultProvider) ValidateConfigUpdate(prevRaw, nextRaw map[string]interface{}) error {
	prev, err := ParseVaultCAConfig(prevRaw)
	if err != nil {
		return fmt.Errorf("failed to parse existing CA config: %w", err)
	}
	next, err := ParseVaultCAConfig(nextRaw)
	if err != nil {
		return fmt.Errorf("failed to parse new CA config: %w", err)
	}

	if prev.RootPKIPath != next.RootPKIPath {
		return nil
	}

	if prev.PrivateKeyType != "" && prev.PrivateKeyType != connect.DefaultPrivateKeyType {
		if prev.PrivateKeyType != next.PrivateKeyType {
			return fmt.Errorf("cannot update the PrivateKeyType field without changing RootPKIPath")
		}
	}

	if prev.PrivateKeyBits != 0 && prev.PrivateKeyBits != connect.DefaultPrivateKeyBits {
		if prev.PrivateKeyBits != next.PrivateKeyBits {
			return fmt.Errorf("cannot update the PrivateKeyBits field without changing RootPKIPath")
		}
	}
	return nil
}

// renewToken uses a vaultapi.LifetimeWatcher to repeatedly renew our token's lease.
// If the token can no longer be renewed and auth method is set,
// it will re-authenticate to Vault using the auth method and restart the renewer with the new token.
func (v *VaultProvider) renewToken(ctx context.Context, watcher *vaultapi.LifetimeWatcher) {
	go watcher.Start()
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-watcher.DoneCh():
			if err != nil {
				v.logger.Error("Error renewing token for Vault provider", "error", err)
			}

			// If the watcher has exited and auth method is enabled,
			// re-authenticate using the auth method and set up a new watcher.
			if v.config.AuthMethod != nil {
				// Login to Vault using the auth method.
				loginResp, err := vaultLogin(v.client, v.config.AuthMethod)
				if err != nil {
					v.logger.Error("Error login in to Vault with %q auth method", v.config.AuthMethod.Type)
					// Restart the watcher.
					go watcher.Start()
					continue
				}

				// Set the new token for the vault client.
				v.client.SetToken(loginResp.Auth.ClientToken)
				v.logger.Info("Successfully re-authenticated with Vault using auth method")

				// Start the new watcher for the new token.
				watcher, err = v.client.NewLifetimeWatcher(&vaultapi.LifetimeWatcherInput{
					Secret:        loginResp,
					RenewBehavior: vaultapi.RenewBehaviorIgnoreErrors,
				})
				if err != nil {
					v.logger.Error("Error starting token renewal process")
					go watcher.Start()
					continue
				}
			}

			// Restart the watcher.
			go watcher.Start()

		case <-watcher.RenewCh():
			v.logger.Info("Successfully renewed token for Vault provider")
		}
	}
}

// State implements Provider. Vault provider needs no state other than the
// user-provided config currently.
func (v *VaultProvider) State() (map[string]string, error) {
	return nil, nil
}

// GenerateRoot mounts and initializes a new root PKI backend if needed.
func (v *VaultProvider) GenerateRoot() (RootResult, error) {
	if !v.isPrimary {
		return RootResult{}, fmt.Errorf("provider is not the root certificate authority")
	}

	// Set up the root PKI backend if necessary.
	rootPEM, err := v.getCA(v.config.RootPKIPath)
	switch err {
	case ErrBackendNotMounted:
		err := v.client.Sys().Mount(v.config.RootPKIPath, &vaultapi.MountInput{
			Type:        "pki",
			Description: "root CA backend for Consul Connect",
			Config: vaultapi.MountConfigInput{
				// the max lease ttl denotes the maximum ttl that secrets are created from the engine
				// the default lease ttl is the kind of ttl that will *reliably* set the ttl to v.config.RootCertTTL
				// https://www.vaultproject.io/docs/secrets/pki#configure-a-ca-certificate
				MaxLeaseTTL:     v.config.RootCertTTL.String(),
				DefaultLeaseTTL: v.config.RootCertTTL.String(),
			},
		})
		if err != nil {
			return RootResult{}, err
		}

		fallthrough
	case ErrBackendNotInitialized:
		uid, err := connect.CompactUID()
		if err != nil {
			return RootResult{}, err
		}
		_, err = v.client.Logical().Write(v.config.RootPKIPath+"root/generate/internal", map[string]interface{}{
			"common_name": connect.CACN("vault", uid, v.clusterID, v.isPrimary),
			"uri_sans":    v.spiffeID.URI().String(),
			"key_type":    v.config.PrivateKeyType,
			"key_bits":    v.config.PrivateKeyBits,
		})
		if err != nil {
			return RootResult{}, err
		}

		// retrieve the newly generated cert so that we can return it
		// TODO: is this already available from the Local().Write() above?
		rootPEM, err = v.getCA(v.config.RootPKIPath)
		if err != nil {
			return RootResult{}, err
		}

	default:
		if err != nil {
			return RootResult{}, err
		}
	}

	return RootResult{PEM: rootPEM}, nil
}

// GenerateIntermediateCSR creates a private key and generates a CSR
// for another datacenter's root to sign, overwriting the intermediate backend
// in the process.
func (v *VaultProvider) GenerateIntermediateCSR() (string, error) {
	if v.isPrimary {
		return "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	return v.generateIntermediateCSR()
}

func (v *VaultProvider) setupIntermediatePKIPath() error {
	if v.setupIntermediatePKIPathDone {
		return nil
	}
	mounts, err := v.client.Sys().ListMounts()
	if err != nil {
		return err
	}

	// Mount the backend if it isn't mounted already.
	if _, ok := mounts[v.config.IntermediatePKIPath]; !ok {
		err := v.client.Sys().Mount(v.config.IntermediatePKIPath, &vaultapi.MountInput{
			Type:        "pki",
			Description: "intermediate CA backend for Consul Connect",
			Config: vaultapi.MountConfigInput{
				MaxLeaseTTL: v.config.IntermediateCertTTL.String(),
			},
		})

		if err != nil {
			return err
		}
	}

	// Create the role for issuing leaf certs if it doesn't exist yet
	rolePath := v.config.IntermediatePKIPath + "roles/" + VaultCALeafCertRole
	role, err := v.client.Logical().Read(rolePath)
	if err != nil {
		return err
	}
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
			return err
		}
	}
	v.setupIntermediatePKIPathDone = true
	return nil
}

func (v *VaultProvider) generateIntermediateCSR() (string, error) {
	err := v.setupIntermediatePKIPath()
	if err != nil {
		return "", err
	}

	// Generate a new intermediate CSR for the root to sign.
	uid, err := connect.CompactUID()
	if err != nil {
		return "", err
	}
	data, err := v.client.Logical().Write(v.config.IntermediatePKIPath+"intermediate/generate/internal", map[string]interface{}{
		"common_name": connect.CACN("vault", uid, v.clusterID, v.isPrimary),
		"key_type":    v.config.PrivateKeyType,
		"key_bits":    v.config.PrivateKeyBits,
		"uri_sans":    v.spiffeID.URI().String(),
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
	if v.isPrimary {
		return fmt.Errorf("cannot set an intermediate using another root in the primary datacenter")
	}

	// the private key is in vault, so we can't use it in this validation
	err := validateSetIntermediate(intermediatePEM, rootPEM, "", v.spiffeID)
	if err != nil {
		return err
	}

	_, err = v.client.Logical().Write(v.config.IntermediatePKIPath+"intermediate/set-signed", map[string]interface{}{
		"certificate": intermediatePEM,
	})
	if err != nil {
		return err
	}

	return nil
}

// ActiveIntermediate returns the current intermediate certificate.
func (v *VaultProvider) ActiveIntermediate() (string, error) {
	if err := v.setupIntermediatePKIPath(); err != nil {
		return "", err
	}

	cert, err := v.getCA(v.config.IntermediatePKIPath)

	// This error is expected when calling initializeSecondaryCA for the
	// first time. It means that the backend is mounted and ready, but
	// there is no intermediate.
	// This error is swallowed because there is nothing the caller can do
	// about it. The caller needs to handle the empty cert though and
	// create an intermediate CA.
	if err == ErrBackendNotInitialized {
		return "", nil
	}
	return cert, err
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

	root := EnsureTrailingNewline(string(bytes))
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
		"csr":            csr,
		"use_csr_values": true,
		"format":         "pem_bundle",
		"ttl":            v.config.IntermediateCertTTL.String(),
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
	connect.HackSANExtensionForCSR(csr)

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

	return EnsureTrailingNewline(cert) + EnsureTrailingNewline(ca), nil
}

// SignIntermediate returns a signed CA certificate with a path length constraint
// of 0 to ensure that the certificate cannot be used to generate further CA certs.
func (v *VaultProvider) SignIntermediate(csr *x509.CertificateRequest) (string, error) {
	err := validateSignIntermediate(csr, v.spiffeID)
	if err != nil {
		return "", err
	}

	var pemBuf bytes.Buffer
	err = pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csr.Raw})
	if err != nil {
		return "", err
	}

	// Sign the CSR with the root backend.
	data, err := v.client.Logical().Write(v.config.RootPKIPath+"root/sign-intermediate", map[string]interface{}{
		"csr":             pemBuf.String(),
		"use_csr_values":  true,
		"format":          "pem_bundle",
		"max_path_length": 0,
		"ttl":             v.config.IntermediateCertTTL.String(),
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

	return EnsureTrailingNewline(intermediate), nil
}

// CrossSignCA takes a CA certificate and cross-signs it to form a trust chain
// back to our active root.
func (v *VaultProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	rootPEM, err := v.getCA(v.config.RootPKIPath)
	if err != nil {
		return "", err
	}
	rootCert, err := connect.ParseCert(rootPEM)
	if err != nil {
		return "", fmt.Errorf("error parsing root cert: %v", err)
	}
	if rootCert.NotAfter.Before(time.Now()) {
		return "", fmt.Errorf("root certificate is expired")
	}

	var pemBuf bytes.Buffer
	err = pem.Encode(&pemBuf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
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

	return EnsureTrailingNewline(xcCert), nil
}

// SupportsCrossSigning implements Provider
func (v *VaultProvider) SupportsCrossSigning() (bool, error) {
	return true, nil
}

// Cleanup unmounts the configured intermediate PKI backend. It's fine to tear
// this down and recreate it on small config changes because the intermediate
// certs get bundled with the leaf certs, so there's no cost to the CA changing.
func (v *VaultProvider) Cleanup(providerTypeChange bool, otherConfig map[string]interface{}) error {
	v.Stop()

	if !providerTypeChange {
		newConfig, err := ParseVaultCAConfig(otherConfig)
		if err != nil {
			return err
		}

		// if the intermeidate PKI path isn't changing we don't want to delete it as
		// Cleanup is called after initializing the new provider
		if newConfig.IntermediatePKIPath == v.config.IntermediatePKIPath {
			return nil
		}
	}

	err := v.client.Sys().Unmount(v.config.IntermediatePKIPath)

	switch err {
	case ErrBackendNotMounted, ErrBackendNotInitialized:
		// suppress these errors if we didn't finish initialization before
		return nil
	default:
		return err
	}
}

// Stop shuts down the token renew goroutine.
func (v *VaultProvider) Stop() {
	v.shutdown()
}

func (v *VaultProvider) PrimaryUsesIntermediate() {}

func ParseVaultCAConfig(raw map[string]interface{}) (*structs.VaultCAProviderConfig, error) {
	config := structs.VaultCAProviderConfig{
		CommonCAProviderConfig: defaultCommonConfig(),
	}

	decodeConf := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			structs.ParseDurationFunc(),
			decode.HookTranslateKeys,
		),
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

	if config.Token == "" && config.AuthMethod == nil {
		return nil, fmt.Errorf("must provide a Vault token or configure a Vault auth method")
	}

	if config.Token != "" && config.AuthMethod != nil {
		return nil, fmt.Errorf("only one of Vault token or Vault auth method can be provided, but not both")
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

func vaultLogin(client *vaultapi.Client, authMethod *structs.VaultAuthMethod) (*vaultapi.Secret, error) {
	// Adapted from https://www.vaultproject.io/docs/auth/kubernetes#code-example
	loginPath, err := configureVaultAuthMethod(authMethod)
	if err != nil {
		return nil, err
	}

	resp, err := client.Logical().Write(loginPath, authMethod.Params)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Auth == nil || resp.Auth.ClientToken == "" {
		return nil, fmt.Errorf("login response did not return client token")
	}

	return resp, nil
}

func configureVaultAuthMethod(authMethod *structs.VaultAuthMethod) (loginPath string, err error) {
	if authMethod.MountPath == "" {
		authMethod.MountPath = authMethod.Type
	}

	switch authMethod.Type {
	case VaultAuthMethodTypeKubernetes:
		// For the Kubernetes Auth method, we will try to read the JWT token
		// from the default service account file location if jwt was not provided.
		if jwt, ok := authMethod.Params["jwt"]; !ok || jwt == "" {
			serviceAccountToken, err := os.ReadFile(defaultK8SServiceAccountTokenPath)
			if err != nil {
				return "", err
			}

			authMethod.Params["jwt"] = string(serviceAccountToken)
		}
		loginPath = fmt.Sprintf("auth/%s/login", authMethod.MountPath)
	// These auth methods require a username for the login API path.
	case VaultAuthMethodTypeLDAP, VaultAuthMethodTypeUserpass, VaultAuthMethodTypeOkta, VaultAuthMethodTypeRadius:
		// Get username from the params.
		if username, ok := authMethod.Params["username"]; ok {
			loginPath = fmt.Sprintf("auth/%s/login/%s", authMethod.MountPath, username)
		} else {
			return "", fmt.Errorf("failed to get 'username' from auth method params")
		}
	// This auth method requires a role for the login API path.
	case VaultAuthMethodTypeOCI:
		if role, ok := authMethod.Params["role"]; ok {
			loginPath = fmt.Sprintf("auth/%s/login/%s", authMethod.MountPath, role)
		} else {
			return "", fmt.Errorf("failed to get 'role' from auth method params")
		}
	case VaultAuthMethodTypeToken:
		return "", fmt.Errorf("'token' auth method is not supported via auth method configuration; " +
			"please provide the token with the 'token' parameter in the CA configuration")
	// The rest of the auth methods use auth/<auth method path> login API path.
	case VaultAuthMethodTypeAliCloud,
		VaultAuthMethodTypeAppRole,
		VaultAuthMethodTypeAWS,
		VaultAuthMethodTypeAzure,
		VaultAuthMethodTypeCloudFoundry,
		VaultAuthMethodTypeGitHub,
		VaultAuthMethodTypeGCP,
		VaultAuthMethodTypeJWT,
		VaultAuthMethodTypeKerberos,
		VaultAuthMethodTypeTLS:
		loginPath = fmt.Sprintf("auth/%s/login", authMethod.MountPath)
	default:
		return "", fmt.Errorf("auth method %q is not supported", authMethod.Type)
	}

	return
}
