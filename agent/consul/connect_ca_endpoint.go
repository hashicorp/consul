package consul

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/lib/semaphore"

	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/acl"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/go-memdb"
)

var (
	// Err strings. net/rpc doesn't have a way to transport typed/rich errors so
	// we currently rely on sniffing the error string in a few cases where we need
	// to change client behavior. These are the canonical error strings to use.
	// Note though that client code can't use `err == consul.Err*` directly since
	// the error returned by RPC will be a plain error.errorString created by
	// net/rpc client so will not be the same _instance_ that this package
	// variable points to. Clients need to compare using `err.Error() ==
	// consul.ErrRateLimited.Error()` which is very sad. Short of replacing our
	// RPC mechanism it's hard to know how to make that much better though.
	ErrConnectNotEnabled    = errors.New("Connect must be enabled in order to use this endpoint")
	ErrRateLimited          = errors.New("Rate limit reached, try again later")
	ErrNotPrimaryDatacenter = errors.New("not the primary datacenter")
)

const (
	// csrLimitWait is the maximum time we'll wait for a slot when CSR concurrency
	// limiting or rate limiting is occurring. It's intentionally short so small
	// batches of requests can be accommodated when server has capacity (assuming
	// signing one cert takes much less than this) but failing requests fast when
	// a thundering herd comes along.
	csrLimitWait = 500 * time.Millisecond
)

// ConnectCA manages the Connect CA.
type ConnectCA struct {
	// srv is a pointer back to the server.
	srv *Server

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
func (s *ConnectCA) getCSRRateLimiterWithLimit(limit rate.Limit) *rate.Limiter {
	s.csrRateLimiterMu.RLock()
	lim := s.csrRateLimiter
	s.csrRateLimiterMu.RUnlock()

	// If there is a current limiter with the same limit, return it. This should
	// be the common case.
	if lim != nil && lim.Limit() == limit {
		return lim
	}

	// Need to change limiter, get write lock
	s.csrRateLimiterMu.Lock()
	defer s.csrRateLimiterMu.Unlock()
	// No limiter yet, or limit changed in CA config, reconfigure a new limiter.
	// We use burst of 1 for a hard limit. Note that either bursting or waiting is
	// necessary to get expected behavior in fact of random arrival times, but we
	// don't need both and we use Wait with a small delay to smooth noise. See
	// https://github.com/banks/sim-rate-limit-backoff/blob/master/README.md.
	s.csrRateLimiter = rate.NewLimiter(limit, 1)
	return s.csrRateLimiter
}

// ConfigurationGet returns the configuration for the CA.
func (s *ConnectCA) ConfigurationGet(
	args *structs.DCSpecificRequest,
	reply *structs.CAConfiguration) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.forward("ConnectCA.ConfigurationGet", args, args, reply); done {
		return err
	}

	// This action requires operator read access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorRead() {
		return acl.ErrPermissionDenied
	}

	state := s.srv.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}
	*reply = *config

	return nil
}

// ConfigurationSet updates the configuration for the CA.
func (s *ConnectCA) ConfigurationSet(
	args *structs.CARequest,
	reply *interface{}) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.forward("ConnectCA.ConfigurationSet", args, args, reply); done {
		return err
	}

	// This action requires operator write access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorWrite() {
		return acl.ErrPermissionDenied
	}

	// Exit early if it's a no-op change
	state := s.srv.fsm.State()
	confIdx, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}

	// Don't allow users to change the ClusterID.
	args.Config.ClusterID = config.ClusterID
	if args.Config.Provider == config.Provider && reflect.DeepEqual(args.Config.Config, config.Config) {
		return nil
	}

	// Create a new instance of the provider described by the config
	// and get the current active root CA. This acts as a good validation
	// of the config and makes sure the provider is functioning correctly
	// before we commit any changes to Raft.
	newProvider, err := s.srv.createCAProvider(args.Config)
	if err != nil {
		return fmt.Errorf("could not initialize provider: %v", err)
	}
	if err := newProvider.Configure(args.Config.ClusterID, true, args.Config.Config); err != nil {
		return fmt.Errorf("error configuring provider: %v", err)
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

	// Compare the new provider's root CA ID to the current one. If they
	// match, just update the existing provider with the new config.
	// If they don't match, begin the root rotation process.
	_, root, err := state.CARootActive(nil)
	if err != nil {
		return err
	}

	// If the root didn't change or if this is a secondary DC, just update the
	// config and return.
	if (s.srv.config.Datacenter != s.srv.config.PrimaryDatacenter) ||
		root != nil && root.ID == newActiveRoot.ID {
		args.Op = structs.CAOpSetConfig
		resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
		if err != nil {
			return err
		}
		if respErr, ok := resp.(error); ok {
			return respErr
		}

		// If the config has been committed, update the local provider instance
		s.srv.setCAProvider(newProvider, newActiveRoot)

		s.srv.logger.Printf("[INFO] connect: CA provider config updated")

		return nil
	}

	// At this point, we know the config change has trigged a root rotation,
	// either by swapping the provider type or changing the provider's config
	// to use a different root certificate.

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

	// Have the old provider cross-sign the new intermediate
	oldProvider, _ := s.srv.getCAProvider()
	if oldProvider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}
	xcCert, err := oldProvider.CrossSignCA(newRoot)
	if err != nil {
		return err
	}

	// Add the cross signed cert to the new root's intermediates.
	newActiveRoot.IntermediateCerts = []string{xcCert}
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
	resp, err := s.srv.raftApply(structs.ConnectCARequestType, args)
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
	s.srv.setCAProvider(newProvider, newActiveRoot)

	if err := oldProvider.Cleanup(); err != nil {
		s.srv.logger.Printf("[WARN] connect: failed to clean up old provider %q", config.Provider)
	}

	s.srv.logger.Printf("[INFO] connect: CA rotated to new root under provider %q", args.Config.Provider)

	return nil
}

