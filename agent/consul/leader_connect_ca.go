package consul

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"
)

type caState string

const (
	caStateUninitialized     caState = "UNINITIALIZED"
	caStateInitializing              = "INITIALIZING"
	caStateInitialized               = "INITIALIZED"
	caStateRenewIntermediate         = "RENEWING"
	caStateReconfig                  = "RECONFIGURING"
)

// caServerDelegate is an interface for server operations for facilitating
// easier testing.
type caServerDelegate interface {
	State() *state.Store
	IsLeader() bool

	createCAProvider(conf *structs.CAConfiguration) (ca.Provider, error)
	forwardDC(method, dc string, args interface{}, reply interface{}) error
	generateCASignRequest(csr string) *structs.CASignRequest
	raftApply(t structs.MessageType, msg interface{}) (interface{}, error)

	checkServersProvider
}

// CAManager is a wrapper around CA operations such as updating roots, an intermediate
// or the configuration. All operations should go through the CAManager in order to
// avoid data races.
type CAManager struct {
	delegate   caServerDelegate
	serverConf *Config
	logger     hclog.Logger

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
	stateLock         sync.Mutex
	state             caState
	primaryRoots      structs.IndexedCARoots // The most recently seen state of the root CAs from the primary datacenter.
	actingSecondaryCA bool                   // True if this datacenter has been initialized as a secondary CA.

	leaderRoutineManager *LeaderRoutineManager
}

type caDelegateWithState struct {
	*Server
}

func (c *caDelegateWithState) State() *state.Store {
	return c.fsm.State()
}

func NewCAManager(delegate caServerDelegate, leaderRoutineManager *LeaderRoutineManager, logger hclog.Logger, config *Config) *CAManager {
	return &CAManager{
		delegate:             delegate,
		logger:               logger,
		serverConf:           config,
		state:                caStateUninitialized,
		leaderRoutineManager: leaderRoutineManager,
	}
}

func (c *CAManager) reset() {
	c.state = caStateUninitialized
	c.primaryRoots = structs.IndexedCARoots{}
	c.actingSecondaryCA = false
	c.setCAProvider(nil, nil)
}

// setState attempts to update the CA state to the given state.
// Valid state transitions are:
//
// caStateInitialized -> <any state>
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
		state == caStateInitialized ||
		(state == caStateUninitialized && newState == caStateInitializing) ||
		(state == caStateUninitialized && newState == caStateReconfig) {
		c.state = newState
	} else {
		return state, fmt.Errorf("CA is already in state %q", state)
	}
	return state, nil
}

// setPrimaryRoots updates the most recently seen roots from the primary.
func (c *CAManager) setPrimaryRoots(newRoots structs.IndexedCARoots) error {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()

	if c.state == caStateInitializing || c.state == caStateReconfig {
		c.primaryRoots = newRoots
	} else {
		return fmt.Errorf("Cannot update primary roots in state %q", c.state)
	}
	return nil
}

func (c *CAManager) getPrimaryRoots() structs.IndexedCARoots {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()
	return c.primaryRoots
}

