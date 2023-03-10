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
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	vaultapi "github.com/hashicorp/vault/api"
	"github.com/mitchellh/mapstructure"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/decode"
	"github.com/hashicorp/consul/lib/retry"
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

	retryMin    = 1 * time.Second
	retryMax    = 5 * time.Second
	retryJitter = 20
)

var ErrBackendNotMounted = fmt.Errorf("backend not mounted")
var ErrBackendNotInitialized = fmt.Errorf("backend not initialized")

type VaultProvider struct {
	config *structs.VaultCAProviderConfig

	client *vaultapi.Client
	// We modify the namespace on the fly to override default namespace for rootCertificate and intermediateCertificate. Can't guarantee
	// all operations (specifically Sign) are not called re-entrantly, so we add this for safety.
	clientMutex   sync.Mutex
	baseNamespace string

	stopWatcher func()

	isPrimary bool
	clusterID string
	spiffeID  *connect.SpiffeIDSigning
	logger    hclog.Logger

	// isConsulMountedIntermediate is used to determine if we should tune the
	// mount if the VaultProvider is ever reconfigured. This is at most a
	// "best guess" to determine whether this instance of Consul created the
	// intermediate mount but will not be able to tell if an existing mount
	// was created by Consul (in a previous running instance) or was external.
	isConsulMountedIntermediate bool
}

var _ Provider = (*VaultProvider)(nil)

