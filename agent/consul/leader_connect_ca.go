package consul

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/logging"
	"github.com/hashicorp/go-hclog"
	uuid "github.com/hashicorp/go-uuid"
)

type CAState string

const (
	CAStateUninitialized     CAState = "UNINITIALIZED"
	CAStateInitializing              = "INITIALIZING"
	CAStateReady                     = "READY"
	CAStateRenewIntermediate         = "RENEWING"
	CAStateReconfig                  = "RECONFIGURING"
)

// CAManager is a wrapper around CA operations such as updating roots, an intermediate
// or the configuration. All operations should go through the CAManager in order to
// avoid data races.
type CAManager struct {
	srv    *Server
	logger hclog.Logger

	// provider is the current CA provider in use for Connect. This is
	// only non-nil when we are the leader.
	provider ca.Provider

	// providerRoot is the CARoot that was stored along with the ca.Provider
	// active. It's only updated in lock-step with the provider. This prevents
	// races between state updates to active roots and the fetch of the provider
	// instance.
	providerRoot *structs.CARoot
	providerLock sync.RWMutex

	// primaryRoots is the most recently seen state of the root CAs from the primary datacenter.
	// This is protected by the stateLock and updated by initializeCA and the root CA watch routine.
	primaryRoots structs.IndexedCARoots

	// actingSecondaryCA is whether this datacenter has been initialized as a secondary CA.
	actingSecondaryCA bool
	state             CAState
	stateLock         sync.RWMutex
}

func NewCAManager(srv *Server) *CAManager {
	return &CAManager{
		srv:    srv,
		logger: srv.loggers.Named(logging.Connect),
		state:  CAStateUninitialized,
	}
}

// setState attempts to update the CA state to the given state.
// If the current state is not READY, this will fail. The only exception is when
// the current state is UNINITIALIZED, and the function is called with CAStateInitializing.
func (c *CAManager) setState(newState CAState) error {
	c.stateLock.RLock()
	state := c.state
	c.stateLock.RUnlock()

	if state == CAStateReady || (state == CAStateUninitialized && newState == CAStateInitializing) {
		c.stateLock.Lock()
		c.state = newState
		c.stateLock.Unlock()
	} else {
		return fmt.Errorf("CA is already in %s state", state)
	}
	return nil
}

// setReady sets the CA state back to READY. This should only be called by a function
// that has successfully called setState beforehand.
func (c *CAManager) setReady() {
	c.stateLock.Lock()
	c.state = CAStateReady
	c.stateLock.Unlock()
}