// initializeCAConfig is used to initialize the CA config if necessary
// when setting up the CA during establishLeadership. The state should be set to
// non-ready before calling this.
func (c *CAManager) initializeCAConfig() (*structs.CAConfiguration, error) {
	state := c.delegate.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = c.serverConf.CAConfig
		if config.ClusterID == "" {
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
	if resp, err := c.delegate.raftApply(structs.ConnectCARequestType, req); err != nil {
		return nil, err
	} else if respErr, ok := resp.(error); ok {
		return nil, respErr
	}

	return config, nil
}

// parseCARoot returns a filled-in structs.CARoot from a raw PEM value.
func parseCARoot(pemValue, provider, clusterID string) (*structs.CARoot, error) {
	id, err := connect.CalculateCertFingerprint(pemValue)
	if err != nil {
		return nil, fmt.Errorf("error parsing root fingerprint: %v", err)
	}
	rootCert, err := connect.ParseCert(pemValue)
	if err != nil {
		return nil, fmt.Errorf("error parsing root cert: %v", err)
	}
	keyType, keyBits, err := connect.KeyInfoFromCert(rootCert)
	if err != nil {
		return nil, fmt.Errorf("error extracting root key info: %v", err)
	}
	return &structs.CARoot{
		ID:                  id,
		Name:                fmt.Sprintf("%s CA Root Cert", strings.Title(provider)),
		SerialNumber:        rootCert.SerialNumber.Uint64(),
		SigningKeyID:        connect.EncodeSigningKeyID(rootCert.SubjectKeyId),
		ExternalTrustDomain: clusterID,
		NotBefore:           rootCert.NotBefore,
		NotAfter:            rootCert.NotAfter,
		RootCert:            pemValue,
		PrivateKeyType:      keyType,
		PrivateKeyBits:      keyBits,
		Active:              true,
	}, nil
}

// getCAProvider returns the currently active instance of the CA Provider,
// as well as the active root.
func (c *CAManager) getCAProvider() (ca.Provider, *structs.CARoot) {
	retries := 0
	var result ca.Provider
	var resultRoot *structs.CARoot
	for result == nil {
		c.providerLock.RLock()
		result = c.provider
		resultRoot = c.providerRoot
		c.providerLock.RUnlock()

		// In cases where an agent is started with managed proxies, we may ask
		// for the provider before establishLeadership completes. If we're the
		// leader, then wait and get the provider again
		if result == nil && c.delegate.IsLeader() && retries < 10 {
			retries++
			time.Sleep(50 * time.Millisecond)
			continue
		}

		break
	}

	return result, resultRoot
}

// setCAProvider is being called while holding the stateLock
// which means it must never take that lock itself or call anything that does.
func (c *CAManager) setCAProvider(newProvider ca.Provider, root *structs.CARoot) {
	c.providerLock.Lock()
	c.provider = newProvider
	c.providerRoot = root
	c.providerLock.Unlock()
}

func (c *CAManager) Start() {
	// Attempt to initialize the Connect CA now. This will
	// happen during leader establishment and it would be great
	// if the CA was ready to go once that process was finished.
	if err := c.InitializeCA(); err != nil {
		c.logger.Error("Failed to initialize Connect CA", "error", err)

		// we failed to fully initialize the CA so we need to spawn a
		// go routine to retry this process until it succeeds or we lose
		// leadership and the go routine gets stopped.
		c.leaderRoutineManager.Start(backgroundCAInitializationRoutineName, c.backgroundCAInitialization)
	} else {
		// We only start these if CA initialization was successful. If not the completion of the
		// background CA initialization will start these routines.
		c.startPostInitializeRoutines()
	}
}

func (c *CAManager) Stop() {
	c.leaderRoutineManager.Stop(secondaryCARootWatchRoutineName)
	c.leaderRoutineManager.Stop(intermediateCertRenewWatchRoutineName)
	c.leaderRoutineManager.Stop(backgroundCAInitializationRoutineName)
}

func (c *CAManager) startPostInitializeRoutines() {
	// Start the Connect secondary DC actions if enabled.
	if c.serverConf.Datacenter != c.serverConf.PrimaryDatacenter {
		c.leaderRoutineManager.Start(secondaryCARootWatchRoutineName, c.secondaryCARootWatch)
	}

	c.leaderRoutineManager.Start(intermediateCertRenewWatchRoutineName, c.intermediateCertRenewalWatch)
}

func (c *CAManager) backgroundCAInitialization(ctx context.Context) error {
	retryLoopBackoffAbortOnSuccess(ctx, c.InitializeCA, func(err error) {
		c.logger.Error("Failed to initialize Connect CA",
			"routine", backgroundCAInitializationRoutineName,
			"error", err,
		)
	})

	if err := ctx.Err(); err != nil {
		return err
	}

	c.logger.Info("Successfully initialized the Connect CA")

	c.startPostInitializeRoutines()
	return nil
}

// InitializeCA sets up the CA provider when gaining leadership, either bootstrapping
// the CA if this is the primary DC or making a remote RPC for intermediate signing
// if this is a secondary DC.
func (c *CAManager) InitializeCA() (reterr error) {
	// Bail if connect isn't enabled.
	if !c.serverConf.ConnectEnabled {
		return nil
	}

	// Update the state before doing anything else.
	oldState, err := c.setState(caStateInitializing, true)
	// if we were already in the initialized state then there is nothing to be done.
	if oldState == caStateInitialized {
		return nil
	}
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
			c.setState(caStateUninitialized, false)
		}
	}()

	// Initialize the provider based on the current config.
	conf, err := c.initializeCAConfig()
	if err != nil {
		return err
	}
	provider, err := c.delegate.createCAProvider(conf)
	if err != nil {
		return err
	}

	c.setCAProvider(provider, nil)

	// Run the root CA initialization if this is the primary DC.
	if c.serverConf.PrimaryDatacenter == c.serverConf.Datacenter {
		return c.initializeRootCA(provider, conf)
	}

	// If this isn't the primary DC, run the secondary DC routine if the primary has already been upgraded to at least 1.6.0
	versionOk, foundPrimary := ServersInDCMeetMinimumVersion(c.delegate, c.serverConf.PrimaryDatacenter, minMultiDCConnectVersion)
	if !foundPrimary {
		c.logger.Warn("primary datacenter is configured but unreachable - deferring initialization of the secondary datacenter CA")
		// return nil because we will initialize the secondary CA later
		return nil
	} else if !versionOk {
		// return nil because we will initialize the secondary CA later
		c.logger.Warn("servers in the primary datacenter are not at least at the minimum version - deferring initialization of the secondary datacenter CA",
			"min_version", minMultiDCConnectVersion.String(),
		)
		return nil
	}

	// Get the root CA to see if we need to refresh our intermediate.
	args := structs.DCSpecificRequest{
		Datacenter: c.serverConf.PrimaryDatacenter,
	}
	var roots structs.IndexedCARoots
	if err := c.delegate.forwardDC("ConnectCA.Roots", c.serverConf.PrimaryDatacenter, &args, &roots); err != nil {
		return err
	}
	if err := c.setPrimaryRoots(roots); err != nil {
		return err
	}

	// Configure the CA provider and initialize the intermediate certificate if necessary.
	if err := c.initializeSecondaryProvider(provider, roots); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}
	if err := c.initializeSecondaryCA(provider, nil); err != nil {
		return err
	}

	c.logger.Info("initialized secondary datacenter CA with provider", "provider", conf.Provider)
	return nil
}

