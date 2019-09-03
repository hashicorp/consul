package consul

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/structs"
	uuid "github.com/hashicorp/go-uuid"
)

const (
	// loopRateLimit is the maximum rate per second at which we can rerun CA and intention
	// replication watches.
	loopRateLimit rate.Limit = 0.2

	// retryBucketSize is the maximum number of stored rate limit attempts for looped
	// blocking query operations.
	retryBucketSize = 5

	// maxIntentionTxnSize is the maximum size (in bytes) of a transaction used during
	// Intention replication.
	maxIntentionTxnSize = raftWarnSize / 4
)

var (
	// maxRetryBackoff is the maximum number of seconds to wait between failed blocking
	// queries when backing off.
	maxRetryBackoff = 256

	// maxRootsQueryTime is the maximum time the primary roots watch query can block before
	// returning.
	maxRootsQueryTime = maxQueryTime
)

// initializeCAConfig is used to initialize the CA config if necessary
// when setting up the CA during establishLeadership
func (s *Server) initializeCAConfig() (*structs.CAConfiguration, error) {
	state := s.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	if config != nil {
		return config, nil
	}

	config = s.config.CAConfig
	if config.ClusterID == "" {
		id, err := uuid.GenerateUUID()
		if err != nil {
			return nil, err
		}
		config.ClusterID = id
	}

	req := structs.CARequest{
		Op:     structs.CAOpSetConfig,
		Config: config,
	}
	if _, err = s.raftApply(structs.ConnectCARequestType, req); err != nil {
		return nil, err
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
	return &structs.CARoot{
		ID:                  id,
		Name:                fmt.Sprintf("%s CA Root Cert", strings.Title(provider)),
		SerialNumber:        rootCert.SerialNumber.Uint64(),
		SigningKeyID:        connect.HexString(rootCert.SubjectKeyId),
		ExternalTrustDomain: clusterID,
		NotBefore:           rootCert.NotBefore,
		NotAfter:            rootCert.NotAfter,
		RootCert:            pemValue,
		Active:              true,
	}, nil
}

// createProvider returns a connect CA provider from the given config.
func (s *Server) createCAProvider(conf *structs.CAConfiguration) (ca.Provider, error) {
	switch conf.Provider {
	case structs.ConsulCAProvider:
		return &ca.ConsulProvider{Delegate: &consulCADelegate{s}}, nil
	case structs.VaultCAProvider:
		return &ca.VaultProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown CA provider %q", conf.Provider)
	}
}

func (s *Server) getCAProvider() (ca.Provider, *structs.CARoot) {
	retries := 0
	var result ca.Provider
	var resultRoot *structs.CARoot
	for result == nil {
		s.caProviderLock.RLock()
		result = s.caProvider
		resultRoot = s.caProviderRoot
		s.caProviderLock.RUnlock()

		// In cases where an agent is started with managed proxies, we may ask
		// for the provider before establishLeadership completes. If we're the
		// leader, then wait and get the provider again
		if result == nil && s.IsLeader() && retries < 10 {
			retries++
			time.Sleep(50 * time.Millisecond)
			continue
		}

		break
	}

	return result, resultRoot
}

func (s *Server) setCAProvider(newProvider ca.Provider, root *structs.CARoot) {
	s.caProviderLock.Lock()
	defer s.caProviderLock.Unlock()
	s.caProvider = newProvider
	s.caProviderRoot = root
}

// initializeCA sets up the CA provider when gaining leadership, either bootstrapping
// the CA if this is the primary DC or making a remote RPC for intermediate signing
// if this is a secondary DC.
func (s *Server) initializeCA() error {
	// Bail if connect isn't enabled.
	if !s.config.ConnectEnabled {
		return nil
	}

	// Initialize the provider based on the current config.
	conf, err := s.initializeCAConfig()
	if err != nil {
		return err
	}
	provider, err := s.createCAProvider(conf)
	if err != nil {
		return err
	}
	s.setCAProvider(provider, nil)

	// If this isn't the primary DC, run the secondary DC routine if the primary has already been upgraded to at least 1.6.0
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		versionOk, foundPrimary := ServersInDCMeetMinimumVersion(s.WANMembers(), s.config.PrimaryDatacenter, minMultiDCConnectVersion)
		if !foundPrimary {
			s.logger.Printf("[WARN] connect: primary datacenter is configured but unreachable - deferring initialization of the secondary datacenter CA")
			// return nil because we will initialize the secondary CA later
			return nil
		} else if !versionOk {
			// return nil because we will initialize the secondary CA later
			s.logger.Printf("[WARN] connect: servers in the primary datacenter are not at least at version %s - deferring initialization of the secondary datacenter CA", minMultiDCConnectVersion)
			return nil
		}

		// Get the root CA to see if we need to refresh our intermediate.
		args := structs.DCSpecificRequest{
			Datacenter: s.config.PrimaryDatacenter,
		}
		var roots structs.IndexedCARoots
		if err := s.forwardDC("ConnectCA.Roots", s.config.PrimaryDatacenter, &args, &roots); err != nil {
			return err
		}

		// Configure the CA provider and initialize the intermediate certificate if necessary.
		if err := s.initializeSecondaryProvider(provider, roots); err != nil {
			return fmt.Errorf("error configuring provider: %v", err)
		}
		if err := s.initializeSecondaryCA(provider, roots); err != nil {
			return err
		}

		s.logger.Printf("[INFO] connect: initialized secondary datacenter CA with provider %q", conf.Provider)
		return nil
	}

	return s.initializeRootCA(provider, conf)
}

