package consul

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/lib/routine"
	"github.com/hashicorp/consul/lib/semaphore"
)

type caState string

const (
	caStateUninitialized     caState = "UNINITIALIZED"
	caStateInitializing      caState = "INITIALIZING"
	caStateInitialized       caState = "INITIALIZED"
	caStateRenewIntermediate caState = "RENEWING"
	caStateReconfig          caState = "RECONFIGURING"
)

// caServerDelegate is an interface for server operations for facilitating
// easier testing.
type caServerDelegate interface {
	ca.ConsulProviderStateDelegate

	State() *state.Store
	IsLeader() bool
	ApplyCALeafRequest() (uint64, error)

	forwardDC(method, dc string, args interface{}, reply interface{}) error
	generateCASignRequest(csr string) *structs.CASignRequest

	ServersSupportMultiDCConnectCA() error
}

// CAManager is a wrapper around CA operations such as updating roots, an intermediate
// or the configuration. All operations should go through the CAManager in order to
// avoid data races.
type CAManager struct {
	delegate   caServerDelegate
	serverConf *Config
	logger     hclog.Logger
	// rate limiter to use when signing leaf certificates
	caLeafLimiter connectSignRateLimiter

	providerLock sync.RWMutex
	// provider is the current CA provider in use for Connect. This is
	// only non-nil when we are the leader.
	provider ca.Provider
	// providerRoot is the CARoot that was stored along with the ca.Provider
	// active. It's only updated in lock-step with the provider. This prevents
	// races between state updates to active roots and the fetch of the provider
	// instance.
	providerRoot *structs.CARoot

	// stateLock protects the internal state used for administrative CA tasks.
	stateLock    sync.Mutex
	state        caState
	primaryRoots structs.IndexedCARoots // The most recently seen state of the root CAs from the primary datacenter.

	leaderRoutineManager *routine.Manager
	// providerShim is used to test CAManager with a fake provider.
	providerShim ca.Provider

	// shim time.Now for testing
	timeNow func() time.Time
}

type caDelegateWithState struct {
	*Server
}

func (c *caDelegateWithState) State() *state.Store {
	return c.fsm.State()
}

func (c *caDelegateWithState) ApplyCARequest(req *structs.CARequest) (interface{}, error) {
	return c.Server.raftApplyMsgpack(structs.ConnectCARequestType, req)
}

func (c *caDelegateWithState) ApplyCALeafRequest() (uint64, error) {
	// TODO(banks): when we implement IssuedCerts table we can use the insert to
	// that as the raft index to return in response.
	//
	// UPDATE(mkeeler): The original implementation relied on updating the CAConfig
	// and using its index as the ModifyIndex for certs. This was buggy. The long
	// term goal is still to insert some metadata into raft about the certificates
	// and use that raft index for the ModifyIndex. This is a partial step in that
	// direction except that we only are setting an index and not storing the
	// metadata.
	req := structs.CALeafRequest{
		Op:         structs.CALeafOpIncrementIndex,
		Datacenter: c.Server.config.Datacenter,
	}
	resp, err := c.Server.raftApplyMsgpack(structs.ConnectCALeafRequestType|structs.IgnoreUnknownTypeFlag, &req)
	if err != nil {
		return 0, err
	}

	modIdx, ok := resp.(uint64)
	if !ok {
		return 0, fmt.Errorf("Invalid response from updating the leaf cert index")
	}
	return modIdx, err
}

func (c *caDelegateWithState) generateCASignRequest(csr string) *structs.CASignRequest {
	return &structs.CASignRequest{
		Datacenter:   c.Server.config.PrimaryDatacenter,
		CSR:          csr,
		WriteRequest: structs.WriteRequest{Token: c.Server.tokens.ReplicationToken()},
	}
}

func (c *caDelegateWithState) ServersSupportMultiDCConnectCA() error {
	versionOk, primaryFound := ServersInDCMeetMinimumVersion(c.Server, c.Server.config.PrimaryDatacenter, minMultiDCConnectVersion)
	if !primaryFound {
		return fmt.Errorf("primary datacenter is unreachable")
	}
	if !versionOk {
		return fmt.Errorf("all servers in the primary datacenter are not at the minimum version %v", minMultiDCConnectVersion)
	}
	return nil
}

func (c *caDelegateWithState) ProviderState(id string) (*structs.CAConsulProviderState, error) {
	_, s, err := c.fsm.State().CAProviderState(id)
	return s, err
}

func NewCAManager(delegate caServerDelegate, leaderRoutineManager *routine.Manager, logger hclog.Logger, config *Config) *CAManager {
	return &CAManager{
		delegate:             delegate,
		logger:               logger,
		serverConf:           config,
		state:                caStateUninitialized,
		leaderRoutineManager: leaderRoutineManager,
		timeNow:              time.Now,
	}
}

// setState attempts to update the CA state to the given state.
// Valid state transitions are:
//
// caStateInitialized -> <any state except caStateInitializing>
// caStateUninitialized -> caStateInitializing
// caStateUninitialized -> caStateReconfig
//
// Other state transitions may be forced if the validateState parameter is set to false.
// This will mainly be used in deferred functions which aim to set the final status based
// a successful/error return.
func (c *CAManager) setState(newState caState, validateState bool) (caState, error) {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()
	state := c.state

	if !validateState ||
		(state == caStateInitialized && newState != caStateInitializing) ||
		(state == caStateUninitialized && newState == caStateInitializing) ||
		(state == caStateUninitialized && newState == caStateReconfig) {
		c.state = newState
	} else {
		return state, &caStateError{Current: state}
	}
	return state, nil
}

type caStateError struct {
	Current caState
}

func (e *caStateError) Error() string {
	return fmt.Sprintf("CA is already in state %q", e.Current)
}

// secondarySetPrimaryRoots updates the most recently seen roots from the primary.
func (c *CAManager) secondarySetPrimaryRoots(newRoots structs.IndexedCARoots) {
	// TODO: this could be a different lock, as long as its the same lock in secondaryGetPrimaryRoots
	c.stateLock.Lock()
	defer c.stateLock.Unlock()
	c.primaryRoots = newRoots
}

func (c *CAManager) secondaryGetActivePrimaryCARoot() (*structs.CARoot, error) {
	// TODO: this could be a different lock, as long as its the same lock in secondarySetPrimaryRoots
	c.stateLock.Lock()
	primaryRoots := c.primaryRoots
	c.stateLock.Unlock()

	for _, root := range primaryRoots.Roots {
		if root.ID == primaryRoots.ActiveRootID && root.Active {
			return root, nil
		}
	}
	return nil, fmt.Errorf("primary datacenter does not have an active root CA for Connect")
}

