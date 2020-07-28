package autoconf

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/consul/agent/config"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/consul/proto/pbautoconf"
	"github.com/hashicorp/go-discover"
	discoverk8s "github.com/hashicorp/go-discover/provider/k8s"
	"github.com/hashicorp/go-hclog"

	"github.com/golang/protobuf/jsonpb"
)

const (
	// autoConfigFileName is the name of the file that the agent auto-config settings are
	// stored in within the data directory
	autoConfigFileName = "auto-config.json"

	dummyTrustDomain = "dummytrustdomain"
)

var (
	pbMarshaler = &jsonpb.Marshaler{
		OrigName:     false,
		EnumsAsInts:  false,
		Indent:       "   ",
		EmitDefaults: true,
	}

	pbUnmarshaler = &jsonpb.Unmarshaler{
		AllowUnknownFields: false,
	}
)

// AutoConfig is all the state necessary for being able to parse a configuration
// as well as perform the necessary RPCs to perform Agent Auto Configuration.
//
// NOTE: This struct and methods on it are not currently thread/goroutine safe.
// However it doesn't spawn any of its own go routines yet and is used in a
// synchronous fashion. In the future if either of those two conditions change
// then we will need to add some locking here. I am deferring that for now
// to help ease the review of this already large PR.
type AutoConfig struct {
	builderOpts    config.BuilderOpts
	logger         hclog.Logger
	directRPC      DirectRPC
	waiter         *lib.RetryWaiter
	overrides      []config.Source
	certMonitor    CertMonitor
	config         *config.RuntimeConfig
	autoConfigData string
	cancel         context.CancelFunc
}

// New creates a new AutoConfig object for providing automatic
// Consul configuration.
func New(config *Config) (*AutoConfig, error) {
	if config == nil {
		return nil, fmt.Errorf("must provide a config struct")
	}

	if config.DirectRPC == nil {
		return nil, fmt.Errorf("must provide a direct RPC delegate")
	}

	logger := config.Logger
	if logger == nil {
		logger = hclog.NewNullLogger()
	} else {
		logger = logger.Named(logging.AutoConfig)
	}

	waiter := config.Waiter
	if waiter == nil {
		waiter = lib.NewRetryWaiter(1, 0, 10*time.Minute, lib.NewJitterRandomStagger(25))
	}

	ac := &AutoConfig{
		builderOpts: config.BuilderOpts,
		logger:      logger,
		directRPC:   config.DirectRPC,
		waiter:      waiter,
		overrides:   config.Overrides,
		certMonitor: config.CertMonitor,
	}

	return ac, nil
}

// ReadConfig will parse the current configuration and inject any
// auto-config sources if present into the correct place in the parsing chain.
func (ac *AutoConfig) ReadConfig() (*config.RuntimeConfig, error) {
	src := config.Source{
		Name:   autoConfigFileName,
		Format: "json",
		Data:   ac.autoConfigData,
	}

	cfg, warnings, err := LoadConfig(ac.builderOpts, src, ac.overrides...)
	if err != nil {
		return cfg, err
	}

	for _, w := range warnings {
		ac.logger.Warn(w)
	}

	ac.config = cfg
	return cfg, nil
}

// restorePersistedAutoConfig will attempt to load the persisted auto-config
// settings from the data directory. It returns true either when there was an
// unrecoverable error or when the configuration was successfully loaded from
// disk. Recoverable errors, such as "file not found" are suppressed and this
// method will return false for the first boolean.
func (ac *AutoConfig) restorePersistedAutoConfig() (bool, error) {
	if ac.config.DataDir == "" {
		// no data directory means we don't have anything to potentially load
		return false, nil
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)
	ac.logger.Debug("attempting to restore any persisted configuration", "path", path)

	content, err := ioutil.ReadFile(path)
	if err == nil {
		rdr := strings.NewReader(string(content))

		var resp pbautoconf.AutoConfigResponse
		if err := pbUnmarshaler.Unmarshal(rdr, &resp); err != nil {
			return false, fmt.Errorf("failed to decode persisted auto-config data: %w", err)
		}

		if err := ac.update(&resp); err != nil {
			return false, fmt.Errorf("error restoring persisted auto-config response: %w", err)
		}

		ac.logger.Info("restored persisted configuration", "path", path)
		return true, nil
	}

	if !os.IsNotExist(err) {
		return true, fmt.Errorf("failed to load %s: %w", path, err)
	}

	// ignore non-existence errors as that is an indicator that we haven't
	// performed the auto configuration before
	return false, nil
}