// initializeRootCA runs the initialization logic for a root CA. It should only
// be called while the state lock is held by setting the state to non-ready.
func (c *CAManager) initializeRootCA(provider ca.Provider, conf *structs.CAConfiguration) error {
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
	if err := provider.GenerateRoot(); err != nil {
		return fmt.Errorf("error generating CA root certificate: %v", err)
	}

	// Get the active root cert from the CA
	rootPEM, err := provider.ActiveRoot()
	if err != nil {
		return fmt.Errorf("error getting root cert: %v", err)
	}
	rootCA, err := parseCARoot(rootPEM, conf.Provider, conf.ClusterID)
	if err != nil {
		return err
	}

	// Also create the intermediate CA, which is the one that actually signs leaf certs
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
		if _, err = c.delegate.raftApply(structs.ConnectCARequestType, req); err != nil {
			return fmt.Errorf("error persisting provider state: %v", err)
		}
	}

	// Versions prior to 1.9.2, 1.8.8, and 1.7.12 incorrectly used the primary
	// rootCA's subjectKeyID here instead of the intermediate. For
	// provider=consul this didn't matter since there are no intermediates in
	// the primaryDC, but for vault it does matter.
	expectedSigningKeyID := connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)
	needsSigningKeyUpdate := (rootCA.SigningKeyID != expectedSigningKeyID)

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
	if activeRoot != nil && needsSigningKeyUpdate {
		c.logger.Info("Correcting stored SigningKeyID value", "previous", rootCA.SigningKeyID, "updated", expectedSigningKeyID)

	} else if activeRoot != nil && !needsSigningKeyUpdate {
		// This state shouldn't be possible to get into because we update the root and
		// CA config in the same FSM operation.
		if activeRoot.ID != rootCA.ID {
			return fmt.Errorf("stored CA root %q is not the active root (%s)", rootCA.ID, activeRoot.ID)
		}

		rootCA.IntermediateCerts = activeRoot.IntermediateCerts
		c.setCAProvider(provider, rootCA)

		return nil
	}

	if needsSigningKeyUpdate {
		rootCA.SigningKeyID = expectedSigningKeyID
	}

	// Get the highest index
	idx, _, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Store the root cert in raft
	resp, err := c.delegate.raftApply(structs.ConnectCARequestType, &structs.CARequest{
		Op:    structs.CAOpSetRoots,
		Index: idx,
		Roots: []*structs.CARoot{rootCA},
	})
	if err != nil {
		c.logger.Error("Raft apply failed", "error", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	c.setCAProvider(provider, rootCA)

	c.logger.Info("initialized primary datacenter CA with provider", "provider", conf.Provider)

	return nil
}

// initializeSecondaryCA runs the routine for generating an intermediate CA CSR and getting
// it signed by the primary DC if the root CA of the primary DC has changed since the last
// intermediate. It should only be called while the state lock is held by setting the state
// to non-ready.
func (c *CAManager) initializeSecondaryCA(provider ca.Provider, config *structs.CAConfiguration) error {
	activeIntermediate, err := provider.ActiveIntermediate()
	if err != nil {
		return err
	}

	var (
		storedRootID         string
		expectedSigningKeyID string
		currentSigningKeyID  string
		activeSecondaryRoot  *structs.CARoot
	)
	if activeIntermediate != "" {
		// In the event that we already have an intermediate, we must have
		// already replicated some primary root information locally, so check
		// to see if we're up to date by fetching the rootID and the
		// signingKeyID used in the secondary.
		//
		// Note that for the same rootID the primary representation of the root
		// will have a different SigningKeyID field than the secondary
		// representation of the same root. This is because it's derived from
		// the intermediate which is different in all datacenters.
		storedRoot, err := provider.ActiveRoot()
		if err != nil {
			return err
		}

		storedRootID, err = connect.CalculateCertFingerprint(storedRoot)
		if err != nil {
			return fmt.Errorf("error parsing root fingerprint: %v, %#v", err, storedRoot)
		}

		intermediateCert, err := connect.ParseCert(activeIntermediate)
		if err != nil {
			return fmt.Errorf("error parsing active intermediate cert: %v", err)
		}
		expectedSigningKeyID = connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)

		// This will fetch the secondary's exact current representation of the
		// active root. Note that this data should only be used if the IDs
		// match, otherwise it's out of date and should be regenerated.
		_, activeSecondaryRoot, err = c.delegate.State().CARootActive(nil)
		if err != nil {
			return err
		}
		if activeSecondaryRoot != nil {
			currentSigningKeyID = activeSecondaryRoot.SigningKeyID
		}
	}

	// Determine which of the provided PRIMARY representations of roots is the
	// active one. We'll use this as a template to generate any new root
	// representations meant for this secondary.
	var newActiveRoot *structs.CARoot
	primaryRoots := c.getPrimaryRoots()
	for _, root := range primaryRoots.Roots {
		if root.ID == primaryRoots.ActiveRootID && root.Active {
			newActiveRoot = root
			break
		}
	}
	if newActiveRoot == nil {
		return fmt.Errorf("primary datacenter does not have an active root CA for Connect")
	}

	// Get a signed intermediate from the primary DC if the provider
	// hasn't been initialized yet or if the primary's root has changed.
	needsNewIntermediate := false
	if activeIntermediate == "" || storedRootID != primaryRoots.ActiveRootID {
		needsNewIntermediate = true
	}

	// Also we take this opportunity to correct an incorrectly persisted SigningKeyID
	// in secondary datacenters (see PR-6513).
	if expectedSigningKeyID != "" && currentSigningKeyID != expectedSigningKeyID {
		needsNewIntermediate = true
	}

	newIntermediate := false
	if needsNewIntermediate {
		if err := c.getIntermediateCASigned(provider, newActiveRoot); err != nil {
			return err
		}
		newIntermediate = true
	} else {
		// Discard the primary's representation since our local one is
		// sufficiently up to date.
		newActiveRoot = activeSecondaryRoot
	}

	// Update the roots list in the state store if there's a new active root.
	state := c.delegate.State()
	_, activeRoot, err := state.CARootActive(nil)
	if err != nil {
		return err
	}

	// Determine whether a root update is needed, and persist the roots/config accordingly.
	var newRoot *structs.CARoot
	if activeRoot == nil || activeRoot.ID != newActiveRoot.ID || newIntermediate {
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
	if newActiveRoot == nil && config != nil && config.Provider == storedConfig.Provider && reflect.DeepEqual(config.Config, storedConfig.Config) {
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
			newRoot.RotatedOutAt = time.Now()
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
	resp, err := c.delegate.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	if respOk, ok := resp.(bool); ok && !respOk {
		return fmt.Errorf("could not atomically update roots and config")
	}

	c.logger.Info("updated root certificates from primary datacenter")
	return nil
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

	// Attempt to initialize the config if we failed to do so in InitializeCA for some reason
	_, err = c.initializeCAConfig()
	if err != nil {
		return err
	}

	// Exit early if it's a no-op change
	state := c.delegate.State()
	confIdx, config, err := state.CAConfig(nil)
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
	newProvider, err := c.delegate.createCAProvider(args.Config)
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
	if err := newProvider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}

	// Set up a defer to clean up the new provider if we exit early due to an error.
	cleanupNewProvider := true
	defer func() {
		if cleanupNewProvider {
			if err := newProvider.Cleanup(args.Config.Provider != config.Provider, args.Config.Config); err != nil {
				c.logger.Warn("failed to clean up CA provider while handling startup failure", "provider", newProvider, "error", err)
			}
		}
	}()

	// If this is a secondary, just check if the intermediate needs to be regenerated.
	if c.serverConf.Datacenter != c.serverConf.PrimaryDatacenter {
		if err := c.initializeSecondaryCA(newProvider, args.Config); err != nil {
			return fmt.Errorf("Error updating secondary datacenter CA config: %v", err)
		}
		cleanupNewProvider = false
		c.logger.Info("Secondary CA provider config updated")
		return nil
	}

	if err := newProvider.GenerateRoot(); err != nil {
		return fmt.Errorf("error generating CA root certificate: %v", err)
	}

	newRootPEM, err := newProvider.ActiveRoot()
	if err != nil {
		return err
	}

	newActiveRoot, err := parseCARoot(newRootPEM, args.Config.Provider, args.Config.ClusterID)
	if err != nil {
		return err
	}

	// See if the provider needs to persist any state along with the config
	pState, err := newProvider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}
	args.Config.State = pState

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
		resp, err := c.delegate.raftApply(structs.ConnectCARequestType, args)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}

		// If the config has been committed, update the local provider instance
		cleanupNewProvider = false
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
		if !canXSign && args.Config.ForceWithoutCrossSigning {
			c.logger.Warn("current CA doesn't support cross signing but " +
				"CA reconfiguration forced anyway with ForceWithoutCrossSigning")
		}

		if canXSign {
			// Have the old provider cross-sign the new root
			xcCert, err := oldProvider.CrossSignCA(newRoot)
			if err != nil {
				return err
			}

			// Add the cross signed cert to the new CA's intermediates (to be attached
			// to leaf certs).
			newActiveRoot.IntermediateCerts = []string{xcCert}
		}
	}

	intermediate, err := newProvider.GenerateIntermediate()
	if err != nil {
		return err
	}
	if intermediate != newRootPEM {
		newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediate)
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
			newRoot.RotatedOutAt = time.Now()
		}
		newRoots = append(newRoots, &newRoot)
	}
	newRoots = append(newRoots, newActiveRoot)

	args.Op = structs.CAOpSetRootsAndConfig
	args.Index = idx
	args.Config.ModifyIndex = confIdx
	args.Roots = newRoots
	resp, err := c.delegate.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}
	if respOk, ok := resp.(bool); ok && !respOk {
		return fmt.Errorf("could not atomically update roots and config")
	}

	// If the config has been committed, update the local provider instance
	// and call teardown on the old provider
	cleanupNewProvider = false
	c.setCAProvider(newProvider, newActiveRoot)

	if err := oldProvider.Cleanup(args.Config.Provider != config.Provider, args.Config.Config); err != nil {
		c.logger.Warn("failed to clean up old provider", "provider", config.Provider, "error", err)
	}

	c.logger.Info("CA rotated to new root under provider", "provider", args.Config.Provider)

	return nil
}

