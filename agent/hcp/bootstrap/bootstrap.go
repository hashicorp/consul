// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package bootstrap handles bootstrapping an agent's config from HCP.
package bootstrap

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	hcpclient "github.com/hashicorp/consul/agent/hcp/client"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
)

const (
	SubDir = "hcp-config"

	CAFileName      = "server-tls-cas.pem"
	CertFileName    = "server-tls-cert.pem"
	ConfigFileName  = "server-config.json"
	KeyFileName     = "server-tls-key.pem"
	TokenFileName   = "hcp-management-token"
	SuccessFileName = "successful-bootstrap"
)

// UI is a shim to allow the agent command to pass in it's mitchelh/cli.UI so we
// can output useful messages to the user during bootstrapping. For example if
// we have to retry several times to bootstrap we don't want the agent to just
// stall with no output which is the case if we just returned all intermediate
// warnings or errors.
type UI interface {
	Output(string)
	Warn(string)
	Info(string)
	Error(string)
}

// RawBootstrapConfig contains the Consul config as a raw JSON string and the management token
// which either was retrieved from persisted files or from the bootstrap endpoint
type RawBootstrapConfig struct {
	ConfigJSON      string
	ManagementToken string
}

// FetchBootstrapConfig will fetch bootstrap configuration from remote servers and persist it to disk.
// It will retry until successful or a terminal error condition is found (e.g. permission denied).
func FetchBootstrapConfig(ctx context.Context, client hcpclient.Client, dataDir string, ui UI) (*RawBootstrapConfig, error) {
	w := retry.Waiter{
		MinWait: 1 * time.Second,
		MaxWait: 5 * time.Minute,
		Jitter:  retry.NewJitter(50),
	}

	for {
		// Note we don't want to shadow `ctx` here since we need that for the Wait
		// below.
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		cfg, err := fetchBootstrapConfig(reqCtx, client, dataDir)
		if err != nil {
			ui.Error(fmt.Sprintf("Error: failed to fetch bootstrap config from HCP, will retry in %s: %s",
				w.NextWait().Round(time.Second), err))
			if err := w.Wait(ctx); err != nil {
				return nil, err
			}
			// Finished waiting, restart loop
			continue
		}
		return cfg, nil
	}
}

// fetchBootstrapConfig will fetch  the bootstrap configuration from remote servers and persist it to disk.
func fetchBootstrapConfig(ctx context.Context, client hcpclient.Client, dataDir string) (*RawBootstrapConfig, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.FetchBootstrap(reqCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch bootstrap config from HCP: %w", err)
	}

	bsCfg := resp
	devMode := dataDir == ""
	cfgJSON, err := persistAndProcessConfig(dataDir, devMode, bsCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to persist config for existing cluster: %w", err)
	}

	return &RawBootstrapConfig{
		ConfigJSON:      cfgJSON,
		ManagementToken: bsCfg.ManagementToken,
	}, nil
}

// persistAndProcessConfig is called when we receive data from CCM.
// We validate and persist everything that was received, then also update
// the JSON config as needed.
func persistAndProcessConfig(dataDir string, devMode bool, bsCfg *hcpclient.BootstrapConfig) (string, error) {
	if devMode {
		// Agent in dev mode, we still need somewhere to persist the certs
		// temporarily though to be able to start up at all since we don't support
		// inline certs right now. Use temp dir
		tmp, err := os.MkdirTemp(os.TempDir(), "consul-dev-")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir for certificates: %w", err)
		}
		dataDir = tmp
	}

	// Create subdir if it's not already there.
	dir := filepath.Join(dataDir, SubDir)
	if err := lib.EnsurePath(dir, true); err != nil {
		return "", fmt.Errorf("failed to ensure directory %q: %w", dir, err)
	}

	// Parse just to a map for now as we only have to inject to a specific place
	// and parsing whole Config struct is complicated...
	var cfg map[string]any

	if err := json.Unmarshal([]byte(bsCfg.ConsulConfig), &cfg); err != nil {
		return "", fmt.Errorf("failed to unmarshal bootstrap config: %w", err)
	}

	// Avoid ever setting an initial_management token from HCP now that we can
	// separately bootstrap an HCP management token with a distinct accessor ID.
	//
	// CCM will continue to return an initial_management token because previous versions of Consul
	// cannot bootstrap an HCP management token distinct from the initial management token.
	// This block can be deleted once CCM supports tailoring bootstrap config responses
	// based on the version of Consul that requested it.
	acls, aclsOK := cfg["acl"].(map[string]any)
	if aclsOK {
		tokens, tokensOK := acls["tokens"].(map[string]interface{})
		if tokensOK {
			delete(tokens, "initial_management")
		}
	}

	var cfgJSON string
	if bsCfg.TLSCert != "" {
		if err := ValidateTLSCerts(bsCfg.TLSCert, bsCfg.TLSCertKey, bsCfg.TLSCAs); err != nil {
			return "", fmt.Errorf("invalid certificates: %w", err)
		}

		// Persist the TLS cert files from the response since we need to refer to them
		// as disk files either way.
		if err := persistTLSCerts(dir, bsCfg.TLSCert, bsCfg.TLSCertKey, bsCfg.TLSCAs); err != nil {
			return "", fmt.Errorf("failed to persist TLS certificates to dir %q: %w", dataDir, err)
		}

		// Store paths to the persisted TLS cert files.
		cfg["ca_file"] = filepath.Join(dir, CAFileName)
		cfg["cert_file"] = filepath.Join(dir, CertFileName)
		cfg["key_file"] = filepath.Join(dir, KeyFileName)

		// Convert the bootstrap config map back into a string
		cfgJSONBytes, err := json.Marshal(cfg)
		if err != nil {
			return "", err
		}
		cfgJSON = string(cfgJSONBytes)
	}

	if !devMode {
		// Persist the final config we need to add so that it is available locally after a restart.
		// Assuming the configured data dir wasn't a tmp dir to start with.
		if err := persistBootstrapConfig(dir, cfgJSON); err != nil {
			return "", fmt.Errorf("failed to persist bootstrap config: %w", err)
		}

		// HCP only returns the management token if it requires Consul to
		// initialize it
		if bsCfg.ManagementToken != "" {
			if err := validateManagementToken(bsCfg.ManagementToken); err != nil {
				return "", fmt.Errorf("invalid management token: %w", err)
			}
			if err := persistManagementToken(dir, bsCfg.ManagementToken); err != nil {
				return "", fmt.Errorf("failed to persist HCP management token: %w", err)
			}
		}

		if err := persistSuccessMarker(dir); err != nil {
			return "", fmt.Errorf("failed to persist success marker: %w", err)
		}
	}
	return cfgJSON, nil
}