// initializeCAConfig is used to initialize the CA config if necessary
// when setting up the CA during establishLeadership. The state should be set to
// non-ready before calling this.
func (c *CAManager) initializeCAConfig() (*structs.CAConfiguration, error) {
	st := c.delegate.State()
	_, config, err := st.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = c.serverConf.CAConfig

		if c.serverConf.Datacenter == c.serverConf.PrimaryDatacenter && config.ClusterID == "" {
			id, err := uuid.GenerateUUID()
			if err != nil {
				return nil, err
			}
			config.ClusterID = id
		}
	} else if _, ok := config.Config["IntermediateCertTTL"]; !ok {
		dup := *config
		copied := make(map[string]interface{})
		for k, v := range dup.Config {
			copied[k] = v
		}
		copied["IntermediateCertTTL"] = connect.DefaultIntermediateCertTTL.String()
		dup.Config = copied
		config = &dup
	} else {
		return config, nil
	}

	req := structs.CARequest{
		Op:     structs.CAOpSetConfig,
		Config: config,
	}
	if _, err := c.delegate.ApplyCARequest(&req); err != nil {
		return nil, err
	}
	return config, nil
}

// newCARoot returns a filled-in structs.CARoot from a raw PEM value.
func newCARoot(pemValue, provider, clusterID string) (*structs.CARoot, error) {
	primaryCert, err := connect.ParseCert(pemValue)
	if err != nil {
		return nil, err
	}
	keyType, keyBits, err := connect.KeyInfoFromCert(primaryCert)
	if err != nil {
		return nil, fmt.Errorf("error extracting root key info: %v", err)
	}
	return &structs.CARoot{
		ID:                  connect.CalculateCertFingerprint(primaryCert.Raw),
		Name:                fmt.Sprintf("%s CA Primary Cert", providerPrettyName(provider)),
		SerialNumber:        primaryCert.SerialNumber.Uint64(),
		SigningKeyID:        connect.EncodeSigningKeyID(primaryCert.SubjectKeyId),
		ExternalTrustDomain: clusterID,
		NotBefore:           primaryCert.NotBefore,
		NotAfter:            primaryCert.NotAfter,
		RootCert:            lib.EnsureTrailingNewline(pemValue),
		PrivateKeyType:      keyType,
		PrivateKeyBits:      keyBits,
		Active:              true,
	}, nil
}

// getCAProvider returns the currently active instance of the CA Provider,
// as well as the active root.
func (c *CAManager) getCAProvider() (ca.Provider, *structs.CARoot) {
	retries := 0
	for {
		c.providerLock.RLock()
		result := c.provider
		resultRoot := c.providerRoot
		c.providerLock.RUnlock()

		// In cases where an agent is started with managed proxies, we may ask
		// for the provider before establishLeadership completes. If we're the
		// leader, then wait and get the provider again
		if result == nil && c.delegate.IsLeader() && retries < 10 {
			retries++
			time.Sleep(50 * time.Millisecond)
			continue
		}

		return result, resultRoot
	}
}

// setCAProvider is being called while holding the stateLock
// which means it must never take that lock itself or call anything that does.
func (c *CAManager) setCAProvider(newProvider ca.Provider, root *structs.CARoot) {
	c.providerLock.Lock()
	c.provider = newProvider
	c.providerRoot = root
	c.providerLock.Unlock()
}

func (c *CAManager) Start(ctx context.Context) {
	// Attempt to initialize the Connect CA now. This will
	// happen during leader establishment and it would be great
	// if the CA was ready to go once that process was finished.
	if err := c.Initialize(); err != nil {
		c.logger.Error("Failed to initialize Connect CA", "error", err)

		// we failed to fully initialize the CA so we need to spawn a
		// go routine to retry this process until it succeeds or we lose
		// leadership and the go routine gets stopped.
		c.leaderRoutineManager.Start(ctx, backgroundCAInitializationRoutineName, c.backgroundCAInitialization)
	} else {
		// We only start these if CA initialization was successful. If not the completion of the
		// background CA initialization will start these routines.
		c.startPostInitializeRoutines(ctx)
	}
}

func (c *CAManager) Stop() {
	c.leaderRoutineManager.Stop(secondaryCARootWatchRoutineName)
	c.leaderRoutineManager.Stop(intermediateCertRenewWatchRoutineName)
	c.leaderRoutineManager.Stop(backgroundCAInitializationRoutineName)

	if provider, _ := c.getCAProvider(); provider != nil {
		if needsStop, ok := provider.(ca.NeedsStop); ok {
			needsStop.Stop()
		}
	}

	c.setState(caStateUninitialized, false)
	c.primaryRoots = structs.IndexedCARoots{}
	c.setCAProvider(nil, nil)
}

func (c *CAManager) startPostInitializeRoutines(ctx context.Context) {
	// Start the Connect secondary DC actions if enabled.
	if c.serverConf.Datacenter != c.serverConf.PrimaryDatacenter {
		c.leaderRoutineManager.Start(ctx, secondaryCARootWatchRoutineName, c.secondaryCARootWatch)
	}

	c.leaderRoutineManager.Start(ctx, intermediateCertRenewWatchRoutineName, c.runRenewIntermediate)
}

func (c *CAManager) backgroundCAInitialization(ctx context.Context) error {
	retryLoopBackoffAbortOnSuccess(ctx, c.Initialize, func(err error) {
		c.logger.Error("Failed to initialize Connect CA",
			"routine", backgroundCAInitializationRoutineName,
			"error", err,
		)
	})

	if err := ctx.Err(); err != nil {
		return err
	}

	c.logger.Info("Successfully initialized the Connect CA")

	c.startPostInitializeRoutines(ctx)
	return nil
}

// Initialize sets up the CA provider when gaining leadership, either bootstrapping
// the CA if this is the primary DC or making a remote RPC for intermediate signing
// if this is a secondary DC.
func (c *CAManager) Initialize() (reterr error) {
	// Bail if connect isn't enabled.
	if !c.serverConf.ConnectEnabled {
		return nil
	}

	// Update the state before doing anything else.
	_, err := c.setState(caStateInitializing, true)
	var errCaState *caStateError
	switch {
	case errors.As(err, &errCaState) && errCaState.Current == caStateInitialized:
		return nil
	case err != nil:
		return err
	}

	defer func() {
		// Using named return values in deferred funcs isnt too common in our code
		// but it is first class Go functionality. The error erturned from the
		// main func will be available by its given name within deferred functions.
		// See: https://blog.golang.org/defer-panic-and-recover
		if reterr == nil {
			c.setState(caStateInitialized, false)
		} else {
			c.setState(caStateUninitialized, false)
		}
	}()

	// Initialize the provider based on the current config.
	conf, err := c.initializeCAConfig()
	if err != nil {
		return err
	}
	provider, err := c.newProvider(conf)
	if err != nil {
		return err
	}

	c.setCAProvider(provider, nil)

	if c.serverConf.PrimaryDatacenter == c.serverConf.Datacenter {
		return c.primaryInitialize(provider, conf)
	}
	return c.secondaryInitialize(provider, conf)
}

func (c *CAManager) secondaryInitialize(provider ca.Provider, conf *structs.CAConfiguration) error {
	if err := c.delegate.ServersSupportMultiDCConnectCA(); err != nil {
		return fmt.Errorf("initialization will be deferred: %w", err)
	}

	// Get the root CA to see if we need to refresh our intermediate.
	args := structs.DCSpecificRequest{
		Datacenter: c.serverConf.PrimaryDatacenter,
	}
	var roots structs.IndexedCARoots
	if err := c.delegate.forwardDC("ConnectCA.Roots", c.serverConf.PrimaryDatacenter, &args, &roots); err != nil {
		return fmt.Errorf("failed to get CA roots from primary DC: %w", err)
	}
	c.secondarySetPrimaryRoots(roots)

	// Configure the CA provider and initialize the intermediate certificate if necessary.
	if err := c.secondaryInitializeProvider(provider, roots); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}
	if err := c.secondaryInitializeIntermediateCA(provider, nil); err != nil {
		return err
	}

	c.logger.Info("initialized secondary datacenter CA with provider", "provider", conf.Provider)
	return nil
}

