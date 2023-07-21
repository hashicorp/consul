// Package bootstrap handles bootstrapping an agent's config from HCP. It must be a
// separate package from other HCP components because it has a dependency on
// agent/config while other components need to be imported and run within the
// server process in agent/consul and that would create a dependency cycle.
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

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/retry"
	"github.com/hashicorp/go-uuid"
)

const (
	subDir = "hcp-config"

	caFileName      = "server-tls-cas.pem"
	certFileName    = "server-tls-cert.pem"
	configFileName  = "server-config.json"
	keyFileName     = "server-tls-key.pem"
	tokenFileName   = "hcp-management-token"
	successFileName = "successful-bootstrap"
)

type ConfigLoader func(source config.Source) (config.LoadResult, error)

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

// LoadConfig will attempt to load previously-fetched config from disk and fall back to
// fetch from HCP servers if the local data is incomplete.
// It must be passed a (CLI) UI implementation so it can deliver progress
// updates to the user, for example if it is waiting to retry for a long period.
func LoadConfig(ctx context.Context, client hcp.Client, dataDir string, loader ConfigLoader, ui UI) (ConfigLoader, error) {
	ui.Output("Loading configuration from HCP")

	// See if we have existing config on disk
	//
	// OPTIMIZE: We could probably be more intelligent about config loading.
	// The currently implemented approach is:
	// 1. Attempt to load data from disk
	// 2. If that fails or the data is incomplete, block indefinitely fetching remote config.
	//
	// What if instead we had the following flow:
	// 1. Attempt to fetch config from HCP.
	// 2. If that fails, fall back to data on disk from last fetch.
	// 3. If that fails, go into blocking loop to fetch remote config.
	//
	// This should allow us to more gracefully transition cases like when
	// an existing cluster is linked, but then wants to receive TLS materials
	// at a later time. Currently, if we observe the existing-cluster marker we
	// don't attempt to fetch any additional configuration from HCP.

	cfg, ok := loadPersistedBootstrapConfig(dataDir, ui)
	if !ok {
		ui.Info("Fetching configuration from HCP servers")

		var err error
		cfg, err = fetchBootstrapConfig(ctx, client, dataDir, ui)
		if err != nil {
			return nil, fmt.Errorf("failed to bootstrap from HCP: %w", err)
		}
		ui.Info("Configuration fetched from HCP and saved on local disk")

	} else {
		ui.Info("Loaded HCP configuration from local disk")

	}

	// Create a new loader func to return
	newLoader := bootstrapConfigLoader(loader, cfg)
	return newLoader, nil
}

// bootstrapConfigLoader is a ConfigLoader for passing bootstrap JSON config received from HCP
// to the config.builder. ConfigLoaders are functions used to build an agent's RuntimeConfig
// from various sources like files and flags. This config is contained in the config.LoadResult.
//
// The flow to include bootstrap config from HCP as a loader's data source is as follows:
//
//  1. A base ConfigLoader function (baseLoader) is created on agent start, and it sets the input
//     source argument as the DefaultConfig.
//
//  2. When a server agent can be configured by HCP that baseLoader is wrapped in this bootstrapConfigLoader.
//
//  3. The bootstrapConfigLoader calls that base loader with the bootstrap JSON config as the
//     default source. This data will be merged with other valid sources in the config.builder.
//
//  4. The result of the call to baseLoader() below contains the resulting RuntimeConfig, and we do some
//     additional modifications to attach data that doesn't get populated during the build in the config pkg.
//
// Note that since the ConfigJSON is stored as the baseLoader's DefaultConfig, its data is the first
// to be merged by the config.builder and could be overwritten by user-provided values in config files or
// CLI flags. However, values set to RuntimeConfig after the baseLoader call are final.
func bootstrapConfigLoader(baseLoader ConfigLoader, cfg *RawBootstrapConfig) ConfigLoader {
	return func(source config.Source) (config.LoadResult, error) {
		// Don't allow any further attempts to provide a DefaultSource. This should
		// only ever be needed later in client agent AutoConfig code but that should
		// be mutually exclusive from this bootstrapping mechanism since this is
		// only for servers. If we ever try to change that, this clear failure
		// should alert future developers that the assumptions are changing rather
		// than quietly not applying the config they expect!
		if source != nil {
			return config.LoadResult{},
				fmt.Errorf("non-nil config source provided to a loader after HCP bootstrap already provided a DefaultSource")
		}

		// Otherwise, just call to the loader we were passed with our own additional
		// JSON as the source.
		//
		// OPTIMIZE: We could check/log whether any fields set by the remote config were overwritten by a user-provided flag.
		res, err := baseLoader(config.FileSource{
			Name:   "HCP Bootstrap",
			Format: "json",
			Data:   cfg.ConfigJSON,
		})
		if err != nil {
			return res, fmt.Errorf("failed to load HCP Bootstrap config: %w", err)
		}

		finalizeRuntimeConfig(res.RuntimeConfig, cfg)
		return res, nil
	}
}