func NewVaultProvider(logger hclog.Logger) *VaultProvider {
	return &VaultProvider{
		stopWatcher: func() {},
		logger:      logger,
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

	// We don't want to set the namespace if it's empty to prevent potential
	// unknown behavior (what does Vault do with an empty namespace). The Vault
	// client also makes sure the inputs are not empty strings so let's do the
	// same.
	if config.Namespace != "" {
		client.SetNamespace(config.Namespace)
		v.baseNamespace = config.Namespace
	}

	if config.AuthMethod != nil {
		loginResp, err := vaultLogin(client, config.AuthMethod)
		if err != nil {
			return err
		}
		config.Token = loginResp.Auth.ClientToken
	}
	client.SetToken(config.Token)

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
		if v.stopWatcher != nil {
			v.stopWatcher()
		}
		v.stopWatcher = cancel
		go v.renewToken(ctx, lifetimeWatcher)
	}

	// Update the intermediate (managed) PKI mount and role
	if err := v.setupIntermediatePKIPath(); err != nil {
		return err
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

	// TODO: Once we've upgraded to a later version of protobuf we can upgrade to github.com/hashicorp/vault/api@1.1.1
	// or later and rip this out.
	retrier := retry.Waiter{
		MinFailures: 5,
		MinWait:     retryMin,
		MaxWait:     retryMax,
		Jitter:      retry.NewJitter(retryJitter),
	}

	for {
		select {
		case <-ctx.Done():
			return

		case err := <-watcher.DoneCh():
			// In the event we fail to login to Vault or our token is no longer valid we can overwhelm a Vault instance
			// with rate limit configured. We would make these requests to Vault as fast as we possibly could and start
			// causing all client's to receive 429 response codes. To mitigate that we're sleeping 1 second or less
			// before moving on to login again and restart the lifetime watcher. Once we can upgrade to
			// github.com/hashicorp/vault/api@v1.1.1 or later the LifetimeWatcher _should_ perform that backoff for us.
			if err != nil {
				v.logger.Error("Error renewing token for Vault provider", "error", err)
			}

			// wait at least 1 second after returning from the lifetime watcher
			retrier.Wait(ctx)

			// If the watcher has exited and auth method is enabled,
			// re-authenticate using the auth method and set up a new watcher.
			if v.config.AuthMethod != nil {
				// Login to Vault using the auth method.
				loginResp, err := vaultLogin(v.client, v.config.AuthMethod)
				if err != nil {
					v.logger.Error("Error login in to Vault with %q auth method", v.config.AuthMethod.Type)
					// Restart the watcher
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
			retrier.Reset()
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
	rootPEM, err := v.getCA(v.config.RootPKINamespace, v.config.RootPKIPath)
	switch err {
	case ErrBackendNotMounted:

		err := v.mountNamespaced(v.config.RootPKINamespace, v.config.RootPKIPath, &vaultapi.MountInput{
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
			return RootResult{}, fmt.Errorf("failed to mount root CA backend: %w", err)
		}

		// We want to initialize afterwards
		fallthrough
	case ErrBackendNotInitialized:
		uid, err := connect.CompactUID()
		if err != nil {
			return RootResult{}, err
		}
		resp, err := v.writeNamespaced(v.config.RootPKINamespace, v.config.RootPKIPath+"root/generate/internal", map[string]interface{}{
			"common_name": connect.CACN("vault", uid, v.clusterID, v.isPrimary),
			"uri_sans":    v.spiffeID.URI().String(),
			"key_type":    v.config.PrivateKeyType,
			"key_bits":    v.config.PrivateKeyBits,
		})
		if err != nil {
			return RootResult{}, fmt.Errorf("failed to initialize root CA: %w", err)
		}
		var ok bool
		rootPEM, ok = resp.Data["certificate"].(string)
		if !ok {
			return RootResult{}, fmt.Errorf("unexpected response from Vault: %v", resp.Data["certificate"])
		}

	default:
		if err != nil {
			return RootResult{}, fmt.Errorf("unexpected error while setting root PKI backend: %w", err)
		}
	}

	rootChain, err := v.getCAChain(v.config.RootPKINamespace, v.config.RootPKIPath)
	if err != nil {
		return RootResult{}, err
	}

	// Workaround for a bug in the Vault PKI API.
	// See https://github.com/hashicorp/vault/issues/13489
	if rootChain == "" {
		rootChain = rootPEM
	}

	return RootResult{PEM: rootChain}, nil
}

// GenerateIntermediateCSR creates a private key and generates a CSR
// for another datacenter's root to sign, overwriting the intermediate backend
// in the process.
func (v *VaultProvider) GenerateIntermediateCSR() (string, string, error) {
	if v.isPrimary {
		return "", "", fmt.Errorf("provider is the root certificate authority, " +
			"cannot generate an intermediate CSR")
	}

	return v.generateIntermediateCSR()
}

func (v *VaultProvider) setupIntermediatePKIPath() error {
	mountConfig := vaultapi.MountConfigInput{
		MaxLeaseTTL: v.config.IntermediateCertTTL.String(),
	}

	_, err := v.getCA(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath)
	if err != nil {
		if err == ErrBackendNotMounted {
			err := v.mountNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath, &vaultapi.MountInput{
				Type:        "pki",
				Description: "intermediate CA backend for Consul Connect",
				Config:      mountConfig,
			})
			if err != nil {
				return fmt.Errorf("failed to mount intermediate PKI backend: %w", err)
			}
			// Required to determine if we should tune the mount
			// if the VaultProvider is ever reconfigured.
			v.isConsulMountedIntermediate = true

		} else if err == ErrBackendNotInitialized {
			// If this is the first time calling setupIntermediatePKIPath, the backend
			// will not have been initialized. Since the mount is ready we can suppress
			// this error.
		} else {
			return fmt.Errorf("unexpected error while fetching intermediate CA: %w", err)
		}
	} else {
		v.logger.Info("Found existing Intermediate PKI path mount",
			"namespace", v.config.IntermediatePKINamespace,
			"path", v.config.IntermediatePKIPath,
		)

		// This codepath requires the Vault policy:
		//
		//   path "/sys/mounts/<intermediate_pki_path>/tune" {
		//     capabilities = [ "update" ]
		//   }
		//
		err := v.tuneMountNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath, &mountConfig)
		if err != nil {
			if v.isConsulMountedIntermediate {
				v.logger.Warn("Intermediate PKI path was mounted by Consul but could not be tuned",
					"namespace", v.config.IntermediatePKINamespace,
					"path", v.config.IntermediatePKIPath,
					"error", err,
				)
			} else {
				v.logger.Debug("Failed to tune Intermediate PKI mount. 403 Forbidden is expected if Consul does not have tune capabilities for the Intermediate PKI mount (i.e. using Vault-managed policies)",
					"namespace", v.config.IntermediatePKINamespace,
					"path", v.config.IntermediatePKIPath,
					"error", err,
				)
			}

		}
	}

	// Create the role for issuing leaf certs
	rolePath := v.config.IntermediatePKIPath + "roles/" + VaultCALeafCertRole
	_, err = v.writeNamespaced(v.config.IntermediatePKINamespace, rolePath, map[string]interface{}{
		"allow_any_name":   true,
		"allowed_uri_sans": "spiffe://*",
		"key_type":         "any",
		"max_ttl":          v.config.LeafCertTTL.String(),
		"no_store":         true,
		"require_cn":       false,
	})

	return err
}

// generateIntermediateCSR returns the CSR and key_id (only present in
// Vault 1.11+) or any errors encountered.
func (v *VaultProvider) generateIntermediateCSR() (string, string, error) {
	// Generate a new intermediate CSR for the root to sign.
	uid, err := connect.CompactUID()
	if err != nil {
		return "", "", err
	}
	data, err := v.writeNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"intermediate/generate/internal", map[string]interface{}{
		"common_name": connect.CACN("vault", uid, v.clusterID, v.isPrimary),
		"key_type":    v.config.PrivateKeyType,
		"key_bits":    v.config.PrivateKeyBits,
		"uri_sans":    v.spiffeID.URI().String(),
	})
	if err != nil {
		return "", "", err
	}
	if data == nil || data.Data["csr"] == "" {
		return "", "", fmt.Errorf("got empty value when generating intermediate CSR")
	}
	csr, ok := data.Data["csr"].(string)
	if !ok {
		return "", "", fmt.Errorf("csr result is not a string")
	}
	// Vault 1.11+ will return a "key_id" field which helps
	// identify the correct issuer to set as default.
	// https://github.com/hashicorp/vault/blob/e445c8b4f58dc20a0316a7fd1b5725b401c3b17a/builtin/logical/pki/path_intermediate.go#L154
	if rawkeyId, ok := data.Data["key_id"]; ok {
		keyId, ok := rawkeyId.(string)
		if !ok {
			return "", "", fmt.Errorf("key_id is not a string")
		}
		return csr, keyId, nil
	}
	return csr, "", nil
}

// SetIntermediate writes the incoming intermediate and root certificates to the
// intermediate backend (as a chain).
func (v *VaultProvider) SetIntermediate(intermediatePEM, rootPEM, keyId string) error {
	if v.isPrimary {
		return fmt.Errorf("cannot set an intermediate using another root in the primary datacenter")
	}

	err := validateSetIntermediate(intermediatePEM, rootPEM, v.spiffeID)
	if err != nil {
		return err
	}

	importResp, err := v.writeNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"intermediate/set-signed", map[string]interface{}{
		"certificate": intermediatePEM,
	})
	if err != nil {
		return err
	}

	// Vault 1.11+ will return a non-nil response from intermediate/set-signed
	if importResp != nil {
		err := v.setDefaultIntermediateIssuer(importResp, keyId)
		if err != nil {
			return fmt.Errorf("failed to update default intermediate issuer: %w", err)
		}
	}

	return nil
}

// ActiveIntermediate returns the current intermediate certificate.
func (v *VaultProvider) ActiveIntermediate() (string, error) {
	cert, err := v.getCA(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath)

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
func (v *VaultProvider) getCA(namespace, path string) (string, error) {
	defer v.setNamespace(namespace)()

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

	root := lib.EnsureTrailingNewline(string(bytes))
	if root == "" {
		return "", ErrBackendNotInitialized
	}

	return root, nil
}

// TODO: refactor to remove duplication with getCA
func (v *VaultProvider) getCAChain(namespace, path string) (string, error) {
	defer v.setNamespace(namespace)()

	req := v.client.NewRequest("GET", "/v1/"+path+"/ca_chain")
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

	raw, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	root := lib.EnsureTrailingNewline(string(raw))
	return root, nil
}

// GenerateIntermediate mounts the configured intermediate PKI backend if
// necessary, then generates and signs a new CA CSR using the root PKI backend
// and updates the intermediate backend to use that new certificate.
func (v *VaultProvider) GenerateIntermediate() (string, error) {
	csr, keyId, err := v.generateIntermediateCSR()
	if err != nil {
		return "", err
	}

	// Sign the CSR with the root backend.
	intermediate, err := v.writeNamespaced(v.config.RootPKINamespace, v.config.RootPKIPath+"root/sign-intermediate", map[string]interface{}{
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
	importResp, err := v.writeNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"intermediate/set-signed", map[string]interface{}{
		"certificate": intermediate.Data["certificate"],
	})
	if err != nil {
		return "", err
	}

	// Vault 1.11+ will return a non-nil response from intermediate/set-signed
	if importResp != nil {
		err := v.setDefaultIntermediateIssuer(importResp, keyId)
		if err != nil {
			return "", fmt.Errorf("failed to update default intermediate issuer: %w", err)
		}
	}

	return v.ActiveIntermediate()
}

// setDefaultIntermediateIssuer updates the default issuer for
// intermediate CA since Vault, as part of its 1.11+ support for
// multiple issuers, no longer overwrites the default issuer when
// generateIntermediateCSR (intermediate/generate/internal) is called.
//
// The response we get from calling [/intermediate/set-signed]
// should contain a "mapping" data field we can use to cross-reference
// with the keyId returned when calling [/intermediate/generate/internal].
//
// [/intermediate/set-signed]: https://developer.hashicorp.com/vault/api-docs/secret/pki#import-ca-certificates-and-keys
// [/intermediate/generate/internal]: https://developer.hashicorp.com/vault/api-docs/secret/pki#generate-intermediate-csr
func (v *VaultProvider) setDefaultIntermediateIssuer(vaultResp *vaultapi.Secret, keyId string) error {
	if vaultResp.Data["mapping"] == nil {
		return fmt.Errorf("expected Vault response data to have a 'mapping' key")
	}
	if keyId == "" {
		return fmt.Errorf("expected non-empty keyId")
	}

	mapping, ok := vaultResp.Data["mapping"].(map[string]any)
	if !ok {
		return fmt.Errorf("unexpected type for 'mapping' value in Vault response")
	}

	var intermediateId string
	// The value in this KV pair is called "key"
	for issuer, key := range mapping {
		if key == keyId {
			// Expect to find the key_id we got from Vault when we
			// generated the intermediate CSR.
			intermediateId = issuer
			break
		}
	}
	if intermediateId == "" {
		return fmt.Errorf("could not find key_id %q in response from vault", keyId)
	}

	// For Vault 1.11+ it is important to GET then POST to avoid clobbering fields
	// like `default_follows_latest_issuer`.
	// https://developer.hashicorp.com/vault/api-docs/secret/pki#default_follows_latest_issuer
	resp, err := v.readNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"config/issuers")
	if err != nil {
		return fmt.Errorf("could not read from /config/issuers: %w", err)
	}
	issuersConf := resp.Data
	// Overwrite the default issuer
	issuersConf["default"] = intermediateId

	_, err = v.writeNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"config/issuers", issuersConf)
	if err != nil {
		return fmt.Errorf("could not write default issuer to /config/issuers: %w", err)
	}

	return nil
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
	response, err := v.writeNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath+"sign/"+VaultCALeafCertRole, map[string]interface{}{
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
	return lib.EnsureTrailingNewline(cert), nil
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
	data, err := v.writeNamespaced(v.config.RootPKINamespace, v.config.RootPKIPath+"root/sign-intermediate", map[string]interface{}{
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

	return lib.EnsureTrailingNewline(intermediate), nil
}

// CrossSignCA takes a CA certificate and cross-signs it to form a trust chain
// back to our active root.
func (v *VaultProvider) CrossSignCA(cert *x509.Certificate) (string, error) {
	rootPEM, err := v.getCA(v.config.RootPKINamespace, v.config.RootPKIPath)
	if err != nil {
		return "", fmt.Errorf("failed to get root CA: %w", err)
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
	response, err := v.writeNamespaced(v.config.RootPKINamespace, v.config.RootPKIPath+"root/sign-self-issued", map[string]interface{}{
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

	return lib.EnsureTrailingNewline(xcCert), nil
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

	err := v.unmountNamespaced(v.config.IntermediatePKINamespace, v.config.IntermediatePKIPath)

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
	v.stopWatcher()
}

func (v *VaultProvider) PrimaryUsesIntermediate() {}

// We use raw path here
func (v *VaultProvider) mountNamespaced(namespace, path string, mountInfo *vaultapi.MountInput) error {
	defer v.setNamespace(namespace)()
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/sys/mounts/%s", path))
	if err := r.SetJSONBody(mountInfo); err != nil {
		return err
	}
	resp, err := v.client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	return err
}

func (v *VaultProvider) tuneMountNamespaced(namespace, path string, mountConfig *vaultapi.MountConfigInput) error {
	defer v.setNamespace(namespace)()
	r := v.client.NewRequest("POST", fmt.Sprintf("/v1/sys/mounts/%s/tune", path))
	if err := r.SetJSONBody(mountConfig); err != nil {
		return err
	}
	resp, err := v.client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	return err
}

func (v *VaultProvider) unmountNamespaced(namespace, path string) error {
	defer v.setNamespace(namespace)()
	r := v.client.NewRequest("DELETE", fmt.Sprintf("/v1/sys/mounts/%s", path))
	resp, err := v.client.RawRequest(r)
	if resp != nil {
		defer resp.Body.Close()
	}
	return err
}

func makePathHelper(namespace, path string) string {
	var fullPath string
	if namespace != "" {
		fullPath = fmt.Sprintf("/v1/%s/sys/mounts/%s", namespace, path)
	} else {
		fullPath = fmt.Sprintf("/v1/sys/mounts/%s", path)
	}
	return fullPath
}

func (v *VaultProvider) readNamespaced(namespace string, resource string) (*vaultapi.Secret, error) {
	defer v.setNamespace(namespace)()
	return v.client.Logical().Read(resource)
}

func (v *VaultProvider) writeNamespaced(namespace string, resource string, data map[string]interface{}) (*vaultapi.Secret, error) {
	defer v.setNamespace(namespace)()
	return v.client.Logical().Write(resource, data)
}

func (v *VaultProvider) setNamespace(namespace string) func() {
	if namespace != "" {
		v.clientMutex.Lock()
		v.client.SetNamespace(namespace)
		return func() {
			v.client.SetNamespace(v.baseNamespace)
			v.clientMutex.Unlock()
		}
	} else {
		return func() {}
	}
}

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
