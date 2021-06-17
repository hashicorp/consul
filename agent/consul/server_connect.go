package consul

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/url"
	"strings"
	"sync"

	memdb "github.com/hashicorp/go-memdb"
	"golang.org/x/time/rate"

	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/connect/ca"
	"github.com/hashicorp/consul/agent/consul/state"
	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/lib/semaphore"
)

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

// GetCARoots will retrieve
func (s *Server) GetCARoots() (*structs.IndexedCARoots, error) {
	return s.getCARoots(nil, s.fsm.State())
}

func (s *Server) getCARoots(ws memdb.WatchSet, state *state.Store) (*structs.IndexedCARoots, error) {
	index, roots, config, err := state.CARootsAndConfig(ws)
	if err != nil {
		return nil, err
	}

	indexedRoots := &structs.IndexedCARoots{}

	if config != nil {
		// Build TrustDomain based on the ClusterID stored.
		signingID := connect.SpiffeIDSigningForCluster(config)
		if signingID == nil {
			// If CA is bootstrapped at all then this should never happen but be
			// defensive.
			return nil, fmt.Errorf("no cluster trust domain setup")
		}

		indexedRoots.TrustDomain = signingID.Host()
	}

	indexedRoots.Index, indexedRoots.Roots = index, roots
	if indexedRoots.Roots == nil {
		indexedRoots.Roots = make(structs.CARoots, 0)
	}

	// The response should not contain all fields as there are sensitive
	// data such as key material stored within the struct. So here we
	// pull out some of the fields and copy them into
	for i, r := range indexedRoots.Roots {
		// IMPORTANT: r must NEVER be modified, since it is a pointer
		// directly to the structure in the memdb store.

		indexedRoots.Roots[i] = &structs.CARoot{
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
			indexedRoots.ActiveRootID = r.ID
		}
	}

	return indexedRoots, nil
}

func (s *Server) SignCertificate(csr *x509.CertificateRequest, spiffeID connect.CertURI) (*structs.IssuedCert, error) {
	provider, caRoot := s.caManager.getCAProvider()
	if provider == nil {
		return nil, fmt.Errorf("CA is uninitialized and unable to sign certificates yet: provider is nil")
	} else if caRoot == nil {
		return nil, fmt.Errorf("CA is uninitialized and unable to sign certificates yet: no root certificate")
	}

	// Verify that the CSR entity is in the cluster's trust domain
	state := s.fsm.State()
	_, config, err := state.CAConfig(nil)
	if err != nil {
		return nil, err
	}
	signingID := connect.SpiffeIDSigningForCluster(config)
	serviceID, isService := spiffeID.(*connect.SpiffeIDService)
	agentID, isAgent := spiffeID.(*connect.SpiffeIDAgent)
	if !isService && !isAgent {
		return nil, fmt.Errorf("SPIFFE ID in CSR must be a service or agent ID")
	}

	var entMeta structs.EnterpriseMeta
	if isService {
		if !signingID.CanSign(spiffeID) {
			return nil, fmt.Errorf("SPIFFE ID in CSR from a different trust domain: %s, "+
				"we are %s", serviceID.Host, signingID.Host())
		}
		entMeta.Merge(serviceID.GetEnterpriseMeta())
	} else {
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
		entMeta.Merge(structs.DefaultEnterpriseMeta())
	}

	commonCfg, err := config.GetCommonConfig()
	if err != nil {
		return nil, err
	}
	if commonCfg.CSRMaxPerSecond > 0 {
		lim := s.caLeafLimiter.getCSRRateLimiterWithLimit(rate.Limit(commonCfg.CSRMaxPerSecond))
		// Wait up to the small threshold we allow for a token.
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if lim.Wait(ctx) != nil {
			return nil, ErrRateLimited
		}
	} else if commonCfg.CSRMaxConcurrent > 0 {
		s.caLeafLimiter.csrConcurrencyLimiter.SetSize(int64(commonCfg.CSRMaxConcurrent))
		ctx, cancel := context.WithTimeout(context.Background(), csrLimitWait)
		defer cancel()
		if err := s.caLeafLimiter.csrConcurrencyLimiter.Acquire(ctx); err != nil {
			return nil, ErrRateLimited
		}
		defer s.caLeafLimiter.csrConcurrencyLimiter.Release()
	}

	connect.HackSANExtensionForCSR(csr)

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
		pem = strings.TrimSpace(pem) + "\n" + p
	}

	// Append our local CA's intermediate if there is one.
	inter, err := provider.ActiveIntermediate()
	if err != nil {
		return nil, err
	}
	root, err := provider.ActiveRoot()
	if err != nil {
		return nil, err
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
		Op:         structs.CALeafOpIncrementIndex,
		Datacenter: s.config.Datacenter,
	}

	resp, err := s.raftApply(structs.ConnectCALeafRequestType|structs.IgnoreUnknownTypeFlag, &req)
	if err != nil {
		return nil, err
	}

	modIdx, ok := resp.(uint64)
	if !ok {
		return nil, fmt.Errorf("Invalid response from updating the leaf cert index")
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
	if isService {
		reply.Service = serviceID.Service
		reply.ServiceURI = cert.URIs[0].String()
	} else if isAgent {
		reply.Agent = agentID.Agent
		reply.AgentURI = cert.URIs[0].String()
	}

	return &reply, nil
}