// createProvider returns a connect CA provider from the given config.
func (c *CAManager) newProvider(conf *structs.CAConfiguration) (ca.Provider, error) {
	logger := c.logger.Named(conf.Provider)
	switch conf.Provider {
	case structs.ConsulCAProvider:
		return ca.NewConsulProvider(c.delegate, logger), nil
	case structs.VaultCAProvider:
		return ca.NewVaultProvider(logger), nil
	case structs.AWSCAProvider:
		return ca.NewAWSProvider(logger), nil
	default:
		if c.providerShim != nil {
			return c.providerShim, nil
		}
		return nil, fmt.Errorf("unknown CA provider %q", conf.Provider)
	}
}

// primaryInitialize runs the initialization logic for a root CA. It should only
// be called while the state lock is held by setting the state to non-ready.
func (c *CAManager) primaryInitialize(provider ca.Provider, conf *structs.CAConfiguration) error {
	pCfg := ca.ProviderConfig{
		ClusterID:  conf.ClusterID,
		Datacenter: c.serverConf.Datacenter,
		IsPrimary:  true,
		RawConfig:  conf.Config,
		State:      conf.State,
	}
	if err := provider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}
	root, err := provider.GenerateRoot()
	if err != nil {
		return fmt.Errorf("error generating CA root certificate: %v", err)
	}

	rootCA, err := newCARoot(root.PEM, conf.Provider, conf.ClusterID)
	if err != nil {
		return err
	}

	// TODO: https://github.com/hashicorp/consul/issues/12386
	interPEM, err := provider.GenerateIntermediate()
	if err != nil {
		return fmt.Errorf("error generating intermediate cert: %v", err)
	}
	intermediateCert, err := connect.ParseCert(interPEM)
	if err != nil {
		return fmt.Errorf("error getting intermediate cert: %v", err)
	}

	// If the provider has state to persist and it's changed or new then update
	// CAConfig.
	pState, err := provider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}
	if !reflect.DeepEqual(conf.State, pState) {
		// Update the CAConfig in raft to persist the provider state
		conf.State = pState
		req := structs.CARequest{
			Op:     structs.CAOpSetConfig,
			Config: conf,
		}
		if _, err = c.delegate.ApplyCARequest(&req); err != nil {
			return fmt.Errorf("error persisting provider state: %v", err)
		}
	}

	var rootUpdateRequired bool

	// Versions prior to 1.9.3, 1.8.8, and 1.7.12 incorrectly used the primary
	// rootCA's subjectKeyID here instead of the intermediate. For
	// provider=consul this didn't matter since there are no intermediates in
	// the primaryDC, but for vault it does matter.
	expectedSigningKeyID := connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)
	if rootCA.SigningKeyID != expectedSigningKeyID {
		c.logger.Info("Correcting stored CARoot values",
			"previous-signing-key", rootCA.SigningKeyID, "updated-signing-key", expectedSigningKeyID)
		rootCA.SigningKeyID = expectedSigningKeyID
		rootUpdateRequired = true
	}

	// Add the local leaf signing cert to the rootCA struct. This handles both
	// upgrades of existing state, and new rootCA.
	if c.getLeafSigningCertFromRoot(rootCA) != interPEM {
		rootCA.IntermediateCerts = append(rootCA.IntermediateCerts, interPEM)
		rootUpdateRequired = true
	}

	// Check if the CA root is already initialized and exit if it is,
	// adding on any existing intermediate certs since they aren't directly
	// tied to the provider.
	// Every change to the CA after this initial bootstrapping should
	// be done through the rotation process.
	state := c.delegate.State()
	_, activeRoot, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	if activeRoot != nil && !rootUpdateRequired {
		// This state shouldn't be possible to get into because we update the root and
		// CA config in the same FSM operation.
		if activeRoot.ID != rootCA.ID {
			return fmt.Errorf("stored CA root %q is not the active root (%s)", rootCA.ID, activeRoot.ID)
		}

		// TODO: why doesn't this c.setCAProvider(provider, activeRoot) ?
		rootCA.IntermediateCerts = activeRoot.IntermediateCerts
		c.setCAProvider(provider, rootCA)

		c.logger.Info("initialized primary datacenter CA from existing CARoot with provider", "provider", conf.Provider)
		return nil
	}

	if err := c.persistNewRootAndConfig(provider, rootCA, conf); err != nil {
		return err
	}
	c.setCAProvider(provider, rootCA)

	c.logger.Info("initialized primary datacenter CA with provider", "provider", conf.Provider)

	return nil
}

// getLeafSigningCertFromRoot returns the PEM encoded certificate that should be used to
// sign leaf certificates in the local datacenter. The SubjectKeyId of the
// returned cert should always match the SigningKeyID of the CARoot.
//
// TODO: fix the data model so that we don't need this complicated lookup to
// find the leaf signing cert. See github.com/hashicorp/consul/issues/11347.
func (c *CAManager) getLeafSigningCertFromRoot(root *structs.CARoot) string {
	if !c.isIntermediateUsedToSignLeaf() {
		return root.RootCert
	}
	if len(root.IntermediateCerts) == 0 {
		return ""
	}
	return root.IntermediateCerts[len(root.IntermediateCerts)-1]
}

// secondaryInitializeIntermediateCA generates a Certificate Signing Request (CSR)
// for the intermediate CA that is used to sign leaf certificates in the secondary.
// The CSR is signed by the primary DC and then persisted in the state store.
//
// This method should only be called while the state lock is held by setting the
// state to non-ready.
func (c *CAManager) secondaryInitializeIntermediateCA(provider ca.Provider, config *structs.CAConfiguration) error {
	activeIntermediate, err := provider.ActiveIntermediate()
	if err != nil {
		return err
	}

	_, activeRoot, err := c.delegate.State().CARootActive(nil)
	if err != nil {
		return err
	}
	var currentSigningKeyID string
	if activeRoot != nil {
		currentSigningKeyID = activeRoot.SigningKeyID
	}

	var expectedSigningKeyID string
	if activeIntermediate != "" {
		intermediateCert, err := connect.ParseCert(activeIntermediate)
		if err != nil {
			return fmt.Errorf("error parsing active intermediate cert: %v", err)
		}
		expectedSigningKeyID = connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)
	}

	newActiveRoot, err := c.secondaryGetActivePrimaryCARoot()
	if err != nil {
		return err
	}

	// Get a signed intermediate from the primary DC if the provider
	// hasn't been initialized yet or if the primary's root has changed.
	needsNewIntermediate := activeIntermediate == ""
	if activeRoot != nil && newActiveRoot.ID != activeRoot.ID {
		needsNewIntermediate = true
	}

	// Also we take this opportunity to correct an incorrectly persisted SigningKeyID
	// in secondary datacenters (see PR-6513).
	if expectedSigningKeyID != "" && currentSigningKeyID != expectedSigningKeyID {
		needsNewIntermediate = true
	}

	if needsNewIntermediate {
		if err := c.secondaryRequestNewSigningCert(provider, newActiveRoot); err != nil {
			return err
		}
	} else {
		// Discard the primary's representation since our local one is
		// sufficiently up to date.
		newActiveRoot = activeRoot
	}

	// Determine whether a root update is needed, and persist the roots/config accordingly.
	var newRoot *structs.CARoot
	if activeRoot == nil || needsNewIntermediate {
		newRoot = newActiveRoot
	}
	if err := c.persistNewRootAndConfig(provider, newRoot, config); err != nil {
		return err
	}

	c.setCAProvider(provider, newActiveRoot)
	return nil
}

