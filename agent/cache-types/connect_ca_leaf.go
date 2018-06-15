package cachetype

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"
)

// Recommended name for registration.
const ConnectCALeafName = "connect-ca-leaf"

// ConnectCALeaf supports fetching and generating Connect leaf
// certificates.
type ConnectCALeaf struct {
	caIndex uint64 // Current index for CA roots

	issuedCertsLock sync.RWMutex
	issuedCerts     map[string]*structs.IssuedCert

	RPC   RPC          // RPC client for remote requests
	Cache *cache.Cache // Cache that has CA root certs via ConnectCARoot
}

func (c *ConnectCALeaf) Fetch(opts cache.FetchOptions, req cache.Request) (cache.FetchResult, error) {
	var result cache.FetchResult

	// Get the correct type
	reqReal, ok := req.(*ConnectCALeafRequest)
	if !ok {
		return result, fmt.Errorf(
			"Internal cache failure: request wrong type: %T", req)
	}

	// This channel watches our overall timeout. The other goroutines
	// launched in this function should end all around the same time so
	// they clean themselves up.
	timeoutCh := time.After(opts.Timeout)

	// Kick off the goroutine that waits for new CA roots. The channel buffer
	// is so that the goroutine doesn't block forever if we return for other
	// reasons.
	newRootCACh := make(chan error, 1)
	go c.waitNewRootCA(reqReal.Datacenter, newRootCACh, opts.Timeout)

	// Get our prior cert (if we had one) and use that to determine our
	// expiration time. If no cert exists, we expire immediately since we
	// need to generate.
	c.issuedCertsLock.RLock()
	lastCert := c.issuedCerts[reqReal.Service]
	c.issuedCertsLock.RUnlock()

	var leafExpiryCh <-chan time.Time
	if lastCert != nil {
		// Determine how long we wait until triggering. If we've already
		// expired, we trigger immediately.
		if expiryDur := lastCert.ValidBefore.Sub(time.Now()); expiryDur > 0 {
			leafExpiryCh = time.After(expiryDur - 1*time.Hour)
			// TODO(mitchellh): 1 hour buffer is hardcoded above
		}
	}

	if leafExpiryCh == nil {
		// If the channel is still nil then it means we need to generate
		// a cert no matter what: we either don't have an existing one or
		// it is expired.
		leafExpiryCh = time.After(0)
	}

	// Block on the events that wake us up.
	select {
	case <-timeoutCh:
		// On a timeout, we just return the empty result and no error.
		// It isn't an error to timeout, its just the limit of time the
		// caching system wants us to block for. By returning an empty result
		// the caching system will ignore.
		return result, nil

	case err := <-newRootCACh:
		// A new root CA triggers us to refresh the leaf certificate.
		// If there was an error while getting the root CA then we return.
		// Otherwise, we leave the select statement and move to generation.
		if err != nil {
			return result, err
		}

	case <-leafExpiryCh:
		// The existing leaf certificate is expiring soon, so we generate a
		// new cert with a healthy overlapping validity period (determined
		// by the above channel).
	}

	// Need to lookup RootCAs response to discover trust domain. First just lookup
	// with no blocking info - this should be a cache hit most of the time.
	rawRoots, _, err := c.Cache.Get(ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: reqReal.Datacenter,
	})
	if err != nil {
		return result, err
	}
	roots, ok := rawRoots.(*structs.IndexedCARoots)
	if !ok {
		return result, errors.New("invalid RootCA response type")
	}
	if roots.TrustDomain == "" {
		return result, errors.New("cluster has no CA bootstrapped")
	}

	// Build the service ID
	serviceID := &connect.SpiffeIDService{
		Host:       roots.TrustDomain,
		Datacenter: reqReal.Datacenter,
		Namespace:  "default",
		Service:    reqReal.Service,
	}

	// Create a new private key
	pk, pkPEM, err := connect.GeneratePrivateKey()
	if err != nil {
		return result, err
	}

	// Create a CSR.
	csr, err := connect.CreateCSR(serviceID, pk)
	if err != nil {
		return result, err
	}

	// Request signing
	var reply structs.IssuedCert
	args := structs.CASignRequest{
		WriteRequest: structs.WriteRequest{Token: reqReal.Token},
		Datacenter:   reqReal.Datacenter,
		CSR:          csr,
	}
	if err := c.RPC.RPC("ConnectCA.Sign", &args, &reply); err != nil {
		return result, err
	}
	reply.PrivateKeyPEM = pkPEM

	// Lock the issued certs map so we can insert it. We only insert if
	// we didn't happen to get a newer one. This should never happen since
	// the Cache should ensure only one Fetch per service, but we sanity
	// check just in case.
	c.issuedCertsLock.Lock()
	defer c.issuedCertsLock.Unlock()
	lastCert = c.issuedCerts[reqReal.Service]
	if lastCert == nil || lastCert.ModifyIndex < reply.ModifyIndex {
		if c.issuedCerts == nil {
			c.issuedCerts = make(map[string]*structs.IssuedCert)
		}

		c.issuedCerts[reqReal.Service] = &reply
		lastCert = &reply
	}

	result.Value = lastCert
	result.Index = lastCert.ModifyIndex
	return result, nil
}

// waitNewRootCA blocks until a new root CA is available or the timeout is
// reached (on timeout ErrTimeout is returned on the channel).
func (c *ConnectCALeaf) waitNewRootCA(datacenter string, ch chan<- error,
	timeout time.Duration) {
	// We always want to block on at least an initial value. If this isn't
	minIndex := atomic.LoadUint64(&c.caIndex)
	if minIndex == 0 {
		minIndex = 1
	}

	// Fetch some new roots. This will block until our MinQueryIndex is
	// matched or the timeout is reached.
	rawRoots, _, err := c.Cache.Get(ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: datacenter,
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: minIndex,
			MaxQueryTime:  timeout,
		},
	})
	if err != nil {
		ch <- err
		return
	}

	roots, ok := rawRoots.(*structs.IndexedCARoots)
	if !ok {
		// This should never happen but we don't want to even risk a panic
		ch <- fmt.Errorf(
			"internal error: CA root cache returned bad type: %T", rawRoots)
		return
	}

	// We do a loop here because there can be multiple waitNewRootCA calls
	// happening simultaneously. Each Fetch kicks off one call. These are
	// multiplexed through Cache.Get which should ensure we only ever
	// actually make a single RPC call. However, there is a race to set
	// the caIndex field so do a basic CAS loop here.
	for {
		// We only set our index if its newer than what is previously set.
		old := atomic.LoadUint64(&c.caIndex)
		if old == roots.Index || old > roots.Index {
			break
		}

		// Set the new index atomically. If the caIndex value changed
		// in the meantime, retry.
		if atomic.CompareAndSwapUint64(&c.caIndex, old, roots.Index) {
			break
		}
	}

	// Trigger the channel since we updated.
	ch <- nil
}

// ConnectCALeafRequest is the cache.Request implementation for the
// ConnectCALeaf cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ConnectCALeafRequest struct {
	Token         string
	Datacenter    string
	Service       string // Service name, not ID
	MinQueryIndex uint64
}

func (r *ConnectCALeafRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Token:      r.Token,
		Key:        r.Service,
		Datacenter: r.Datacenter,
		MinIndex:   r.MinQueryIndex,
	}
}