// InitialConfiguration will perform a one-time RPC request to the configured servers
// to retrieve various cluster wide configurations. See the proto/pbautoconf/auto_config.proto
// file for a complete reference of what configurations can be applied in this manner.
// The returned configuration will be the new configuration with any auto-config settings
// already applied. If AutoConfig is not enabled this method will just parse any
// local configuration and return the built runtime configuration.
//
// The context passed in can be used to cancel the retrieval of the initial configuration
// like when receiving a signal during startup.
func (ac *AutoConfig) InitialConfiguration(ctx context.Context) (*config.RuntimeConfig, error) {
	if ac.config == nil {
		config, err := ac.ReadConfig()
		if err != nil {
			return nil, err
		}

		ac.config = config
	}

	if !ac.config.AutoConfig.Enabled {
		return ac.config, nil
	}

	ready, err := ac.restorePersistedAutoConfig()
	if err != nil {
		return nil, err
	}

	if !ready {
		ac.logger.Info("retrieving initial agent auto configuration remotely")
		if err := ac.getInitialConfiguration(ctx); err != nil {
			return nil, err
		}
	}

	// re-read the configuration now that we have our initial auto-config
	config, err := ac.ReadConfig()
	if err != nil {
		return nil, err
	}

	ac.config = config
	return ac.config, nil
}

// introToken is responsible for determining the correct intro token to use
// when making the initial AutoConfig.InitialConfiguration RPC request.
func (ac *AutoConfig) introToken() (string, error) {
	conf := ac.config.AutoConfig
	// without an intro token or intro token file we cannot do anything
	if conf.IntroToken == "" && conf.IntroTokenFile == "" {
		return "", fmt.Errorf("neither intro_token or intro_token_file settings are not configured")
	}

	token := conf.IntroToken
	if token == "" {
		// load the intro token from the file
		content, err := ioutil.ReadFile(conf.IntroTokenFile)
		if err != nil {
			return "", fmt.Errorf("Failed to read intro token from file: %w", err)
		}

		token = string(content)

		if token == "" {
			return "", fmt.Errorf("intro_token_file did not contain any token")
		}
	}

	return token, nil
}

// serverHosts is responsible for taking the list of server addresses and
// resolving any go-discover provider invocations. It will then return a list
// of hosts. These might be hostnames and is expected that DNS resolution may
// be performed after this function runs. Additionally these may contain ports
// so SplitHostPort could also be necessary.
func (ac *AutoConfig) serverHosts() ([]string, error) {
	servers := ac.config.AutoConfig.ServerAddresses

	providers := make(map[string]discover.Provider)
	for k, v := range discover.Providers {
		providers[k] = v
	}
	providers["k8s"] = &discoverk8s.Provider{}

	disco, err := discover.New(
		discover.WithUserAgent(lib.UserAgent()),
		discover.WithProviders(providers),
	)

	if err != nil {
		return nil, fmt.Errorf("Failed to create go-discover resolver: %w", err)
	}

	var addrs []string
	for _, addr := range servers {
		switch {
		case strings.Contains(addr, "provider="):
			resolved, err := disco.Addrs(addr, ac.logger.StandardLogger(&hclog.StandardLoggerOptions{InferLevels: true}))
			if err != nil {
				ac.logger.Error("failed to resolve go-discover auto-config servers", "configuration", addr, "err", err)
				continue
			}

			addrs = append(addrs, resolved...)
			ac.logger.Debug("discovered auto-config servers", "servers", resolved)
		default:
			addrs = append(addrs, addr)
		}
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("no auto-config server addresses available for use")
	}

	return addrs, nil
}

// resolveHost will take a single host string and convert it to a list of TCPAddrs
// This will process any port in the input as well as looking up the hostname using
// normal DNS resolution.
func (ac *AutoConfig) resolveHost(hostPort string) []net.TCPAddr {
	port := ac.config.ServerPort
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		if strings.Contains(err.Error(), "missing port in address") {
			host = hostPort
		} else {
			ac.logger.Warn("error splitting host address into IP and port", "address", hostPort, "error", err)
			return nil
		}
	} else {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			ac.logger.Warn("Parsed port is not an integer", "port", portStr, "error", err)
			return nil
		}
	}

	// resolve the host to a list of IPs
	ips, err := net.LookupIP(host)
	if err != nil {
		ac.logger.Warn("IP resolution failed", "host", host, "error", err)
		return nil
	}

	var addrs []net.TCPAddr
	for _, ip := range ips {
		addrs = append(addrs, net.TCPAddr{IP: ip, Port: port})
	}

	return addrs
}