const (
	accessControlHeaderName  = "Access-Control-Expose-Headers"
	accessControlHeaderValue = "x-consul-default-acl-policy"
)

// finalizeRuntimeConfig will set additional HCP-specific values that are not
// handled by the config.builder.
func finalizeRuntimeConfig(rc *config.RuntimeConfig, cfg *RawBootstrapConfig) {
	rc.Cloud.ManagementToken = cfg.ManagementToken

	// HTTP response headers are modified for the HCP UI to work.
	if rc.HTTPResponseHeaders == nil {
		rc.HTTPResponseHeaders = make(map[string]string)
	}
	prevValue, ok := rc.HTTPResponseHeaders[accessControlHeaderName]
	if !ok {
		rc.HTTPResponseHeaders[accessControlHeaderName] = accessControlHeaderValue
	} else {
		rc.HTTPResponseHeaders[accessControlHeaderName] = prevValue + "," + accessControlHeaderValue
	}
}

// fetchBootstrapConfig will fetch boostrap configuration from remote servers and persist it to disk.
// It will retry until successful or a terminal error condition is found (e.g. permission denied).
func fetchBootstrapConfig(ctx context.Context, client hcp.Client, dataDir string, ui UI) (*RawBootstrapConfig, error) {
	w := retry.Waiter{
		MinWait: 1 * time.Second,
		MaxWait: 5 * time.Minute,
		Jitter:  retry.NewJitter(50),
	}

	var bsCfg *hcp.BootstrapConfig
	for {
		// Note we don't want to shadow `ctx` here since we need that for the Wait
		// below.
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		resp, err := client.FetchBootstrap(reqCtx)
		if err != nil {
			ui.Error(fmt.Sprintf("Error: failed to fetch bootstrap config from HCP, will retry in %s: %s",
				w.NextWait().Round(time.Second), err))
			if err := w.Wait(ctx); err != nil {
				return nil, err
			}
			// Finished waiting, restart loop
			continue
		}
		bsCfg = resp
		break
	}

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
func persistAndProcessConfig(dataDir string, devMode bool, bsCfg *hcp.BootstrapConfig) (string, error) {
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
	dir := filepath.Join(dataDir, subDir)
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
		if err := validateTLSCerts(bsCfg.TLSCert, bsCfg.TLSCertKey, bsCfg.TLSCAs); err != nil {
			return "", fmt.Errorf("invalid certificates: %w", err)
		}

		// Persist the TLS cert files from the response since we need to refer to them
		// as disk files either way.
		if err := persistTLSCerts(dir, bsCfg.TLSCert, bsCfg.TLSCertKey, bsCfg.TLSCAs); err != nil {
			return "", fmt.Errorf("failed to persist TLS certificates to dir %q: %w", dataDir, err)
		}

		// Store paths to the persisted TLS cert files.
		cfg["ca_file"] = filepath.Join(dir, caFileName)
		cfg["cert_file"] = filepath.Join(dir, certFileName)
		cfg["key_file"] = filepath.Join(dir, keyFileName)

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
	name := filepath.Join(dir, successFileName)
	return os.WriteFile(name, []byte(""), 0600)

}

func persistTLSCerts(dir string, serverCert, serverKey string, caCerts []string) error {
	if serverCert == "" || serverKey == "" {
		return fmt.Errorf("unexpected bootstrap response from HCP: missing TLS information")
	}

	// Write out CA cert(s). We write them all to one file because Go's x509
	// machinery will read as many certs as it finds from each PEM file provided
	// and add them separaetly to the CertPool for validation
	f, err := os.OpenFile(filepath.Join(dir, caFileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
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

	if err := os.WriteFile(filepath.Join(dir, certFileName), []byte(serverCert), 0600); err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(dir, keyFileName), []byte(serverKey), 0600); err != nil {
		return err
	}

	return nil
}

// Basic validation to ensure a UUID was loaded.
func validateManagementToken(token string) error {
	if token == "" {
		return errors.New("missing HCP management token")
	}

	if _, err := uuid.ParseUUID(token); err != nil {
		return errors.New("management token is not a valid UUID")
	}
	return nil
}

func persistManagementToken(dir, token string) error {
	name := filepath.Join(dir, tokenFileName)
	return os.WriteFile(name, []byte(token), 0600)
}

func persistBootstrapConfig(dir, cfgJSON string) error {
	// Persist the important bits we got from bootstrapping. The TLS certs are
	// already persisted, just need to persist the config we are going to add.
	name := filepath.Join(dir, configFileName)
	return os.WriteFile(name, []byte(cfgJSON), 0600)
}

func loadPersistedBootstrapConfig(dataDir string, ui UI) (*RawBootstrapConfig, bool) {
	if dataDir == "" {
		// There's no files to load when in dev mode.
		return nil, false
	}

	dir := filepath.Join(dataDir, subDir)

	_, err := os.Stat(filepath.Join(dir, successFileName))
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
	filename := filepath.Join(dataDir, subDir, configFileName)

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to check for bootstrap config: %w", err)
	}

	// Attempt to load persisted config to check for errors and basic validity.
	// Errors here will raise issues like referencing unsupported config fields.
	_, err = config.Load(config.LoadOpts{
		ConfigFiles: []string{filename},
		HCL: []string{
			"server = true",
			`bind_addr = "127.0.0.1"`,
			fmt.Sprintf("data_dir = %q", dataDir),
		},
		ConfigFormat: "json",
	})
	if err != nil {
		return "", fmt.Errorf("failed to parse local bootstrap config: %w", err)
	}

	jsonBs, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf(fmt.Sprintf("failed to read local bootstrap config file: %s", err))
	}
	return strings.TrimSpace(string(jsonBs)), nil
}