// initializeRootCA runs the initialization logic for a root CA.
func (s *Server) initializeRootCA(provider ca.Provider, conf *structs.CAConfiguration) error {
	if err := provider.Configure(conf.ClusterID, true, conf.Config); err != nil {
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

	commonConfig, err := conf.GetCommonConfig()
	if err != nil {
		return err
	}
	rootCA.PrivateKeyType = commonConfig.PrivateKeyType
	rootCA.PrivateKeyBits = commonConfig.PrivateKeyBits

	// Check if the CA root is already initialized and exit if it is,
	// adding on any existing intermediate certs since they aren't directly
	// tied to the provider.
	// Every change to the CA after this initial bootstrapping should
	// be done through the rotation process.
	state := s.fsm.State()
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
		s.setCAProvider(provider, rootCA)

		return nil
	}

	// Get the highest index
	idx, _, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	// Store the root cert in raft
	resp, err := s.raftApply(structs.ConnectCARequestType, &structs.CARequest{
		Op:    structs.CAOpSetRoots,
		Index: idx,
		Roots: []*structs.CARoot{rootCA},
	})
	if err != nil {
		s.logger.Printf("[ERR] connect: Apply failed %v", err)
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	s.setCAProvider(provider, rootCA)

	s.logger.Printf("[INFO] connect: initialized primary datacenter CA with provider %q", conf.Provider)

	return nil
}

// initializeSecondaryCA runs the routine for generating an intermediate CA CSR and getting
// it signed by the primary DC if the root CA of the primary DC has changed since the last
// intermediate.
func (s *Server) initializeSecondaryCA(provider ca.Provider, roots structs.IndexedCARoots) error {
	activeIntermediate, err := provider.ActiveIntermediate()
	if err != nil {
		return err
	}

	var storedRootID string
	if activeIntermediate != "" {
		storedRoot, err := provider.ActiveRoot()
		if err != nil {
			return err
		}

		storedRootID, err = connect.CalculateCertFingerprint(storedRoot)
		if err != nil {
			return fmt.Errorf("error parsing root fingerprint: %v, %#v", err, roots)
		}
	}

	var newActiveRoot *structs.CARoot
	for _, root := range roots.Roots {
		if root.ID == roots.ActiveRootID && root.Active {
			newActiveRoot = root
			break
		}
	}
	if newActiveRoot == nil {
		return fmt.Errorf("primary datacenter does not have an active root CA for Connect")
	}

	newIntermediate := false
	// Get a signed intermediate from the primary DC if the provider
	// hasn't been initialized yet or if the primary's root has changed.
	if activeIntermediate == "" || storedRootID != roots.ActiveRootID {
		csr, err := provider.GenerateIntermediateCSR()
		if err != nil {
			return err
		}

		var intermediatePEM string
		if err := s.forwardDC("ConnectCA.SignIntermediate", s.config.PrimaryDatacenter, s.generateCASignRequest(csr), &intermediatePEM); err != nil {
			// this is a failure in the primary and shouldn't be capable of erroring out our establishing leadership
			s.logger.Printf("[WARN] connect: Primary datacenter refused to sign our intermediate CA certificate: %v", err)
			return nil
		}

		if err := provider.SetIntermediate(intermediatePEM, newActiveRoot.RootCert); err != nil {
			return fmt.Errorf("Failed to set the intermediate certificate with the CA provider: %v", err)
		}

		// Append the new intermediate to our local active root entry.
		newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediatePEM)
		newIntermediate = true

		s.logger.Printf("[INFO] connect: received new intermediate certificate from primary datacenter")
	}

	// Update the roots list in the state store if there's a new active root.
	state := s.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	if activeRoot == nil || activeRoot.ID != newActiveRoot.ID || newIntermediate {
		idx, oldRoots, err := state.CARoots(nil)
		if err != nil {
			return err
		}

		_, config, err := state.CAConfig(nil)
		if err != nil {
			return err
		}
		if config == nil {
			return fmt.Errorf("local CA not initialized yet")
		}
		newConf := *config
		newConf.ClusterID = newActiveRoot.ExternalTrustDomain

		// Copy the root list and append the new active root, updating the old root
		// with the time it was rotated out.
		var newRoots structs.CARoots
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

		args := &structs.CARequest{
			Op:     structs.CAOpSetRootsAndConfig,
			Index:  idx,
			Roots:  newRoots,
			Config: &newConf,
		}
		resp, err := s.raftApply(structs.ConnectCARequestType, &args)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}
		if respOk, ok := resp.(bool); ok && !respOk {
			return fmt.Errorf("could not atomically update roots and config")
		}

		s.logger.Printf("[INFO] connect: updated root certificates from primary datacenter")
	}

	s.setCAProvider(provider, newActiveRoot)
	return nil
}