// recordResponse takes an AutoConfig RPC response records it with the agent
// This will persist the configuration to disk (unless in dev mode running without
// a data dir) and will reload the configuration.
func (ac *AutoConfig) recordResponse(resp *pbautoconf.AutoConfigResponse) error {
	serialized, err := pbMarshaler.MarshalToString(resp)
	if err != nil {
		return fmt.Errorf("failed to encode auto-config response as JSON: %w", err)
	}

	if err := ac.update(resp); err != nil {
		return err
	}

	// now that we know the configuration is generally fine including TLS certs go ahead and persist it to disk.
	if ac.config.DataDir == "" {
		ac.logger.Debug("not persisting auto-config settings because there is no data directory")
		return nil
	}

	path := filepath.Join(ac.config.DataDir, autoConfigFileName)

	err = ioutil.WriteFile(path, []byte(serialized), 0660)
	if err != nil {
		return fmt.Errorf("failed to write auto-config configurations: %w", err)
	}

	ac.logger.Debug("auto-config settings were persisted to disk")

	return nil
}

// getInitialConfigurationOnce will perform full server to TCPAddr resolution and
// loop through each host trying to make the AutoConfig.InitialConfiguration RPC call. When
// successful the bool return will be true and the err value will indicate whether we
// successfully recorded the auto config settings (persisted to disk and stored internally
// on the AutoConfig object)
func (ac *AutoConfig) getInitialConfigurationOnce(ctx context.Context, csr string, key string) (*pbautoconf.AutoConfigResponse, error) {
	token, err := ac.introToken()
	if err != nil {
		return nil, err
	}

	request := pbautoconf.AutoConfigRequest{
		Datacenter: ac.config.Datacenter,
		Node:       ac.config.NodeName,
		Segment:    ac.config.SegmentName,
		JWT:        token,
		CSR:        csr,
	}

	var resp pbautoconf.AutoConfigResponse

	servers, err := ac.serverHosts()
	if err != nil {
		return nil, err
	}

	for _, s := range servers {
		// try each IP to see if we can successfully make the request
		for _, addr := range ac.resolveHost(s) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			ac.logger.Debug("making AutoConfig.InitialConfiguration RPC", "addr", addr.String())
			if err = ac.directRPC.RPC(ac.config.Datacenter, ac.config.NodeName, &addr, "AutoConfig.InitialConfiguration", &request, &resp); err != nil {
				ac.logger.Error("AutoConfig.InitialConfiguration RPC failed", "addr", addr.String(), "error", err)
				continue
			}

			// update the Certificate with the private key we generated locally
			if resp.Certificate != nil {
				resp.Certificate.PrivateKeyPEM = key
			}

			return &resp, nil
		}
	}

	return nil, ctx.Err()
}

// getInitialConfiguration implements a loop to retry calls to getInitialConfigurationOnce.
// It uses the RetryWaiter on the AutoConfig object to control how often to attempt
// the initial configuration process. It is also canceallable by cancelling the provided context.
func (ac *AutoConfig) getInitialConfiguration(ctx context.Context) error {
	// generate a CSR
	csr, key, err := ac.generateCSR()
	if err != nil {
		return err
	}

	// this resets the failures so that we will perform immediate request
	wait := ac.waiter.Success()
	for {
		select {
		case <-wait:
			resp, err := ac.getInitialConfigurationOnce(ctx, csr, key)
			if resp != nil {
				return ac.recordResponse(resp)
			} else if err != nil {
				ac.logger.Error(err.Error())
			} else {
				ac.logger.Error("No error returned when fetching the initial auto-configuration but no response was either")
			}
			wait = ac.waiter.Failed()
		case <-ctx.Done():
			ac.logger.Info("interrupted during initial auto configuration", "err", ctx.Err())
			return ctx.Err()
		}
	}
}