func persistSuccessMarker(dir string) error {
	name := filepath.Join(dir, SuccessFileName)
	return os.WriteFile(name, []byte(""), 0600)

}

func persistTLSCerts(dir string, serverCert, serverKey string, caCerts []string) error {
	if serverCert == "" || serverKey == "" {
		return fmt.Errorf("unexpected bootstrap response from HCP: missing TLS information")
	}

	// Write out CA cert(s). We write them all to one file because Go's x509
	// machinery will read as many certs as it finds from each PEM file provided
	// and add them separaetly to the CertPool for validation
	f, err := os.OpenFile(filepath.Join(dir, CAFileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	bf := bufio.NewWriter(f)
	for _, caPEM := range caCerts {
		bf.WriteString(caPEM + "\n")
	}
	if err := bf.Flush(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, CertFileName), []byte(serverCert), 0600); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, KeyFileName), []byte(serverKey), 0600); err != nil {
		return err
	}

	return nil
}

// Basic validation to ensure a UUID was loaded and assumes the token is non-empty
func validateManagementToken(token string) error {
	// note: we assume that the token is not an empty string
	if _, err := uuid.ParseUUID(token); err != nil {
		return errors.New("management token is not a valid UUID")
	}
	return nil
}

func persistManagementToken(dir, token string) error {
	name := filepath.Join(dir, TokenFileName)
	return os.WriteFile(name, []byte(token), 0600)
}

func persistBootstrapConfig(dir, cfgJSON string) error {
	// Persist the important bits we got from bootstrapping. The TLS certs are
	// already persisted, just need to persist the config we are going to add.
	name := filepath.Join(dir, ConfigFileName)
	return os.WriteFile(name, []byte(cfgJSON), 0600)
}

func LoadPersistedBootstrapConfig(dataDir string, ui UI) (*RawBootstrapConfig, bool) {
	if dataDir == "" {
		// There's no files to load when in dev mode.
		return nil, false
	}

	dir := filepath.Join(dataDir, SubDir)

	_, err := os.Stat(filepath.Join(dir, SuccessFileName))
	if os.IsNotExist(err) {
		// Haven't bootstrapped from HCP.
		return nil, false
	}
	if err != nil {
		ui.Warn("failed to check for config on disk, re-fetching from HCP: " + err.Error())
		return nil, false
	}

	if err := checkCerts(dir); err != nil {
		ui.Warn("failed to validate certs on disk, re-fetching from HCP: " + err.Error())
		return nil, false
	}

	configJSON, err := loadBootstrapConfigJSON(dataDir)
	if err != nil {
		ui.Warn("failed to load bootstrap config from disk, re-fetching from HCP: " + err.Error())
		return nil, false
	}

	mgmtToken, err := loadManagementToken(dir)
	if err != nil {
		ui.Warn("failed to load HCP management token from disk, re-fetching from HCP: " + err.Error())
		return nil, false
	}

	return &RawBootstrapConfig{
		ConfigJSON:      configJSON,
		ManagementToken: mgmtToken,
	}, true
}