func (s *Server) generateCASignRequest(csr string) *structs.CASignRequest {
	return &structs.CASignRequest{
		Datacenter:   s.config.PrimaryDatacenter,
		CSR:          csr,
		WriteRequest: structs.WriteRequest{Token: s.tokens.ReplicationToken()},
	}
}

// startConnectLeader starts multi-dc connect leader routines.
func (s *Server) startConnectLeader() {
	s.connectLock.Lock()
	defer s.connectLock.Unlock()

	if s.connectEnabled {
		return
	}

	s.connectCh = make(chan struct{})

	// Start the Connect secondary DC actions if enabled.
	if s.config.ConnectEnabled && s.config.Datacenter != s.config.PrimaryDatacenter {
		go s.secondaryCARootWatch(s.connectCh)
		go s.replicateIntentions(s.connectCh)

	}

	go s.runCARootPruning(s.connectCh)

	s.connectEnabled = true
}

// stopConnectLeader stops connect specific leader functions.
func (s *Server) stopConnectLeader() {
	s.connectLock.Lock()
	defer s.connectLock.Unlock()

	if !s.connectEnabled {
		return
	}

	s.actingSecondaryLock.Lock()
	s.actingSecondaryCA = false
	s.actingSecondaryLock.Unlock()

	close(s.connectCh)
	s.connectEnabled = false
}

func (s *Server) runCARootPruning(stopCh <-chan struct{}) {
	ticker := time.NewTicker(caRootPruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			if err := s.pruneCARoots(); err != nil {
				s.logger.Printf("[ERR] connect: error pruning CA roots: %v", err)
			}
		}
	}
}

// pruneCARoots looks for any CARoots that have been rotated out and expired.
func (s *Server) pruneCARoots() error {
	if !s.config.ConnectEnabled {
		return nil
	}

	state := s.fsm.State()
	idx, roots, err := state.CARoots(nil)
	if err != nil {
		return err
	}

	_, caConf, err := state.CAConfig(nil)
	if err != nil {
		return err
	}

	common, err := caConf.GetCommonConfig()
	if err != nil {
		return err
	}

	var newRoots structs.CARoots
	for _, r := range roots {
		if !r.Active && !r.RotatedOutAt.IsZero() && time.Now().Sub(r.RotatedOutAt) > common.LeafCertTTL*2 {
			s.logger.Printf("[INFO] connect: pruning old unused root CA (ID: %s)", r.ID)
			continue
		}
		newRoot := *r
		newRoots = append(newRoots, &newRoot)
	}

	// Return early if there's nothing to remove.
	if len(newRoots) == len(roots) {
		return nil
	}

	// Commit the new root state.
	var args structs.CARequest
	args.Op = structs.CAOpSetRoots
	args.Index = idx
	args.Roots = newRoots
	resp, err := s.raftApply(structs.ConnectCARequestType, args)
	if err != nil {
		return err
	}
	if respErr, ok := resp.(error); ok {
		return respErr
	}

	return nil
}

