// Package bootstrap handles bootstrapping an agent's config from HCP. It must be a
// separate package from other HCP components because it has a dependency on
// agent/config while other components need to be imported and run within the
// server process in agent/consul and that would create a dependency cycle.
package bootstrap

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/hcp"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/retry"
)

const (
	caFileName     = "server-tls-cas.pem"
	certFileName   = "server-tls-cert.pem"
	keyFileName    = "server-tls-key.pem"
	configFileName = "server-config.json"
	subDir         = "hcp-config"
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

// MaybeBootstrap will use the passed ConfigLoader to read the existing
// configuration, and if required attempt to bootstrap from HCP. It will retry
// until successful or a terminal error condition is found (e.g. permission
// denied). It must be passed a (CLI) UI implementation so it can deliver progress
// updates to the user, for example if it is waiting to retry for a long period.
func MaybeBootstrap(ctx context.Context, loader ConfigLoader, ui UI) (bool, ConfigLoader, error) {
	loader = wrapConfigLoader(loader)
	res, err := loader(nil)
	if err != nil {
		return false, nil, err
	}

	// Check to see if this is a server and HCP is configured

	if !res.RuntimeConfig.IsCloudEnabled() {
		// Not a server, let agent continue unmodified
		return false, loader, nil
	}

	ui.Output("Bootstrapping configuration from HCP")

	// See if we have existing config on disk
	cfgJSON, ok := loadPersistedBootstrapConfig(res.RuntimeConfig, ui)

	if !ok {
		// Fetch from HCP
		ui.Info("Fetching configuration from HCP")
		cfgJSON, err = doHCPBootstrap(ctx, res.RuntimeConfig, ui)
		if err != nil {
			return false, nil, fmt.Errorf("failed to bootstrap from HCP: %w", err)
		}
		ui.Info("Configuration fetched from HCP and saved on local disk")
	} else {
		ui.Info("Loaded configuration from local disk")
	}

	// Create a new loader func to return
	newLoader := func(source config.Source) (config.LoadResult, error) {
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
		s := config.FileSource{
			Name:   "HCP Bootstrap",
			Format: "json",
			Data:   cfgJSON,
		}
		return loader(s)
	}

	return true, newLoader, nil
}

func wrapConfigLoader(loader ConfigLoader) ConfigLoader {
	return func(source config.Source) (config.LoadResult, error) {
		res, err := loader(source)
		if err != nil {
			return res, err
		}

		if res.RuntimeConfig.Cloud.ResourceID == "" {
			res.RuntimeConfig.Cloud.ResourceID = os.Getenv("HCP_RESOURCE_ID")
		}
		return res, nil
	}
}

func doHCPBootstrap(ctx context.Context, rc *config.RuntimeConfig, ui UI) (string, error) {
	w := retry.Waiter{
		MinWait: 1 * time.Second,
		MaxWait: 5 * time.Minute,
		Jitter:  retry.NewJitter(50),
	}

	var bsCfg *hcp.BootstrapConfig

	client, err := hcp.NewClient(rc.Cloud)
	if err != nil {
		return "", err
	}

	for {
		// Note we don't want to shadow `ctx` here since we need that for the Wait
		// below.
		reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		resp, err := client.FetchBootstrap(reqCtx)
		if err != nil {
			ui.Error(fmt.Sprintf("failed to fetch bootstrap config from HCP, will retry in %s: %s",
				w.NextWait().Round(time.Second), err))
			if err := w.Wait(ctx); err != nil {
				return "", err
			}
			// Finished waiting, restart loop
			continue
		}
		bsCfg = resp
		break
	}

	dataDir := rc.DataDir
	shouldPersist := true
	if dataDir == "" {
		// Agent in dev mode, we still need somewhere to persist the certs
		// temporarily though to be able to start up at all since we don't support
		// inline certs right now. Use temp dir
		tmp, err := os.MkdirTemp(os.TempDir(), "consul-dev-")
		if err != nil {
			return "", fmt.Errorf("failed to create temp dir for certificates: %w", err)
		}
		dataDir = tmp
		shouldPersist = false
	}

	// Persist the TLS cert files from the response since we need to refer to them
	// as disk files either way.
	if err := persistTLSCerts(dataDir, bsCfg); err != nil {
		return "", fmt.Errorf("failed to persist TLS certificates to dir %q: %w", dataDir, err)
	}
	// Update the config JSON to include those TLS cert files
	cfgJSON, err := injectTLSCerts(dataDir, bsCfg.ConsulConfig)
	if err != nil {
		return "", fmt.Errorf("failed to inject TLS Certs into bootstrap config: %w", err)
	}

	// Persist the final config we need to add for restarts. Assuming this wasn't
	// a tmp dir to start with.
	if shouldPersist {
		if err := persistBootstrapConfig(dataDir, cfgJSON); err != nil {
			return "", fmt.Errorf("failed to persist bootstrap config to dir %q: %w", dataDir, err)
		}
	}

	return cfgJSON, nil
}

func persistTLSCerts(dataDir string, bsCfg *hcp.BootstrapConfig) error {
	dir := filepath.Join(dataDir, subDir)

	if bsCfg.TLSCert == "" || bsCfg.TLSCertKey == "" {
		return fmt.Errorf("unexpected bootstrap response from HCP: missing TLS information")
	}

	// Create a subdir if it's not already there
	if err := lib.EnsurePath(dir, true); err != nil {
		return err
	}

	// Write out CA cert(s). We write them all to one file because Go's x509
	// machinery will read as many certs as it finds from each PEM file provided
	// and add them separaetly to the CertPool for validation
	f, err := os.OpenFile(filepath.Join(dir, caFileName), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	bf := bufio.NewWriter(f)
	for _, caPEM := range bsCfg.TLSCAs {
		bf.WriteString(caPEM + "\n")
	}
	if err := bf.Flush(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(dir, certFileName), []byte(bsCfg.TLSCert), 0600); err != nil {
		return err
	}

	if err := ioutil.WriteFile(filepath.Join(dir, keyFileName), []byte(bsCfg.TLSCertKey), 0600); err != nil {
		return err
	}

	return nil
}

func injectTLSCerts(dataDir string, bootstrapJSON string) (string, error) {
	// Parse just to a map for now as we only have to inject to a specific place
	// and parsing whole Config struct is complicated...
	var cfg map[string]interface{}

	if err := json.Unmarshal([]byte(bootstrapJSON), &cfg); err != nil {
		return "", err
	}

	// Inject TLS cert files
	cfg["ca_file"] = filepath.Join(dataDir, subDir, caFileName)
	cfg["cert_file"] = filepath.Join(dataDir, subDir, certFileName)
	cfg["key_file"] = filepath.Join(dataDir, subDir, keyFileName)

	jsonBs, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}

	return string(jsonBs), nil
}

func persistBootstrapConfig(dataDir, cfgJSON string) error {
	// Persist the important bits we got from bootstrapping. The TLS certs are
	// already persisted, just need to persist the config we are going to add.
	name := filepath.Join(dataDir, subDir, configFileName)
	return ioutil.WriteFile(name, []byte(cfgJSON), 0600)
}

func loadPersistedBootstrapConfig(rc *config.RuntimeConfig, ui UI) (string, bool) {
	// Check if the files all exist
	files := []string{
		filepath.Join(rc.DataDir, subDir, configFileName),
		filepath.Join(rc.DataDir, subDir, caFileName),
		filepath.Join(rc.DataDir, subDir, certFileName),
		filepath.Join(rc.DataDir, subDir, keyFileName),
	}
	hasSome := false
	for _, name := range files {
		if _, err := os.Stat(name); errors.Is(err, os.ErrNotExist) {
			// At least one required file doesn't exist, failed loading. This is not
			// an error though
			if hasSome {
				ui.Warn("ignoring incomplete local bootstrap config files")
			}
			return "", false
		}
		hasSome = true
	}

	name := filepath.Join(rc.DataDir, subDir, configFileName)
	jsonBs, err := ioutil.ReadFile(name)
	if err != nil {
		ui.Warn(fmt.Sprintf("failed to read local bootstrap config file, ignoring local files: %s", err))
		return "", false
	}

	// Check this looks non-empty at least
	jsonStr := strings.TrimSpace(string(jsonBs))
	// 50 is arbitrary but config containing the right secrets would always be
	// bigger than this in JSON format so it is a reasonable test that this wasn't
	// empty or just an empty JSON object or something.
	if len(jsonStr) < 50 {
		ui.Warn("ignoring incomplete local bootstrap config files")
		return "", false
	}

	// TODO we could parse the certificates and check they are still valid here
	// and force a reload if not. We could also attempt to parse config and check
	// it's all valid just in case the local config was really old and has
	// deprecated fields or something?
	return jsonStr, true
}
