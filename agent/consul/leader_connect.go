package consul

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/autopilot"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-version"
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

	// minMultiDCConnectVersion is the minimum version in order to support multi-DC Connect
	// features.
	minMultiDCConnectVersion = version.Must(version.NewVersion("1.4.0"))

	// maxRootsQueryTime is the maximum time the primary roots watch query can block before
	// returning.
	maxRootsQueryTime = maxQueryTime

	errEmptyVersion = errors.New("version string is empty")
)

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

	// Check whether the primary DC has been upgraded to support multi-DC Connect.
	// If it hasn't, we skip the secondary initialization routine and continue acting
	// as a primary DC. This is periodically re-checked in the goroutine watching the
	// primary's CA roots so that we can transition to a secondary DC when it has
	// been upgraded.
	var primaryHasVersion bool
	if s.config.PrimaryDatacenter != s.config.Datacenter {
		primaryHasVersion, err = s.datacentersMeetMinVersion(minMultiDCConnectVersion)
		if err == errEmptyVersion {
			s.logger.Printf("[WARN] connect: primary datacenter %q is reachable but not yet initialized", s.config.PrimaryDatacenter)
			return nil
		} else if err != nil {
			s.logger.Printf("[ERR] connect: error initializing CA: could not query primary datacenter: %v", err)
			return nil
		}
	}

	// If this isn't the primary DC, run the secondary DC routine if the primary has already
	// been upgraded to at least 1.4.0.
	if s.config.PrimaryDatacenter != s.config.Datacenter && primaryHasVersion {
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

	// Update the roots list in the state store if there's a new active root.
	state := s.fsm.State()
	_, activeRoot, err := state.CARootActive(nil)
	if err != nil {
		return err
	}
	if activeRoot == nil || activeRoot.ID != newActiveRoot.ID {
		idx, oldRoots, err := state.CARoots(nil)
		if err != nil {
			return err
		}

		_, config, err := state.CAConfig()
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

	// Get a signed intermediate from the primary DC if the provider
	// hasn't been initialized yet or if the primary's root has changed.
	if activeIntermediate == "" || storedRootID != roots.ActiveRootID {
		csr, err := provider.GenerateIntermediateCSR()
		if err != nil {
			return err
		}

		var intermediatePEM string
		if err := s.forwardDC("ConnectCA.SignIntermediate", s.config.PrimaryDatacenter, s.generateCASignRequest(csr), &intermediatePEM); err != nil {
			return err
		}

		if err := provider.SetIntermediate(intermediatePEM, newActiveRoot.RootCert); err != nil {
			return err
		}

		// Append the new intermediate to our local active root entry.
		newActiveRoot.IntermediateCerts = append(newActiveRoot.IntermediateCerts, intermediatePEM)

		s.logger.Printf("[INFO] connect: received new intermediate certificate from primary datacenter")
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
			return err
		}

		// Check to see if the primary has been upgraded in case we're waiting to switch to
		// secondary mode.
		provider, _ := s.getCAProvider()
		if !s.configuredSecondaryCA() {
			primaryHasVersion, err := s.datacentersMeetMinVersion(minMultiDCConnectVersion)
			if err != nil {
				return err
			}

			if primaryHasVersion {
				if err := s.initializeSecondaryProvider(provider, roots); err != nil {
					return err
				}
			}
		}

		// Run the secondary CA init routine to see if we need to request a new
		// intermediate.
		if s.configuredSecondaryCA() {
			if err := s.initializeSecondaryCA(provider, roots); err != nil {
				return err
			}
		}

		args.QueryOptions.MinQueryIndex = nextIndexVal(args.QueryOptions.MinQueryIndex, roots.QueryMeta.Index)
		return nil
	}, func(err error) {
		// Don't log the error if it's a result of the primary still starting up.
		if err != errEmptyVersion {
			s.logger.Printf("[ERR] connect: error watching primary datacenter roots: %v", err)
		}
	})
}

// replicateIntentions executes a blocking query to the primary datacenter to replicate
// the intentions there to the local state.
func (s *Server) replicateIntentions(stopCh <-chan struct{}) {
	args := structs.DCSpecificRequest{
		Datacenter:   s.config.PrimaryDatacenter,
		QueryOptions: structs.QueryOptions{Token: s.tokens.ReplicationToken()},
	}

	s.logger.Printf("[DEBUG] connect: starting Connect intention replication from primary datacenter %q", s.config.PrimaryDatacenter)

	retryLoopBackoff(stopCh, func() error {
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

// datacentersMeetMinVersion returns whether this datacenter and the primary
// are ready and have upgraded to at least the given version.
func (s *Server) datacentersMeetMinVersion(minVersion *version.Version) (bool, error) {
	localAutopilotHealth := s.autopilot.GetClusterHealth()
	localServersMeetVersion, err := autopilotServersMeetMinimumVersion(localAutopilotHealth.Servers, minVersion)
	if err != nil {
		return false, err
	}
	if !localServersMeetVersion {
		return false, err
	}

	args := structs.DCSpecificRequest{
		Datacenter: s.config.PrimaryDatacenter,
	}
	var reply autopilot.OperatorHealthReply
	if err := s.forwardDC("Operator.ServerHealth", s.config.PrimaryDatacenter, &args, &reply); err != nil {
		return false, err
	}
	remoteServersMeetVersion, err := autopilotServersMeetMinimumVersion(reply.Servers, minVersion)
	if err != nil {
		return false, err
	}

	return localServersMeetVersion && remoteServersMeetVersion, nil
}

// autopilotServersMeetMinimumVersion returns whether the given slice of servers
// meets a minimum version.
func autopilotServersMeetMinimumVersion(servers []autopilot.ServerHealth, minVersion *version.Version) (bool, error) {
	for _, server := range servers {
		if server.Version == "" {
			return false, errEmptyVersion
		}
		version, err := version.NewVersion(server.Version)
		if err != nil {
			return false, fmt.Errorf("error parsing remote server version: %v", err)
		}

		if version.LessThan(minVersion) {
			return false, nil
		}
	}

	return true, nil
}

// initializeSecondaryProvider configures the given provider for a secondary, non-root datacenter.
func (s *Server) initializeSecondaryProvider(provider ca.Provider, roots structs.IndexedCARoots) error {
	if roots.TrustDomain == "" {
		return fmt.Errorf("trust domain from primary datacenter is not initialized")
	}

	clusterID := strings.Split(roots.TrustDomain, ".")[0]
	_, conf, err := s.fsm.State().CAConfig()
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