// secondaryCARootWatch maintains a blocking query to the primary datacenter's
// ConnectCA.Roots endpoint to monitor when it needs to request a new signed
// intermediate certificate.
func (s *Server) secondaryCARootWatch(stopCh <-chan struct{}) {
	args := structs.DCSpecificRequest{
		Datacenter: s.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{
			MaxQueryTime: maxRootsQueryTime,
		},
	}

	s.logger.Printf("[DEBUG] connect: starting Connect CA root replication from primary datacenter %q", s.config.PrimaryDatacenter)

	retryLoopBackoff(stopCh, func() error {
		var roots structs.IndexedCARoots
		if err := s.forwardDC("ConnectCA.Roots", s.config.PrimaryDatacenter, &args, &roots); err != nil {
			return fmt.Errorf("Error retrieving the primary datacenter's roots: %v", err)
		}

		// Check to see if the primary has been upgraded in case we're waiting to switch to
		// secondary mode.
		provider, _ := s.getCAProvider()
		if provider == nil {
			// this happens when leadership is being revoked and this go routine will be stopped
			return nil
		}
		if !s.configuredSecondaryCA() {
			versionOk, primaryFound := ServersInDCMeetMinimumVersion(s.WANMembers(), s.config.PrimaryDatacenter, minMultiDCConnectVersion)
			if !primaryFound {
				return fmt.Errorf("Primary datacenter is unreachable - deferring secondary CA initialization")
			}

			if versionOk {
				if err := s.initializeSecondaryProvider(provider, roots); err != nil {
					return fmt.Errorf("Failed to initialize secondary CA provider: %v", err)
				}
			}
		}

		// Run the secondary CA init routine to see if we need to request a new
		// intermediate.
		if s.configuredSecondaryCA() {
			if err := s.initializeSecondaryCA(provider, roots); err != nil {
				return fmt.Errorf("Failed to initialize the secondary CA: %v", err)
			}
		}

		args.QueryOptions.MinQueryIndex = nextIndexVal(args.QueryOptions.MinQueryIndex, roots.QueryMeta.Index)
		return nil
	}, func(err error) {
		s.logger.Printf("[ERR] connect: %v", err)
	})
}

// replicateIntentions executes a blocking query to the primary datacenter to replicate
// the intentions there to the local state.
func (s *Server) replicateIntentions(stopCh <-chan struct{}) {
	args := structs.DCSpecificRequest{
		Datacenter: s.config.PrimaryDatacenter,
	}

	s.logger.Printf("[DEBUG] connect: starting Connect intention replication from primary datacenter %q", s.config.PrimaryDatacenter)

	retryLoopBackoff(stopCh, func() error {
		// Always use the latest replication token value in case it changed while looping.
		args.QueryOptions.Token = s.tokens.ReplicationToken()

		var remote structs.IndexedIntentions
		if err := s.forwardDC("Intention.List", s.config.PrimaryDatacenter, &args, &remote); err != nil {
			return err
		}

		_, local, err := s.fsm.State().Intentions(nil)
		if err != nil {
			return err
		}

		// Compute the diff between the remote and local intentions.
		deletes, updates := diffIntentions(local, remote.Intentions)
		txnOpSets := batchIntentionUpdates(deletes, updates)

		// Apply batched updates to the state store.
		for _, ops := range txnOpSets {
			txnReq := structs.TxnRequest{Ops: ops}

			resp, err := s.raftApply(structs.TxnRequestType, &txnReq)
			if err != nil {
				return err
			}
			if respErr, ok := resp.(error); ok {
				return respErr
			}

			if txnResp, ok := resp.(structs.TxnResponse); ok {
				if len(txnResp.Errors) > 0 {
					return txnResp.Error()
				}
			} else {
				return fmt.Errorf("unexpected return type %T", resp)
			}
		}

		args.QueryOptions.MinQueryIndex = nextIndexVal(args.QueryOptions.MinQueryIndex, remote.QueryMeta.Index)
		return nil
	}, func(err error) {
		s.logger.Printf("[ERR] connect: error replicating intentions: %v", err)
	})
}