// getIntermediateCAPrimary regenerates the intermediate cert in the primary datacenter.
// This is only run for CAs that require an intermediary in the primary DC, such as Vault.
//  It should only be called while the state lock is held by setting the state to non-ready.
func (c *CAManager) getIntermediateCAPrimary(provider ca.Provider, newActiveRoot *structs.CARoot) error {
	// Generate and sign an intermediate cert using the root CA.
	intermediatePEM, err := provider.GenerateIntermediate()
	if err != nil {
		return fmt.Errorf("error generating new intermediate cert: %v", err)
	}

	intermediateCert, err := connect.ParseCert(intermediatePEM)
	if err != nil {
		return fmt.Errorf("error parsing intermediate cert: %v", err)
	}

	// Append the new intermediate to our local active root entry. This is
	// where the root representations start to diverge.
	newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediatePEM)
	newActiveRoot.SigningKeyID = connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)

	c.logger.Info("generated new intermediate certificate for primary datacenter")
	return nil
}

// getIntermediateCASigned should only be called while the state lock is held by
// setting the state to non-ready.
func (c *CAManager) getIntermediateCASigned(provider ca.Provider, newActiveRoot *structs.CARoot) error {
	csr, err := provider.GenerateIntermediateCSR()
	if err != nil {
		return err
	}

	var intermediatePEM string
	if err := c.delegate.forwardDC("ConnectCA.SignIntermediate", c.serverConf.PrimaryDatacenter, c.delegate.generateCASignRequest(csr), &intermediatePEM); err != nil {
		// this is a failure in the primary and shouldn't be capable of erroring out our establishing leadership
		c.logger.Warn("Primary datacenter refused to sign our intermediate CA certificate", "error", err)
		return nil
	}

	if err := provider.SetIntermediate(intermediatePEM, newActiveRoot.RootCert); err != nil {
		return fmt.Errorf("Failed to set the intermediate certificate with the CA provider: %v", err)
	}

	intermediateCert, err := connect.ParseCert(intermediatePEM)
	if err != nil {
		return fmt.Errorf("error parsing intermediate cert: %v", err)
	}

	// Append the new intermediate to our local active root entry. This is
	// where the root representations start to diverge.
	newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediatePEM)
	newActiveRoot.SigningKeyID = connect.EncodeSigningKeyID(intermediateCert.SubjectKeyId)

	c.logger.Info("received new intermediate certificate from primary datacenter")
	return nil
}