func loadBootstrapConfigJSON(dataDir string) (string, error) {
	filename := filepath.Join(dataDir, SubDir, ConfigFileName)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to check for bootstrap config: %w", err)
	}

	jsonBs, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf(fmt.Sprintf("failed to read local bootstrap config file: %s", err))
	}
	return strings.TrimSpace(string(jsonBs)), nil
}

func loadManagementToken(dir string) (string, error) {
	name := filepath.Join(dir, TokenFileName)
	bytes, err := os.ReadFile(name)
	if os.IsNotExist(err) {
		return "", errors.New("configuration files on disk are incomplete, missing: " + name)
	}
	if err != nil {
		return "", fmt.Errorf("failed to read: %w", err)
	}

	token := string(bytes)
	if err := validateManagementToken(token); err != nil {
		return "", fmt.Errorf("invalid management token: %w", err)
	}

	return token, nil
}

func checkCerts(dir string) error {
	files := []string{
		filepath.Join(dir, CAFileName),
		filepath.Join(dir, CertFileName),
		filepath.Join(dir, KeyFileName),
	}

	missing := make([]string, 0)
	for _, file := range files {
		_, err := os.Stat(file)
		if os.IsNotExist(err) {
			missing = append(missing, file)
			continue
		}
		if err != nil {
			return err
		}
	}

	// If all the TLS files are missing, assume this is intentional.
	// Existing clusters do not receive any TLS certs.
	if len(missing) == len(files) {
		return nil
	}

	// If only some of the files are missing, something went wrong.
	if len(missing) > 0 {
		return fmt.Errorf("configuration files on disk are incomplete, missing: %v", missing)
	}

	cert, key, caCerts, err := LoadCerts(dir)
	if err != nil {
		return fmt.Errorf("failed to load certs from disk: %w", err)
	}

	if err = ValidateTLSCerts(cert, key, caCerts); err != nil {
		return fmt.Errorf("invalid certs on disk: %w", err)
	}
	return nil
}

func LoadCerts(dir string) (cert, key string, caCerts []string, err error) {
	certPEMBlock, err := os.ReadFile(filepath.Join(dir, CertFileName))
	if err != nil {
		return "", "", nil, err
	}
	keyPEMBlock, err := os.ReadFile(filepath.Join(dir, KeyFileName))
	if err != nil {
		return "", "", nil, err
	}

	caPEMs, err := os.ReadFile(filepath.Join(dir, CAFileName))
	if err != nil {
		return "", "", nil, err
	}
	caCerts, err = splitCACerts(caPEMs)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse CA certs: %w", err)
	}

	return string(certPEMBlock), string(keyPEMBlock), caCerts, nil
}

// splitCACerts takes a list of concatenated PEM blocks and splits
// them back up into strings. This is used because CACerts are written
// into a single file, but validated individually.
func splitCACerts(caPEMs []byte) ([]string, error) {
	var out []string

	for {
		nextBlock, remaining := pem.Decode(caPEMs)
		if nextBlock == nil {
			break
		}
		if nextBlock.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("PEM-block should be CERTIFICATE type")
		}

		// Collect up to the start of the remaining bytes.
		// We don't grab nextBlock.Bytes because it's not PEM encoded.
		out = append(out, string(caPEMs[:len(caPEMs)-len(remaining)]))
		caPEMs = remaining
	}

	if len(out) == 0 {
		return nil, errors.New("invalid CA certificate")
	}
	return out, nil
}

// ValidateTLSCerts checks that the CA cert, server cert, and key on disk are structurally valid.
//
// OPTIMIZE: This could be improved by returning an error if certs are expired or close to expiration.
// However, that requires issuing new certs on bootstrap requests, since returning an error
// would trigger a re-fetch from HCP.
func ValidateTLSCerts(cert, key string, caCerts []string) error {
	leaf, err := tls.X509KeyPair([]byte(cert), []byte(key))
	if err != nil {
		return errors.New("invalid server certificate or key")
	}
	_, err = x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		return errors.New("invalid server certificate")
	}

	for _, caCert := range caCerts {
		_, err = connect.ParseCert(caCert)
		if err != nil {
			return errors.New("invalid CA certificate")
		}
	}
	return nil
}

// LoadManagementToken returns the management token, either by loading it from the persisted
// token config file or by fetching it from HCP if the token file does not exist.
func LoadManagementToken(ctx context.Context, logger hclog.Logger, client hcpclient.Client, dataDir string) (string, error) {
	hcpCfgDir := filepath.Join(dataDir, SubDir)
	token, err := loadManagementToken(hcpCfgDir)

	if err != nil {
		logger.Debug("fetching configuration from HCP")
		var err error
		cfg, err := fetchBootstrapConfig(ctx, client, dataDir)
		if err != nil {
			return "", err
		}
		logger.Debug("configuration fetched from HCP and saved on local disk")
		token = cfg.ManagementToken
	} else {
		logger.Trace("loaded HCP configuration from local disk")
	}

	return token, nil
}