// persistNewRootAndConfig should only be called while the state lock is held
// by setting the state to non-ready.
// If newActiveRoot is non-nil, it will be appended to the current roots list.
// If config is non-nil, it will be used to overwrite the existing config.
func (c *CAManager) persistNewRootAndConfig(provider ca.Provider, newActiveRoot *structs.CARoot, config *structs.CAConfiguration) error {
	state := c.delegate.State()
	idx, oldRoots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Look up the existing CA config if a new one wasn't provided.
	var newConf structs.CAConfiguration
	_, storedConfig, err := state.CAConfig(nil)
	if err != nil {
		return err
	}
	if storedConfig == nil {
		return fmt.Errorf("local CA not initialized yet")
	}
	// Exit early if the change is a no-op.
	if !shouldPersistNewRootAndConfig(newActiveRoot, storedConfig, config) {
		return nil
	}

	if config != nil {
		newConf = *config
	} else {
		newConf = *storedConfig
	}

	// Update the trust domain for the config if there's a new root, or keep the old
	// one if the root isn't being updated.
	newConf.ModifyIndex = storedConfig.ModifyIndex
	if newActiveRoot != nil {
		newConf.ClusterID = newActiveRoot.ExternalTrustDomain
	} else {
		_, activeRoot, err := state.CARootActive(nil)
		if err != nil {
			return err
		}
		newConf.ClusterID = activeRoot.ExternalTrustDomain
	}

	// Persist any state the provider needs us to
	newConf.State, err = provider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}

	// If there's a new active root, copy the root list and append it, updating
	// the old root with the time it was rotated out.
	var newRoots structs.CARoots
	for _, r := range oldRoots {
		newRoot := *r
		if newRoot.Active && newActiveRoot != nil {
			newRoot.Active = false
			newRoot.RotatedOutAt = c.timeNow()
		}
		if newRoot.ExternalTrustDomain == "" {
			newRoot.ExternalTrustDomain = newConf.ClusterID
		}
		newRoots = append(newRoots, &newRoot)
	}
	if newActiveRoot != nil {
		newRoots = append(newRoots, newActiveRoot)
	}

	args := &structs.CARequest{
		Op:     structs.CAOpSetRootsAndConfig,
		Index:  idx,
		Roots:  newRoots,
		Config: &newConf,
	}
	resp, err := c.delegate.ApplyCARequest(args)
	if err != nil {
		return err
	}
	if respOk, ok := resp.(bool); ok && !respOk {
		return fmt.Errorf("could not atomically update roots and config")
	}

	c.logger.Info("updated root certificates from primary datacenter")
	return nil
}

func shouldPersistNewRootAndConfig(newActiveRoot *structs.CARoot, oldConfig, newConfig *structs.CAConfiguration) bool {
	if newActiveRoot != nil {
		return true
	}

	if newConfig == nil {
		return false
	}
	return newConfig.Provider == oldConfig.Provider && reflect.DeepEqual(newConfig.Config, oldConfig.Config)
}

func (c *CAManager) UpdateConfiguration(args *structs.CARequest) (reterr error) {
	// Attempt to update the state first.
	oldState, err := c.setState(caStateReconfig, true)
	if err != nil {
		return err
	}
	defer func() {
		// Using named return values in deferred funcs isnt too common in our code
		// but it is first class Go functionality. The error erturned from the
		// main func will be available by its given name within deferred functions.
		// See: https://blog.golang.org/defer-panic-and-recover
		if reterr == nil {
			c.setState(caStateInitialized, false)
		} else {
			c.setState(oldState, false)
		}
	}()

	// Attempt to initialize the config if we failed to do so in Initialize for some reason
	prevConfig, err := c.initializeCAConfig()
	if err != nil {
		return err
	}

	// Exit early if it's a no-op change
	state := c.delegate.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}

	// Don't allow state changes. Either it needs to be empty or the same to allow
	// read-modify-write loops that don't touch the State field.
	if len(args.Config.State) > 0 &&
		!reflect.DeepEqual(args.Config.State, config.State) {
		return ErrStateReadOnly
	}

	// Don't allow users to change the ClusterID.
	args.Config.ClusterID = config.ClusterID
	if args.Config.Provider == config.Provider && reflect.DeepEqual(args.Config.Config, config.Config) {
		return nil
	}

	// If the provider hasn't changed, we need to load the current Provider state
	// so it can decide if it needs to change resources or not based on the config
	// change.
	if args.Config.Provider == config.Provider {
		// Note this is a shallow copy since the State method doc requires the
		// provider return a map that will not be further modified and should not
		// modify the one we pass to Configure.
		args.Config.State = config.State
	}

	// Create a new instance of the provider described by the config
	// and get the current active root CA. This acts as a good validation
	// of the config and makes sure the provider is functioning correctly
	// before we commit any changes to Raft.
	newProvider, err := c.newProvider(args.Config)
	if err != nil {
		return fmt.Errorf("could not initialize provider: %v", err)
	}
	pCfg := ca.ProviderConfig{
		ClusterID:  args.Config.ClusterID,
		Datacenter: c.serverConf.Datacenter,
		// This endpoint can be called in a secondary DC too so set this correctly.
		IsPrimary: c.serverConf.Datacenter == c.serverConf.PrimaryDatacenter,
		RawConfig: args.Config.Config,
		State:     args.Config.State,
	}

	if args.Config.Provider == config.Provider {
		if validator, ok := newProvider.(ValidateConfigUpdater); ok {
			if err := validator.ValidateConfigUpdate(prevConfig.Config, args.Config.Config); err != nil {
				return fmt.Errorf("new configuration is incompatible with previous configuration: %w", err)
			}
		}
	}

	if err := newProvider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}

	cleanupNewProvider := func() {
		if err := newProvider.Cleanup(args.Config.Provider != config.Provider, args.Config.Config); err != nil {
			c.logger.Warn("failed to clean up CA provider while handling startup failure", "provider", newProvider, "error", err)
		}
	}

	// If this is a secondary, just check if the intermediate needs to be regenerated.
	if c.serverConf.Datacenter != c.serverConf.PrimaryDatacenter {
		if err := c.secondaryInitializeIntermediateCA(newProvider, args.Config); err != nil {
			cleanupNewProvider()
			return fmt.Errorf("Error updating secondary datacenter CA config: %v", err)
		}
		c.logger.Info("Secondary CA provider config updated")
		return nil
	}
	if err := c.primaryUpdateRootCA(newProvider, args, config); err != nil {
		cleanupNewProvider()
		return err
	}
	return nil
}