// intermediateCertRenewalWatch periodically attempts to renew the intermediate cert.
func (c *CAManager) intermediateCertRenewalWatch(ctx context.Context) error {
	isPrimary := c.serverConf.Datacenter == c.serverConf.PrimaryDatacenter

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(structs.IntermediateCertRenewInterval):
			retryLoopBackoffAbortOnSuccess(ctx, func() error {
				return c.RenewIntermediate(ctx, isPrimary)
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
func (c *CAManager) RenewIntermediate(ctx context.Context, isPrimary bool) error {
	// Grab the 'lock' right away so the provider/config can't be changed out while we check
	// the intermediate.
	if _, err := c.setState(caStateRenewIntermediate, true); err != nil {
		return err
	}
	defer c.setState(caStateInitialized, false)

	provider, _ := c.getCAProvider()
	if provider == nil {
		// this happens when leadership is being revoked and this go routine will be stopped
		return nil
	}
	// If this isn't the primary, make sure the CA has been initialized.
	if !isPrimary && !c.configuredSecondaryCA() {
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
	if isPrimary {
		_, config, err := state.CAConfig(nil)
		if err != nil {
			return err
		}

		if _, ok := ca.PrimaryIntermediateProviders[config.Provider]; !ok {
			return nil
		}
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

	if lessThanHalfTimePassed(time.Now(), intermediateCert.NotBefore.Add(ca.CertificateTimeDriftBuffer),
		intermediateCert.NotAfter) {
		return nil
	}

	// Enough time has passed, go ahead with getting a new intermediate.
	renewalFunc := c.getIntermediateCAPrimary
	if !isPrimary {
		renewalFunc = c.getIntermediateCASigned
	}
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
		if err := c.UpdateRoots(roots); err != nil {
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

// UpdateRoots updates the cached roots from the primary and regenerates the intermediate
// certificate if necessary.
func (c *CAManager) UpdateRoots(roots structs.IndexedCARoots) error {
	// Update the state first to claim the 'lock'.
	if _, err := c.setState(caStateReconfig, true); err != nil {
		return err
	}
	defer c.setState(caStateInitialized, false)

	// Update the cached primary roots now that the lock is held.
	if err := c.setPrimaryRoots(roots); err != nil {
		return err
	}

	// Check to see if the primary has been upgraded in case we're waiting to switch to
	// secondary mode.
	provider, _ := c.getCAProvider()
	if provider == nil {
		// this happens when leadership is being revoked and this go routine will be stopped
		return nil
	}
	if !c.configuredSecondaryCA() {
		versionOk, primaryFound := ServersInDCMeetMinimumVersion(c.delegate, c.serverConf.PrimaryDatacenter, minMultiDCConnectVersion)
		if !primaryFound {
			return fmt.Errorf("Primary datacenter is unreachable - deferring secondary CA initialization")
		}

		if versionOk {
			if err := c.initializeSecondaryProvider(provider, roots); err != nil {
				return fmt.Errorf("Failed to initialize secondary CA provider: %v", err)
			}
		}
	}

	// Run the secondary CA init routine to see if we need to request a new
	// intermediate.
	if c.configuredSecondaryCA() {
		if err := c.initializeSecondaryCA(provider, nil); err != nil {
			return fmt.Errorf("Failed to initialize the secondary CA: %v", err)
		}
	}

	return nil
}

// initializeSecondaryProvider configures the given provider for a secondary, non-root datacenter.
func (c *CAManager) initializeSecondaryProvider(provider ca.Provider, roots structs.IndexedCARoots) error {
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

	return c.setSecondaryCA()
}

// setSecondaryCA sets the flag for acting as a secondary CA to true.
func (c *CAManager) setSecondaryCA() error {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()

	if c.state == caStateInitializing || c.state == caStateReconfig {
		c.actingSecondaryCA = true
	} else {
		return fmt.Errorf("Cannot update secondary CA flag in state %q", c.state)
	}

	return nil
}

// configuredSecondaryCA returns true if we have been initialized as a secondary datacenter's CA.
func (c *CAManager) configuredSecondaryCA() bool {
	c.stateLock.Lock()
	defer c.stateLock.Unlock()
	return c.actingSecondaryCA
}