// Roots returns the currently trusted root certificates.
func (s *ConnectCA) Roots(
	args *structs.DCSpecificRequest,
	reply *structs.IndexedCARoots) error {
	// Forward if necessary
	if done, err := s.srv.forward("ConnectCA.Roots", args, args, reply); done {
		return err
	}

	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	return s.srv.blockingQuery(
		&args.QueryOptions, &reply.QueryMeta,
		func(ws memdb.WatchSet, state *state.Store) error {
			index, roots, config, err := state.CARootsAndConfig(ws)
			if err != nil {
				return err
			}

			if config != nil {
				// Build TrustDomain based on the ClusterID stored.
				signingID := connect.SpiffeIDSigningForCluster(config)
				if signingID == nil {
					// If CA is bootstrapped at all then this should never happen but be
					// defensive.
					return errors.New("no cluster trust domain setup")
				}
				reply.TrustDomain = signingID.Host()
			}

			reply.Index, reply.Roots = index, roots
			if reply.Roots == nil {
				reply.Roots = make(structs.CARoots, 0)
			}

			// The API response must NEVER contain the secret information
			// such as keys and so on. We use a whitelist below to copy the
			// specific fields we want to expose.
			for i, r := range reply.Roots {
				// IMPORTANT: r must NEVER be modified, since it is a pointer
				// directly to the structure in the memdb store.

				reply.Roots[i] = &structs.CARoot{
					ID:                  r.ID,
					Name:                r.Name,
					SerialNumber:        r.SerialNumber,
					SigningKeyID:        r.SigningKeyID,
					ExternalTrustDomain: r.ExternalTrustDomain,
					NotBefore:           r.NotBefore,
					NotAfter:            r.NotAfter,
					RootCert:            r.RootCert,
					IntermediateCerts:   r.IntermediateCerts,
					RaftIndex:           r.RaftIndex,
					Active:              r.Active,
					PrivateKeyType:      r.PrivateKeyType,
					PrivateKeyBits:      r.PrivateKeyBits,
				}

				if r.Active {
					reply.ActiveRootID = r.ID
				}
			}

			return nil
		},
	)
}