// ValidateConfigUpdater is an optional interface that may be implemented
// by a ca.Provider. If the provider implements this interface, the
// ValidateConfigurationUpdate will be called when a user attempts to change the
// CA configuration, and the provider type has not changed from the previous
// configuration.
type ValidateConfigUpdater interface {
	// ValidateConfigUpdate should return an error if the next configuration is
	// incompatible with the previous configuration.
	//
	// TODO: use better types after https://github.com/hashicorp/consul/issues/12238
	ValidateConfigUpdate(previous, next map[string]interface{}) error
}

func (c *CAManager) primaryUpdateRootCA(newProvider ca.Provider, args *structs.CARequest, config *structs.CAConfiguration) error {
	providerRoot, err := newProvider.GenerateRoot()
	if err != nil {
		return fmt.Errorf("error generating CA root certificate: %v", err)
	}

	newRootPEM := providerRoot.PEM
	newActiveRoot, err := newCARoot(newRootPEM, args.Config.Provider, args.Config.ClusterID)
	if err != nil {
		return err
	}

	// TODO: https://github.com/hashicorp/consul/issues/12386
	intermediate, err := newProvider.ActiveIntermediate()
	if err != nil {
		return fmt.Errorf("error fetching active intermediate: %w", err)
	}
	if intermediate == "" {
		intermediate, err = newProvider.GenerateIntermediate()
		if err != nil {
			return fmt.Errorf("error generating intermediate: %w", err)
		}
	}
	if intermediate != newRootPEM {
		if err := setLeafSigningCert(newActiveRoot, intermediate); err != nil {
			return err
		}
	}

	// See if the provider needs to persist any state along with the config
	pState, err := newProvider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}
	args.Config.State = pState

	state := c.delegate.State()
	// Compare the new provider's root CA ID to the current one. If they
	// match, just update the existing provider with the new config.
	// If they don't match, begin the root rotation process.
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}

	// If the root didn't change, just update the config and return.
	if root != nil && root.ID == newActiveRoot.ID {
		args.Op = structs.CAOpSetConfig
		_, err := c.delegate.ApplyCARequest(args)
		if err != nil {
			return err
		}

		// If the config has been committed, update the local provider instance
		c.setCAProvider(newProvider, newActiveRoot)
		c.logger.Info("CA provider config updated")
		return nil
	}

	// get the old CA provider to be used for Cross Signing and to clean it up at the end
	// of the functi8on.
	oldProvider, _ := c.getCAProvider()
	if oldProvider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}

	// We only even think about cross signing if the current provider has a root cert
	// In some cases such as having a bad CA configuration during startup the provider
	// may not have been able to generate a cert. We then want to be able to prevent
	// an attempt to cross sign the cert which will definitely fail.
	if root != nil {
		// If it's a config change that would trigger a rotation (different provider/root):
		// 1. Get the root from the new provider.
		// 2. Call CrossSignCA on the old provider to sign the new root with the old one to
		// get a cross-signed certificate.
		// 3. Take the active root for the new provider and append the intermediate from step 2
		// to its list of intermediates.
		// TODO: this cert is already parsed once in newCARoot, could we remove the second parse?
		newRoot, err := connect.ParseCert(newRootPEM)
		if err != nil {
			return err
		}

		// At this point, we know the config change has triggered a root rotation,
		// either by swapping the provider type or changing the provider's config
		// to use a different root certificate.

		// First up, check that the current provider actually supports
		// cross-signing.
		canXSign, err := oldProvider.SupportsCrossSigning()
		if err != nil {
			return fmt.Errorf("CA provider error: %s", err)
		}
		if !canXSign && !args.Config.ForceWithoutCrossSigning {
			return errors.New("The current CA Provider does not support cross-signing. " +
				"You can try again with ForceWithoutCrossSigningSet but this may cause " +
				"disruption - see documentation for more.")
		}
		if args.Config.ForceWithoutCrossSigning {
			c.logger.Warn("ForceWithoutCrossSigning set, CA reconfiguration skipping cross-signing")
		}

		// If ForceWithoutCrossSigning wasn't set, attempt to have the old CA generate a
		// cross-signed intermediate.
		if canXSign && !args.Config.ForceWithoutCrossSigning {
			// Have the old provider cross-sign the new root
			xcCert, err := oldProvider.CrossSignCA(newRoot)
			if err != nil {
				return err
			}

			// Add the cross signed cert to the new CA's intermediates (to be attached
			// to leaf certs). We do not want it to be the last cert if there are any
			// existing intermediate certs so we push to the front.
			newActiveRoot.IntermediateCerts = append([]string{xcCert}, newActiveRoot.IntermediateCerts...)
		}
	}

	// Update the roots and CA config in the state store at the same time
	idx, roots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	var newRoots structs.CARoots
	for _, r := range roots {
		newRoot := *r
		if newRoot.Active {
			newRoot.Active = false
			newRoot.RotatedOutAt = c.timeNow()
		}
		newRoots = append(newRoots, &newRoot)
	}
	newRoots = append(newRoots, newActiveRoot)

	args.Op = structs.CAOpSetRootsAndConfig
	args.Index = idx
	args.Config.ModifyIndex = config.ModifyIndex
	args.Roots = newRoots
	resp, err := c.delegate.ApplyCARequest(args)
	if err != nil {
		return err
	}
	if respOk, ok := resp.(bool); ok && !respOk {
		return fmt.Errorf("could not atomically update roots and config")
	}

	// If the config has been committed, update the local provider instance
	// and call teardown on the old provider
	c.setCAProvider(newProvider, newActiveRoot)

	if err := oldProvider.Cleanup(args.Config.Provider != config.Provider, args.Config.Config); err != nil {
		c.logger.Warn("failed to clean up old provider", "provider", config.Provider, "error", err)
	}

	c.logger.Info("CA rotated to new root under provider", "provider", args.Config.Provider)

	return nil
}

// primaryRenewIntermediate regenerates the intermediate cert in the primary datacenter.
// This is only run for CAs that require an intermediary in the primary DC, such as Vault.
// It should only be called while the state lock is held by setting the state to non-ready.
func (c *CAManager) primaryRenewIntermediate(provider ca.Provider, newActiveRoot *structs.CARoot) error {
	// Generate and sign an intermediate cert using the root CA.
	intermediatePEM, err := provider.GenerateIntermediate()
	if err != nil {
		return fmt.Errorf("error generating new intermediate cert: %v", err)
	}

	if err := setLeafSigningCert(newActiveRoot, intermediatePEM); err != nil {
		return err
	}

	c.logger.Info("generated new intermediate certificate for primary datacenter")
	return nil
}