// retryLoopBackoff loops a given function indefinitely, backing off exponentially
// upon errors up to a maximum of maxRetryBackoff seconds.
func retryLoopBackoff(stopCh <-chan struct{}, loopFn func() error, errFn func(error)) {
	var failedAttempts uint
	limiter := rate.NewLimiter(loopRateLimit, retryBucketSize)
	for {
		// Rate limit how often we run the loop
		limiter.Wait(context.Background())
		select {
		case <-stopCh:
			return
		default:
		}
		if (1 << failedAttempts) < maxRetryBackoff {
			failedAttempts++
		}
		retryTime := (1 << failedAttempts) * time.Second

		if err := loopFn(); err != nil {
			errFn(err)
			time.Sleep(retryTime)
			continue
		}

		// Reset the failed attempts after a successful run.
		failedAttempts = 0
	}
}

// diffIntentions computes the difference between the local and remote intentions
// and returns lists of deletes and updates.
func diffIntentions(local, remote structs.Intentions) (structs.Intentions, structs.Intentions) {
	localIdx := make(map[string][]byte, len(local))
	remoteIdx := make(map[string]struct{}, len(remote))

	var deletes structs.Intentions
	var updates structs.Intentions

	for _, intention := range local {
		localIdx[intention.ID] = intention.Hash
	}
	for _, intention := range remote {
		remoteIdx[intention.ID] = struct{}{}
	}

	for _, intention := range local {
		if _, ok := remoteIdx[intention.ID]; !ok {
			deletes = append(deletes, intention)
		}
	}

	for _, intention := range remote {
		existingHash, ok := localIdx[intention.ID]
		if !ok {
			updates = append(updates, intention)
		} else if bytes.Compare(existingHash, intention.Hash) != 0 {
			updates = append(updates, intention)
		}
	}

	return deletes, updates
}

// batchIntentionUpdates breaks up the given updates into sets of TxnOps based
// on the estimated size of the operations.
func batchIntentionUpdates(deletes, updates structs.Intentions) []structs.TxnOps {
	var txnOps structs.TxnOps
	for _, delete := range deletes {
		deleteOp := &structs.TxnIntentionOp{
			Op:        structs.IntentionOpDelete,
			Intention: delete,
		}
		txnOps = append(txnOps, &structs.TxnOp{Intention: deleteOp})
	}

	for _, update := range updates {
		updateOp := &structs.TxnIntentionOp{
			Op:        structs.IntentionOpUpdate,
			Intention: update,
		}
		txnOps = append(txnOps, &structs.TxnOp{Intention: updateOp})
	}

	// Divide the operations into chunks according to maxIntentionTxnSize.
	var batchedOps []structs.TxnOps
	for batchStart := 0; batchStart < len(txnOps); {
		// inner loop finds the last element to include in this batch.
		batchSize := 0
		batchEnd := batchStart
		for ; batchEnd < len(txnOps) && batchSize < maxIntentionTxnSize; batchEnd += 1 {
			batchSize += txnOps[batchEnd].Intention.Intention.EstimateSize()
		}

		batchedOps = append(batchedOps, txnOps[batchStart:batchEnd])

		// txnOps[batchEnd] wasn't included as the slicing doesn't include the element at the stop index
		batchStart = batchEnd
	}

	return batchedOps
}

// nextIndexVal computes the next index value to query for, resetting to zero
// if the index went backward.
func nextIndexVal(prevIdx, idx uint64) uint64 {
	if prevIdx > idx {
		return 0
	}
	return idx
}

// initializeSecondaryProvider configures the given provider for a secondary, non-root datacenter.
func (s *Server) initializeSecondaryProvider(provider ca.Provider, roots structs.IndexedCARoots) error {
	if roots.TrustDomain == "" {
		return fmt.Errorf("trust domain from primary datacenter is not initialized")
	}

	clusterID := strings.Split(roots.TrustDomain, ".")[0]
	_, conf, err := s.fsm.State().CAConfig(nil)
	if err != nil {
		return err
	}

	if err := provider.Configure(clusterID, false, conf.Config); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
	}

	s.actingSecondaryLock.Lock()
	s.actingSecondaryCA = true
	s.actingSecondaryLock.Unlock()

	return nil
}

func (s *Server) configuredSecondaryCA() bool {
	s.actingSecondaryLock.RLock()
	defer s.actingSecondaryLock.RUnlock()
	return s.actingSecondaryCA
}