func loadManagementToken(dir string) (string, error) {
	name := filepath.Join(dir, tokenFileName)
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
		filepath.Join(dir, caFileName),
		filepath.Join(dir, certFileName),
		filepath.Join(dir, keyFileName),
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

	cert, key, caCerts, err := loadCerts(dir)
	if err != nil {
		return fmt.Errorf("failed to load certs from disk: %w", err)
	}

	if err = validateTLSCerts(cert, key, caCerts); err != nil {
		return fmt.Errorf("invalid certs on disk: %w", err)
	}
	return nil
}

func loadCerts(dir string) (cert, key string, caCerts []string, err error) {
	certPEMBlock, err := os.ReadFile(filepath.Join(dir, certFileName))
	if err != nil {
		return "", "", nil, err
	}
	keyPEMBlock, err := os.ReadFile(filepath.Join(dir, keyFileName))
	if err != nil {
		return "", "", nil, err
	}

	caPEMs, err := os.ReadFile(filepath.Join(dir, caFileName))
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

// validateTLSCerts checks that the CA cert, server cert, and key on disk are structurally valid.
//
// OPTIMIZE: This could be improved by returning an error if certs are expired or close to expiration.
// However, that requires issuing new certs on bootstrap requests, since returning an error
// would trigger a re-fetch from HCP.
func validateTLSCerts(cert, key string, caCerts []string) error {
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