// secondaryRequestNewSigningCert creates a Certificate Signing Request, sends
// the request to the primary, and stores the received certificate in the
// provider.
// Should only be called while the state lock is held by setting the state to non-ready.
func (c *CAManager) secondaryRequestNewSigningCert(provider ca.Provider, newActiveRoot *structs.CARoot) error {
	csr, opaque, err := provider.GenerateIntermediateCSR()
	if err != nil {
		return err
	}

	var intermediatePEM string
	if err := c.delegate.forwardDC("ConnectCA.SignIntermediate", c.serverConf.PrimaryDatacenter, c.delegate.generateCASignRequest(csr), &intermediatePEM); err != nil {
		// this is a failure in the primary and shouldn't be capable of erroring out our establishing leadership
		c.logger.Warn("Primary datacenter refused to sign our intermediate CA certificate", "error", err)
		return nil
	}

	if err := provider.SetIntermediate(intermediatePEM, newActiveRoot.RootCert, opaque); err != nil {
		return fmt.Errorf("Failed to set the intermediate certificate with the CA provider: %v", err)
	}

	if err := setLeafSigningCert(newActiveRoot, intermediatePEM); err != nil {
		return fmt.Errorf("Failed to set the leaf signing cert to the intermediate: %w", err)
	}

	c.logger.Info("received new intermediate certificate from primary datacenter")
	return nil
}

// setLeafSigningCert updates the CARoot by appending the pem to the list of
// intermediate certificates, and setting the SigningKeyID to the encoded
// SubjectKeyId of the certificate.
func setLeafSigningCert(caRoot *structs.CARoot, pem string) error {
	cert, err := connect.ParseCert(pem)
	if err != nil {
		return fmt.Errorf("error parsing leaf signing cert: %w", err)
	}

	if err := pruneExpiredIntermediates(caRoot); err != nil {
		return err
	}

	caRoot.IntermediateCerts = append(caRoot.IntermediateCerts, pem)
	caRoot.SigningKeyID = connect.EncodeSigningKeyID(cert.SubjectKeyId)
	return nil
}

// pruneExpiredIntermediates removes expired intermediate certificates
// from the given CARoot.
func pruneExpiredIntermediates(caRoot *structs.CARoot) error {
	var newIntermediates []string
	now := time.Now()
	for _, intermediatePEM := range caRoot.IntermediateCerts {
		cert, err := connect.ParseCert(intermediatePEM)
		if err != nil {
			return fmt.Errorf("error parsing leaf signing cert: %w", err)
		}

		// Only keep the intermediate cert if it's still valid.
		if cert.NotAfter.After(now) {
			newIntermediates = append(newIntermediates, intermediatePEM)
		}
	}

	caRoot.IntermediateCerts = newIntermediates
	return nil
}

// runRenewIntermediate periodically attempts to renew the intermediate cert.
func (c *CAManager) runRenewIntermediate(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(structs.IntermediateCertRenewInterval):
			retryLoopBackoffAbortOnSuccess(ctx, func() error {
				return c.RenewIntermediate(ctx)
			}, func(err error) {
				c.logger.Error("error renewing intermediate certs",
					"routine", intermediateCertRenewWatchRoutineName,
					"error", err,
				)
			})
		}
	}
}

// RenewIntermediate checks the intermediate cert for
// expiration. If more than half the time a cert is valid has passed,
// it will try to renew it.
func (c *CAManager) RenewIntermediate(ctx context.Context) error {
	return c.renewIntermediate(ctx, false)
}

func (c *CAManager) renewIntermediateNow(ctx context.Context) error {
	return c.renewIntermediate(ctx, true)
}

func (c *CAManager) renewIntermediate(ctx context.Context, forceNow bool) error {
	// Grab the 'lock' right away so the provider/config can't be changed out while we check
	// the intermediate.
	if _, err := c.setState(caStateRenewIntermediate, true); err != nil {
		return err
	}
	defer c.setState(caStateInitialized, false)

	isPrimary := c.serverConf.InPrimaryDatacenter()

	provider, _ := c.getCAProvider()
	if provider == nil {
		// this happens when leadership is being revoked and this go routine will be stopped
		return nil
	}
	// If this isn't the primary, make sure the CA has been initialized.
	if !isPrimary && !c.secondaryHasProviderRoots() {
		return fmt.Errorf("secondary CA is not yet configured.")
	}

	state := c.delegate.State()
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	activeRoot := root.Clone()

	// If this is the primary, check if this is a provider that uses an intermediate cert. If
	// it isn't, we don't need to check for a renewal.
	if isPrimary && !primaryUsesIntermediate(provider) {
		return nil
	}

	activeIntermediate, err := provider.ActiveIntermediate()
	if err != nil {
		return err
	}

	if activeIntermediate == "" {
		return fmt.Errorf("datacenter doesn't have an active intermediate.")
	}

	intermediateCert, err := connect.ParseCert(activeIntermediate)
	if err != nil {
		return fmt.Errorf("error parsing active intermediate cert: %v", err)
	}

	if !forceNow {
		if lessThanHalfTimePassed(c.timeNow(), intermediateCert.NotBefore, intermediateCert.NotAfter) {
			return nil
		}
	}

	// Enough time has passed, go ahead with getting a new intermediate.
	renewalFunc := c.primaryRenewIntermediate
	if !isPrimary {
		renewalFunc = c.secondaryRequestNewSigningCert
	}

	if forceNow {
		err := renewalFunc(provider, activeRoot)
		if err != nil {
			return err
		}
	} else {
		errCh := make(chan error, 1)
		go func() {
			errCh <- renewalFunc(provider, activeRoot)
		}()

		// Wait for the renewal func to return or for the context to be canceled.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}

	if err := c.persistNewRootAndConfig(provider, activeRoot, nil); err != nil {
		return err
	}

	c.setCAProvider(provider, activeRoot)
	return nil
}

// secondaryCARootWatch maintains a blocking query to the primary datacenter's
// ConnectCA.Roots endpoint to monitor when it needs to request a new signed
// intermediate certificate.
func (c *CAManager) secondaryCARootWatch(ctx context.Context) error {
	args := structs.DCSpecificRequest{
		Datacenter: c.serverConf.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			// the maximum time the primary roots watch query can block before returning
			MaxQueryTime: c.serverConf.MaxQueryTime,
		},
	}

	c.logger.Debug("starting Connect CA root replication from primary datacenter", "primary", c.serverConf.PrimaryDatacenter)

	retryLoopBackoff(ctx, func() error {
		var roots structs.IndexedCARoots
		if err := c.delegate.forwardDC("ConnectCA.Roots", c.serverConf.PrimaryDatacenter, &args, &roots); err != nil {
			return fmt.Errorf("Error retrieving the primary datacenter's roots: %v", err)
		}

		// Return if the context has been canceled while waiting on the RPC.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Attempt to update the roots using the returned data.
		if err := c.secondaryUpdateRoots(roots); err != nil {
			return err
		}
		args.QueryOptions.MinQueryIndex = nextIndexVal(args.QueryOptions.MinQueryIndex, roots.QueryMeta.Index)
		return nil
	}, func(err error) {
		c.logger.Error("CA root replication failed, will retry",
			"routine", secondaryCARootWatchRoutineName,
			"error", err,
		)
	})

	return nil
}