// initializeCAConfig is used to initialize the CA config if necessary
// when setting up the CA during establishLeadership
func (c *CAManager) initializeCAConfig() (*structs.CAConfiguration, error) {
	state := c.srv.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	if config == nil {
		config = c.srv.config.CAConfig
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
	if resp, err := c.srv.raftApply(structs.ConnectCARequestType, req); err != nil {
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

// getCAProvider is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
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
		if result == nil && c.srv.IsLeader() && retries < 10 {
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

// initializeCA sets up the CA provider when gaining leadership, either bootstrapping
// the CA if this is the primary DC or making a remote RPC for intermediate signing
// if this is a secondary DC.
func (c *CAManager) initializeCA() error {
	// Bail if connect isn't enabled.
	if !c.srv.config.ConnectEnabled {
		return nil
	}

	err := c.setState(CAStateInitializing)
	if err != nil {
		return err
	}
	defer c.setReady()

	// Initialize the provider based on the current config.
	conf, err := c.initializeCAConfig()
	if err != nil {
		return err
	}
	provider, err := c.srv.createCAProvider(conf)
	if err != nil {
		return err
	}

	c.setCAProvider(provider, nil)

	// If this isn't the primary DC, run the secondary DC routine if the primary has already been upgraded to at least 1.6.0
	if c.srv.config.PrimaryDatacenter != c.srv.config.Datacenter {
		versionOk, foundPrimary := ServersInDCMeetMinimumVersion(c.srv, c.srv.config.PrimaryDatacenter, minMultiDCConnectVersion)
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
			Datacenter: c.srv.config.PrimaryDatacenter,
		}
		var roots structs.IndexedCARoots
		if err := c.srv.forwardDC("ConnectCA.Roots", c.srv.config.PrimaryDatacenter, &args, &roots); err != nil {
			return err
		}
		c.primaryRoots = roots

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

	return c.initializeRootCA(provider, conf)
}

// initializeRootCA runs the initialization logic for a root CA.
// It is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
func (c *CAManager) initializeRootCA(provider ca.Provider, conf *structs.CAConfiguration) error {
	pCfg := ca.ProviderConfig{
		ClusterID:  conf.ClusterID,
		Datacenter: c.srv.config.Datacenter,
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
	_, err = connect.ParseCert(interPEM)
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
		if _, err = c.srv.raftApply(structs.ConnectCARequestType, req); err != nil {
			return fmt.Errorf("error persisting provider state: %v", err)
		}
	}

	// Check if the CA root is already initialized and exit if it is,
	// adding on any existing intermediate certs since they aren't directly
	// tied to the provider.
	// Every change to the CA after this initial bootstrapping should
	// be done through the rotation process.
	state := c.srv.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	if activeRoot != nil {
		// This state shouldn't be possible to get into because we update the root and
		// CA config in the same FSM operation.
		if activeRoot.ID != rootCA.ID {
			return fmt.Errorf("stored CA root %q is not the active root (%s)", rootCA.ID, activeRoot.ID)
		}

		rootCA.IntermediateCerts = activeRoot.IntermediateCerts
		c.setCAProvider(provider, rootCA)

		return nil
	}

	// Get the highest index
	idx, _, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Store the root cert in raft
	resp, err := c.srv.raftApply(structs.ConnectCARequestType, &structs.CARequest{
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
// intermediate.
// It is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
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
		_, activeSecondaryRoot, err = c.srv.fsm.State().CARootActive(nil)
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
	for _, root := range c.primaryRoots.Roots {
		if root.ID == c.primaryRoots.ActiveRootID && root.Active {
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
	if activeIntermediate == "" || storedRootID != c.primaryRoots.ActiveRootID {
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
	state := c.srv.fsm.State()
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

// persistNewRootAndConfig is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
// If newActiveRoot is non-nil, it will be appended to the current roots list.
// If config is non-nil, it will be used to overwrite the existing config.
func (c *CAManager) persistNewRootAndConfig(provider ca.Provider, newActiveRoot *structs.CARoot, config *structs.CAConfiguration) error {
	state := c.srv.fsm.State()
	idx, oldRoots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	var newConf structs.CAConfiguration
	_, storedConfig, err := state.CAConfig(nil)
	if err != nil {
		return err
	}
	if storedConfig == nil {
		return fmt.Errorf("local CA not initialized yet")
	}
	if config != nil {
		newConf = *config
	} else {
		newConf = *storedConfig
	}
	newConf.ModifyIndex = storedConfig.ModifyIndex
	if newActiveRoot != nil {
		newConf.ClusterID = newActiveRoot.ExternalTrustDomain
	}

	// Persist any state the provider needs us to
	newConf.State, err = provider.State()
	if err != nil {
		return fmt.Errorf("error getting provider state: %v", err)
	}

	// If there's a new active root, copy the root list and append it, updating
	// the old root with the time it was rotated out.
	var newRoots structs.CARoots
	if newActiveRoot != nil {
		for _, r := range oldRoots {
			newRoot := *r
			if newRoot.Active {
				newRoot.Active = false
				newRoot.RotatedOutAt = time.Now()
			}
			if newRoot.ExternalTrustDomain == "" {
				newRoot.ExternalTrustDomain = config.ClusterID
			}
			newRoots = append(newRoots, &newRoot)
		}
		newRoots = append(newRoots, newActiveRoot)
	} else {
		newRoots = oldRoots
	}

	args := &structs.CARequest{
		Op:     structs.CAOpSetRootsAndConfig,
		Index:  idx,
		Roots:  newRoots,
		Config: &newConf,
	}
	resp, err := c.srv.raftApply(structs.ConnectCARequestType, &args)
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

// getIntermediateCAPrimary regenerates the intermediate cert in the primary datacenter.
// This is only run for CAs that require an intermediary in the primary DC, such as Vault.
// This function is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
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

// getIntermediateCASigned is being called while holding caProviderReconfigurationLock
// which means it must never take that lock itself or call anything that does.
func (c *CAManager) getIntermediateCASigned(provider ca.Provider, newActiveRoot *structs.CARoot) error {
	csr, err := provider.GenerateIntermediateCSR()
	if err != nil {
		return err
	}

	var intermediatePEM string
	if err := c.srv.forwardDC("ConnectCA.SignIntermediate", c.srv.config.PrimaryDatacenter, c.srv.generateCASignRequest(csr), &intermediatePEM); err != nil {
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

// intermediateCertRenewalWatch checks the intermediate cert for
// expiration. As soon as more than half the time a cert is valid has passed,
// it will try to renew it.
func (c *CAManager) intermediateCertRenewalWatch(ctx context.Context) error {
	isPrimary := c.srv.config.Datacenter == c.srv.config.PrimaryDatacenter

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(structs.IntermediateCertRenewInterval):
			retryLoopBackoffAbortOnSuccess(ctx, func() error {
				if !isPrimary {
					c.logger.Info("starting check for intermediate renewal")
				}
				// Grab the 'lock' right away so the provider/config can't be changed out while we check
				// the intermediate.
				if err := c.setState(CAStateRenewIntermediate); err != nil {
					return err
				}
				defer c.setReady()

				provider, _ := c.getCAProvider()
				if provider == nil {
					// this happens when leadership is being revoked and this go routine will be stopped
					return nil
				}
				// If this isn't the primary, make sure the CA has been initialized.
				if !isPrimary && !c.configuredSecondaryCA() {
					return fmt.Errorf("secondary CA is not yet configured.")
				}

				state := c.srv.fsm.State()
				_, activeRoot, err := state.CARootActive(nil)
				if err != nil {
					return err
				}

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
				} else {
					c.logger.Info("Checking for intermediate renewal")
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
				if err := renewalFunc(provider, activeRoot); err != nil {
					return err
				}

				if err := c.persistNewRootAndConfig(provider, activeRoot, nil); err != nil {
					return err
				}

				c.setCAProvider(provider, activeRoot)
				return nil
			}, func(err error) {
				c.logger.Error("error renewing intermediate certs",
					"routine", intermediateCertRenewWatchRoutineName,
					"error", err,
				)
			})
		}
	}
}

// secondaryCARootWatch maintains a blocking query to the primary datacenter's
// ConnectCA.Roots endpoint to monitor when it needs to request a new signed
// intermediate certificate.
func (c *CAManager) secondaryCARootWatch(ctx context.Context) error {
	args := structs.DCSpecificRequest{
		Datacenter: c.srv.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			// the maximum time the primary roots watch query can block before returning
			MaxQueryTime: c.srv.config.MaxQueryTime,
		},
	}

	c.logger.Debug("starting Connect CA root replication from primary datacenter", "primary", c.srv.config.PrimaryDatacenter)

	retryLoopBackoff(ctx, func() error {
		var roots structs.IndexedCARoots
		if err := c.srv.forwardDC("ConnectCA.Roots", c.srv.config.PrimaryDatacenter, &args, &roots); err != nil {
			return fmt.Errorf("Error retrieving the primary datacenter's roots: %v", err)
		}

		// Update the state first to claim the 'lock'.
		if err := c.setState(CAStateReconfig); err != nil {
			return err
		}
		defer c.setReady()

		// Update the cached primary roots now that the lock is held.
		c.primaryRoots = roots

		// Check to see if the primary has been upgraded in case we're waiting to switch to
		// secondary mode.
		provider, _ := c.getCAProvider()
		if provider == nil {
			// this happens when leadership is being revoked and this go routine will be stopped
			return nil
		}
		if !c.configuredSecondaryCA() {
			versionOk, primaryFound := ServersInDCMeetMinimumVersion(c.srv, c.srv.config.PrimaryDatacenter, minMultiDCConnectVersion)
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

// initializeSecondaryProvider configures the given provider for a secondary, non-root datacenter.
// It is being called while holding the stateLock in order to update actingSecondaryCA, which means
// it must never take that lock itself or call anything that does.
func (c *CAManager) initializeSecondaryProvider(provider ca.Provider, roots structs.IndexedCARoots) error {
	if roots.TrustDomain == "" {
		return fmt.Errorf("trust domain from primary datacenter is not initialized")
	}

	clusterID := strings.Split(roots.TrustDomain, ".")[0]
	_, conf, err := c.srv.fsm.State().CAConfig(nil)
	if err != nil {
		return err
	}

	pCfg := ca.ProviderConfig{
		ClusterID:  clusterID,
		Datacenter: c.srv.config.Datacenter,
		IsPrimary:  false,
		RawConfig:  conf.Config,
		State:      conf.State,
	}
	if err := provider.Configure(pCfg); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}

	c.actingSecondaryCA = true

	return nil
}

// configuredSecondaryCA returns true if we have been initialized as a secondary datacenter's CA.
func (c *CAManager) configuredSecondaryCA() bool {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.actingSecondaryCA
}
