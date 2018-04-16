package cachetype

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/agent/cache"
	"github.com/hashicorp/consul/agent/connect"
	"github.com/hashicorp/consul/agent/structs"

	// NOTE(mitcehllh): This is temporary while certs are stubbed out.
	"github.com/mitchellh/go-testing-interface"
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
	go c.waitNewRootCA(newRootCACh, opts.Timeout)

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
		// TODO: what is the right error for a timeout?
		return result, fmt.Errorf("timeout")

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

	// Create a CSR.
	// TODO(mitchellh): This is obviously not production ready!
	csr, pk := connect.TestCSR(&testing.RuntimeT{}, &connect.SpiffeIDService{
		Host:       "1234.consul",
		Namespace:  "default",
		Datacenter: reqReal.Datacenter,
		Service:    reqReal.Service,
	})

	// Request signing
	var reply structs.IssuedCert
	args := structs.CASignRequest{CSR: csr}
	if err := c.RPC.RPC("ConnectCA.Sign", &args, &reply); err != nil {
		return result, err
	}
	reply.PrivateKeyPEM = pk

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
func (c *ConnectCALeaf) waitNewRootCA(ch chan<- error, timeout time.Duration) {
	// Fetch some new roots. This will block until our MinQueryIndex is
	// matched or the timeout is reached.
	rawRoots, err := c.Cache.Get(ConnectCARootName, &structs.DCSpecificRequest{
		Datacenter: "",
		QueryOptions: structs.QueryOptions{
			MinQueryIndex: atomic.LoadUint64(&c.caIndex),
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

	// Set the new index
	atomic.StoreUint64(&c.caIndex, roots.QueryMeta.Index)

	// Trigger the channel since we updated.
	ch <- nil
}

// ConnectCALeafRequest is the cache.Request implementation for the
// COnnectCALeaf cache type. This is implemented here and not in structs
// since this is only used for cache-related requests and not forwarded
// directly to any Consul servers.
type ConnectCALeafRequest struct {
	Datacenter    string
	Service       string // Service name, not ID
	MinQueryIndex uint64
}

func (r *ConnectCALeafRequest) CacheInfo() cache.RequestInfo {
	return cache.RequestInfo{
		Key:        r.Service,
		Datacenter: r.Datacenter,
		MinIndex:   r.MinQueryIndex,
	}
}