// generateCSR will generate a CSR for an Agent certificate. This should
// be sent along with the AutoConfig.InitialConfiguration RPC. The generated
// CSR does NOT have a real trust domain as when generating this we do
// not yet have the CA roots. The server will update the trust domain
// for us though.
func (ac *AutoConfig) generateCSR() (csr string, key string, err error) {
	// We don't provide the correct host here, because we don't know any
	// better at this point. Apart from the domain, we would need the
	// ClusterID, which we don't have. This is why we go with
	// dummyTrustDomain the first time. Subsequent CSRs will have the
	// correct TrustDomain.
	id := &connect.SpiffeIDAgent{
		// will be replaced
		Host:       dummyTrustDomain,
		Datacenter: ac.config.Datacenter,
		Agent:      ac.config.NodeName,
	}

	caConfig, err := ac.config.ConnectCAConfiguration()
	if err != nil {
		return "", "", fmt.Errorf("Cannot generate CSR: %w", err)
	}

	conf, err := caConfig.GetCommonConfig()
	if err != nil {
		return "", "", fmt.Errorf("Failed to load common CA configuration: %w", err)
	}

	if conf.PrivateKeyType == "" {
		conf.PrivateKeyType = connect.DefaultPrivateKeyType
	}
	if conf.PrivateKeyBits == 0 {
		conf.PrivateKeyBits = connect.DefaultPrivateKeyBits
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKeyWithConfig(conf.PrivateKeyType, conf.PrivateKeyBits)
	if err != nil {
		return "", "", fmt.Errorf("Failed to generate private key: %w", err)
	}

	dnsNames := append([]string{"localhost"}, ac.config.AutoConfig.DNSSANs...)
	ipAddresses := append([]net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::")}, ac.config.AutoConfig.IPSANs...)

	// Create a CSR.
	//
	// The Common Name includes the dummy trust domain for now but Server will
	// override this when it is signed anyway so it's OK.
	cn := connect.AgentCN(ac.config.NodeName, dummyTrustDomain)
	csr, err = connect.CreateCSR(id, cn, pk, dnsNames, ipAddresses)
	if err != nil {
		return "", "", err
	}

	return csr, pkPEM, nil
}

// update will take an AutoConfigResponse and do all things necessary
// to restore those settings. This currently involves updating the
// config data to be used during a call to ReadConfig, updating the
// tls Configurator and prepopulating the cache.
func (ac *AutoConfig) update(resp *pbautoconf.AutoConfigResponse) error {
	if err := ac.updateConfigFromResponse(resp); err != nil {
		return err
	}

	if err := ac.updateTLSFromResponse(resp); err != nil {
		return err
	}

	return nil
}

// updateConfigFromResponse is responsible for generating the JSON compatible with the
// agent/config.Config struct
func (ac *AutoConfig) updateConfigFromResponse(resp *pbautoconf.AutoConfigResponse) error {
	// here we want to serialize the translated configuration for use in injecting into the normal
	// configuration parsing chain.
	conf, err := json.Marshal(translateConfig(resp.Config))
	if err != nil {
		return fmt.Errorf("failed to encode auto-config configuration as JSON: %w", err)
	}

	ac.autoConfigData = string(conf)
	return nil
}

// updateTLSFromResponse will update the TLS certificate and roots in the shared
// TLS configurator.
func (ac *AutoConfig) updateTLSFromResponse(resp *pbautoconf.AutoConfigResponse) error {
	if ac.certMonitor == nil {
		return nil
	}

	roots, err := translateCARootsToStructs(resp.CARoots)
	if err != nil {
		return err
	}

	cert, err := translateIssuedCertToStructs(resp.Certificate)
	if err != nil {
		return err
	}

	update := &structs.SignedResponse{
		IssuedCert:     *cert,
		ConnectCARoots: *roots,
		ManualCARoots:  resp.ExtraCACertificates,
	}

	if resp.Config != nil && resp.Config.TLS != nil {
		update.VerifyServerHostname = resp.Config.TLS.VerifyServerHostname
	}

	if err := ac.certMonitor.Update(update); err != nil {
		return fmt.Errorf("failed to update the certificate monitor: %w", err)
	}

	return nil
}

func (ac *AutoConfig) Start(ctx context.Context) error {
	if ac.certMonitor == nil {
		return nil
	}

	if !ac.config.AutoConfig.Enabled {
		return nil
	}

	_, err := ac.certMonitor.Start(ctx)
	return err
}

func (ac *AutoConfig) Stop() bool {
	if ac.certMonitor == nil {
		return false
	}

	if !ac.config.AutoConfig.Enabled {
		return false
	}

	return ac.certMonitor.Stop()
}

func (ac *AutoConfig) FallbackTLS(ctx context.Context) (*structs.SignedResponse, error) {
	// generate a CSR
	csr, key, err := ac.generateCSR()
	if err != nil {
		return nil, err
	}

	resp, err := ac.getInitialConfigurationOnce(ctx, csr, key)
	if err != nil {
		return nil, err
	}

	return extractSignedResponse(resp)
}