// Sign signs a certificate for a service.
func (s *ConnectCA) Sign(
	args *structs.CASignRequest,
	reply *structs.IssuedCert) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.forward("ConnectCA.Sign", args, args, reply); done {
		return err
	}

	// Parse the CSR
	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	// Parse the SPIFFE ID
	spiffeID, err := connect.ParseCertURI(csr.URIs[0])
	if err != nil {
		return err
	}

	provider, caRoot := s.srv.getCAProvider()
	if provider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	} else if caRoot == nil {
		return fmt.Errorf("internal error: CA root is nil")
	}

	// Verify that the CSR entity is in the cluster's trust domain
	state := s.srv.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return err
	}
	signingID := connect.SpiffeIDSigningForCluster(config)
	serviceID, isService := spiffeID.(*connect.SpiffeIDService)
	agentID, isAgent := spiffeID.(*connect.SpiffeIDAgent)
	if !isService && !isAgent {
		return fmt.Errorf("SPIFFE ID in CSR must be a service or agent ID")
	}

	if isService {
		if !signingID.CanSign(spiffeID) {
			return fmt.Errorf("SPIFFE ID in CSR from a different trust domain: %s, "+
				"we are %s", serviceID.Host, signingID.Host())
		}
	}

	// Verify that the ACL token provided has permission to act as this service
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if isService {
		if rule != nil && !rule.ServiceWrite(serviceID.Service, nil) {
			return acl.ErrPermissionDenied
		}

		// Verify that the DC in the service URI matches us. We might relax this
		// requirement later but being restrictive for now is safer.
		if serviceID.Datacenter != s.srv.config.Datacenter {
			return fmt.Errorf("SPIFFE ID in CSR from a different datacenter: %s, "+
				"we are %s", serviceID.Datacenter, s.srv.config.Datacenter)
		}
	} else if isAgent {
		if rule != nil && !rule.NodeWrite(agentID.Agent, nil) {
			return acl.ErrPermissionDenied
		}
	}

	commonCfg, err := config.GetCommonConfig()
	if err != nil {
		return err
	}
	if commonCfg.CSRMaxPerSecond > 0 {
		lim := s.getCSRRateLimiterWithLimit(rate.Limit(commonCfg.CSRMaxPerSecond))
		// Wait up to the small threshold we allow for a token.
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if lim.Wait(ctx) != nil {
			return ErrRateLimited
		}
	} else if commonCfg.CSRMaxConcurrent > 0 {
		s.csrConcurrencyLimiter.SetSize(int64(commonCfg.CSRMaxConcurrent))
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if err := s.csrConcurrencyLimiter.Acquire(ctx); err != nil {
			return ErrRateLimited
		}
		defer s.csrConcurrencyLimiter.Release()
	}

	// All seems to be in order, actually sign it.
	pem, err := provider.Sign(csr)
	if err != nil {
		return err
	}

	// Append any intermediates needed by this root.
	for _, p := range caRoot.IntermediateCerts {
		pem = strings.TrimSpace(pem) + "\n" + p
	}

	// Append our local CA's intermediate if there is one.
	inter, err := provider.ActiveIntermediate()
	if err != nil {
		return err
	}
	root, err := provider.ActiveRoot()
	if err != nil {
		return err
	}

	if inter != root {
		pem = strings.TrimSpace(pem) + "\n" + inter
	}

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
		Op:           structs.CALeafOpIncrementIndex,
		Datacenter:   s.srv.config.Datacenter,
		WriteRequest: structs.WriteRequest{Token: args.Token},
	}

	resp, err := s.srv.raftApply(structs.ConnectCALeafRequestType|structs.IgnoreUnknownTypeFlag, &req)
	if err != nil {
		return err
	}

	modIdx, ok := resp.(uint64)
	if !ok {
		return fmt.Errorf("Invalid response from updating the leaf cert index")
	}

	cert, err := connect.ParseCert(pem)
	if err != nil {
		return err
	}

	// Set the response
	*reply = structs.IssuedCert{
		SerialNumber: connect.HexString(cert.SerialNumber.Bytes()),
		CertPEM:      pem,
		ValidAfter:   cert.NotBefore,
		ValidBefore:  cert.NotAfter,
		RaftIndex: structs.RaftIndex{
			ModifyIndex: modIdx,
			CreateIndex: modIdx,
		},
	}
	if isService {
		reply.Service = serviceID.Service
		reply.ServiceURI = cert.URIs[0].String()
	} else if isAgent {
		reply.Agent = agentID.Agent
		reply.AgentURI = cert.URIs[0].String()
	}

	return nil
}

// SignIntermediate signs an intermediate certificate for a remote datacenter.
func (s *ConnectCA) SignIntermediate(
	args *structs.CASignRequest,
	reply *string) error {
	// Exit early if Connect hasn't been enabled.
	if !s.srv.config.ConnectEnabled {
		return ErrConnectNotEnabled
	}

	if done, err := s.srv.forward("ConnectCA.SignIntermediate", args, args, reply); done {
		return err
	}

	// Verify we are allowed to serve this request
	if s.srv.config.PrimaryDatacenter != s.srv.config.Datacenter {
		return ErrNotPrimaryDatacenter
	}

	// This action requires operator write access.
	rule, err := s.srv.ResolveToken(args.Token)
	if err != nil {
		return err
	}
	if rule != nil && !rule.OperatorWrite() {
		return acl.ErrPermissionDenied
	}

	provider, _ := s.srv.getCAProvider()
	if provider == nil {
		return fmt.Errorf("internal error: CA provider is nil")
	}

	csr, err := connect.ParseCSR(args.CSR)
	if err != nil {
		return err
	}

	cert, err := provider.SignIntermediate(csr)
	if err != nil {
		return err
	}

	*reply = cert

	return nil
}