// secondaryUpdateRoots updates the cached roots from the primary and regenerates the intermediate
// certificate if necessary.
func (c *CAManager) secondaryUpdateRoots(roots structs.IndexedCARoots) error {
	// Update the state first to claim the 'lock'.
	if _, err := c.setState(caStateReconfig, true); err != nil {
		return err
	}
	defer c.setState(caStateInitialized, false)

	// Update the cached primary roots now that the lock is held.
	c.secondarySetPrimaryRoots(roots)

	provider, _ := c.getCAProvider()
	if provider == nil {
		// this happens when leadership is being revoked and this go routine will be stopped
		return nil
	}

	// Run the secondary CA init routine to see if we need to request a new
	// intermediate.
	if c.secondaryHasProviderRoots() {
		if err := c.secondaryInitializeIntermediateCA(provider, nil); err != nil {
			return fmt.Errorf("Failed to initialize the secondary CA: %v", err)
		}
		return nil
	}

	// Attempt to initialize now that we have updated roots. This is an optimization
	// so that we don't have to wait for the Initialize retry backoff if we were
	// waiting on roots from the primary to be able to complete initialization.
	if err := c.delegate.ServersSupportMultiDCConnectCA(); err != nil {
		return fmt.Errorf("failed to initialize while updating primary roots: %w", err)
	}
	if err := c.secondaryInitializeProvider(provider, roots); err != nil {
		return fmt.Errorf("Failed to initialize secondary CA provider: %v", err)
	}
	if err := c.secondaryInitializeIntermediateCA(provider, nil); err != nil {
		return fmt.Errorf("Failed to initialize the secondary CA: %v", err)
	}
	return nil
}

// secondaryInitializeProvider configures the given provider for a secondary, non-root datacenter.
func (c *CAManager) secondaryInitializeProvider(provider ca.Provider, roots structs.IndexedCARoots) error {
	if roots.TrustDomain == "" {
		return fmt.Errorf("trust domain from primary datacenter is not initialized")
	}

	clusterID := strings.Split(roots.TrustDomain, ".")[0]
	_, conf, err := c.delegate.State().CAConfig(nil)
	if err != nil {
		return err
	}

	pCfg := ca.ProviderConfig{
		ClusterID:  clusterID,
		Datacenter: c.serverConf.Datacenter,
		IsPrimary:  false,
		RawConfig:  conf.Config,
		State:      conf.State,
	}
	if err := provider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}
	return nil
}

// secondaryHasProviderRoots returns true after providerRoot has been set. This
// method is used to detect when the secondary has received the roots from the
// primary DC.
func (c *CAManager) secondaryHasProviderRoots() bool {
	// TODO: this could potentially also use primaryRoots instead of providerRoot
	c.providerLock.Lock()
	defer c.providerLock.Unlock()
	return c.providerRoot != nil
}

type connectSignRateLimiter struct {
	// csrRateLimiter limits the rate of signing new certs if configured. Lazily
	// initialized from current config to support dynamic changes.
	// csrRateLimiterMu must be held while dereferencing the pointer or storing a
	// new one, but methods can be called on the limiter object outside of the
	// locked section. This is done only in the getCSRRateLimiterWithLimit method.
	csrRateLimiter   *rate.Limiter
	csrRateLimiterMu sync.RWMutex

	// csrConcurrencyLimiter is a dynamically resizable semaphore used to limit
	// Sign RPC concurrency if configured. The zero value is usable as soon as
	// SetSize is called which we do dynamically in the RPC handler to avoid
	// having to hook elaborate synchronization mechanisms through the CA config
	// endpoint and config reload etc.
	csrConcurrencyLimiter semaphore.Dynamic
}

// getCSRRateLimiterWithLimit returns a rate.Limiter with the desired limit set.
// It uses the shared server-wide limiter unless the limit has been changed in
// config or the limiter has not been setup yet in which case it just-in-time
// configures the new limiter. We assume that limit changes are relatively rare
// and that all callers (there is currently only one) use the same config value
// as the limit. There might be some flapping if there are multiple concurrent
// requests in flight at the time the config changes where A sees the new value
// and updates, B sees the old but then gets this lock second and changes back.
// Eventually though and very soon (once all current RPCs are complete) we are
// guaranteed to have the correct limit set by the next RPC that comes in so I
// assume this is fine. If we observe strange behavior because of it, we could
// add hysteresis that prevents changes too soon after a previous change but
// that seems unnecessary for now.
func (l *connectSignRateLimiter) getCSRRateLimiterWithLimit(limit rate.Limit) *rate.Limiter {
	l.csrRateLimiterMu.RLock()
	lim := l.csrRateLimiter
	l.csrRateLimiterMu.RUnlock()

	// If there is a current limiter with the same limit, return it. This should
	// be the common case.
	if lim != nil && lim.Limit() == limit {
		return lim
	}

	// Need to change limiter, get write lock
	l.csrRateLimiterMu.Lock()
	defer l.csrRateLimiterMu.Unlock()
	// No limiter yet, or limit changed in CA config, reconfigure a new limiter.
	// We use burst of 1 for a hard limit. Note that either bursting or waiting is
	// necessary to get expected behavior in fact of random arrival times, but we
	// don't need both and we use Wait with a small delay to smooth noise. See
	// https://github.com/banks/sim-rate-limit-backoff/blob/master/README.md.
	l.csrRateLimiter = rate.NewLimiter(limit, 1)
	return l.csrRateLimiter
}

// AuthorizeAndSignCertificate signs a leaf certificate for the service or agent
// identified by the SPIFFE ID in the given CSR's SAN. It performs authorization
// using the given acl.Authorizer.
func (c *CAManager) AuthorizeAndSignCertificate(csr *x509.CertificateRequest, authz acl.Authorizer) (*structs.IssuedCert, error) {
	// Note that only one spiffe id is allowed currently. If more than one is desired
	// in future implmentations, then each ID should have authorization checks.
	if len(csr.URIs) != 1 {
		return nil, connect.InvalidCSRError("CSR SAN contains an invalid number of URIs: %v", len(csr.URIs))
	}
	if len(csr.EmailAddresses) > 0 {
		return nil, connect.InvalidCSRError("CSR SAN does not allow specifying email addresses")
	}
	// Parse the SPIFFE ID from the CSR SAN.
	spiffeID, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return nil, err
	}

	// Perform authorization.
	var authzContext acl.AuthorizerContext
	allow := authz.ToAllowAuthorizer()
	switch v := spiffeID.(type) {
	case *connect.SpiffeIDService:
		v.GetEnterpriseMeta().FillAuthzContext(&authzContext)
		if err := allow.ServiceWriteAllowed(v.Service, &authzContext); err != nil {
			return nil, err
		}

		// Verify that the DC in the service URI matches us. We might relax this
		// requirement later but being restrictive for now is safer.
		dc := c.serverConf.Datacenter
		if v.Datacenter != dc {
			return nil, connect.InvalidCSRError("SPIFFE ID in CSR from a different datacenter: %s, "+
				"we are %s", v.Datacenter, dc)
		}
	case *connect.SpiffeIDAgent:
		v.GetEnterpriseMeta().FillAuthzContext(&authzContext)
		if err := allow.NodeWriteAllowed(v.Agent, &authzContext); err != nil {
			return nil, err
		}
	case *connect.SpiffeIDMeshGateway:
		// TODO(peering): figure out what is appropriate here for ACLs
		v.GetEnterpriseMeta().FillAuthzContext(&authzContext)
		if err := allow.MeshWriteAllowed(&authzContext); err != nil {
			return nil, err
		}

		// Verify that the DC in the gateway URI matches us. We might relax this
		// requirement later but being restrictive for now is safer.
		dc := c.serverConf.Datacenter
		if v.Datacenter != dc {
			return nil, connect.InvalidCSRError("SPIFFE ID in CSR from a different datacenter: %s, "+
				"we are %s", v.Datacenter, dc)
		}
	default:
		return nil, connect.InvalidCSRError("SPIFFE ID in CSR must be a service, mesh-gateway, or agent ID")
	}

	return c.SignCertificate(csr, spiffeID)
}

func (c *CAManager) SignCertificate(csr *x509.CertificateRequest, spiffeID connect.CertURI) (*structs.IssuedCert, error) {
	provider, caRoot := c.getCAProvider()
	if provider == nil {
		return nil, fmt.Errorf("CA is uninitialized and unable to sign certificates yet: provider is nil")
	} else if caRoot == nil {
		return nil, fmt.Errorf("CA is uninitialized and unable to sign certificates yet: no root certificate")
	}

	// Verify that the CSR entity is in the cluster's trust domain
	state := c.delegate.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	signingID := connect.SpiffeIDSigningForCluster(config.ClusterID)
	serviceID, isService := spiffeID.(*connect.SpiffeIDService)
	agentID, isAgent := spiffeID.(*connect.SpiffeIDAgent)
	mgwID, isMeshGateway := spiffeID.(*connect.SpiffeIDMeshGateway)

	var entMeta acl.EnterpriseMeta
	switch {
	case isService:
		if !signingID.CanSign(spiffeID) {
			return nil, connect.InvalidCSRError("SPIFFE ID in CSR from a different trust domain: %s, "+
				"we are %s", serviceID.Host, signingID.Host())
		}
		entMeta.Merge(serviceID.GetEnterpriseMeta())

	case isMeshGateway:
		if !signingID.CanSign(spiffeID) {
			return nil, connect.InvalidCSRError("SPIFFE ID in CSR from a different trust domain: %s, "+
				"we are %s", mgwID.Host, signingID.Host())
		}
		entMeta.Merge(mgwID.GetEnterpriseMeta())

	case isAgent:
		// isAgent - if we support more ID types then this would need to be an else if
		// here we are just automatically fixing the trust domain. For auto-encrypt and
		// auto-config they make certificate requests before learning about the roots
		// so they will have a dummy trust domain in the CSR.
		trustDomain := signingID.Host()
		if agentID.Host != trustDomain {
			originalURI := agentID.URI()

			agentID.Host = trustDomain

			// recreate the URIs list
			uris := make([]*url.URL, len(csr.URIs))
			for i, uri := range csr.URIs {
				if originalURI.String() == uri.String() {
					uris[i] = agentID.URI()
				} else {
					uris[i] = uri
				}
			}

			csr.URIs = uris
		}
		entMeta.Merge(agentID.GetEnterpriseMeta())

	default:
		return nil, connect.InvalidCSRError("SPIFFE ID in CSR must be a service, agent, or mesh gateway ID")
	}

	commonCfg, err := config.GetCommonConfig()
	if err != nil {
		return nil, err
	}
	if commonCfg.CSRMaxPerSecond > 0 {
		lim := c.caLeafLimiter.getCSRRateLimiterWithLimit(rate.Limit(commonCfg.CSRMaxPerSecond))
		// Wait up to the small threshold we allow for a token.
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if lim.Wait(ctx) != nil {
			return nil, ErrRateLimited
		}
	} else if commonCfg.CSRMaxConcurrent > 0 {
		c.caLeafLimiter.csrConcurrencyLimiter.SetSize(int64(commonCfg.CSRMaxConcurrent))
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if err := c.caLeafLimiter.csrConcurrencyLimiter.Acquire(ctx); err != nil {
			return nil, ErrRateLimited
		}
		defer c.caLeafLimiter.csrConcurrencyLimiter.Release()
	}

	connect.HackSANExtensionForCSR(csr)

	// Check if the root expired before using it to sign.
	// TODO: we store NotBefore and NotAfter on this struct, so we could avoid
	// parsing the cert here.
	err = c.checkExpired(caRoot.RootCert)
	if err != nil {
		return nil, fmt.Errorf("root expired: %w", err)
	}

	if c.isIntermediateUsedToSignLeaf() && len(caRoot.IntermediateCerts) > 0 {
		inter := caRoot.IntermediateCerts[len(caRoot.IntermediateCerts)-1]
		if err := c.checkExpired(inter); err != nil {
			return nil, fmt.Errorf("intermediate expired: %w", err)
		}
	}

	// All seems to be in order, actually sign it.
	pem, err := provider.Sign(csr)
	if err == ca.ErrRateLimited {
		return nil, ErrRateLimited
	}
	if err != nil {
		return nil, err
	}

	// Append any intermediates needed by this root.
	for _, p := range caRoot.IntermediateCerts {
		pem = pem + lib.EnsureTrailingNewline(p)
	}

	modIdx, err := c.delegate.ApplyCALeafRequest()
	if err != nil {
		return nil, err
	}

	cert, err := connect.ParseCert(pem)
	if err != nil {
		return nil, err
	}

	// Set the response
	reply := structs.IssuedCert{
		SerialNumber:   connect.EncodeSerialNumber(cert.SerialNumber),
		CertPEM:        pem,
		ValidAfter:     cert.NotBefore,
		ValidBefore:    cert.NotAfter,
		EnterpriseMeta: entMeta,
		RaftIndex: structs.RaftIndex{
			ModifyIndex: modIdx,
			CreateIndex: modIdx,
		},
	}

	switch {
	case isService:
		reply.Service = serviceID.Service
		reply.ServiceURI = cert.URIs[0].String()
	case isMeshGateway:
		reply.Kind = structs.ServiceKindMeshGateway
		reply.KindURI = cert.URIs[0].String()
	case isAgent:
		reply.Agent = agentID.Agent
		reply.AgentURI = cert.URIs[0].String()
	default:
		return nil, errors.New("not possible")
	}

	return &reply, nil
}

func (c *CAManager) checkExpired(pem string) error {
	cert, err := connect.ParseCert(pem)
	if err != nil {
		return err
	}
	if cert.NotAfter.Before(c.timeNow()) {
		return fmt.Errorf("certificate expired, expiration date: %s ", cert.NotAfter.String())
	}
	return nil
}

func primaryUsesIntermediate(provider ca.Provider) bool {
	_, ok := provider.(ca.PrimaryUsesIntermediate)
	return ok
}

func (c *CAManager) isIntermediateUsedToSignLeaf() bool {
	if c.serverConf.Datacenter != c.serverConf.PrimaryDatacenter {
		return true
	}
	provider, _ := c.getCAProvider()
	return primaryUsesIntermediate(provider)
}

func providerPrettyName(provider string) string {
	switch provider {
	case "consul":
		return "Consul"
	case "vault":
		return "Vault"
	case "aws-pca":
		return "Aws-Pca"
	case "provider-name":
		return "Provider-Name"
	default:
		return provider
	}
}
