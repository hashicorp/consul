package healthcheck

import (
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// UpstreamHostDownFunc can be used to customize how Down behaves.
type UpstreamHostDownFunc func(*UpstreamHost) bool

// UpstreamHost represents a single proxy upstream
type UpstreamHost struct {
	Conns       int64  // must be first field to be 64-bit aligned on 32-bit systems
	Name        string // IP address (and port) of this upstream host
	Fails       int32
	FailTimeout time.Duration
	CheckDown   UpstreamHostDownFunc
	CheckURL    string
	Checking    bool
	sync.Mutex
}

// Down checks whether the upstream host is down or not.
// Down will try to use uh.CheckDown first, and will fall
// back to some default criteria if necessary.
func (uh *UpstreamHost) Down() bool {
	if uh.CheckDown == nil {
		fails := atomic.LoadInt32(&uh.Fails)
		return fails > 0
	}
	return uh.CheckDown(uh)
}

// HostPool is a collection of UpstreamHosts.
type HostPool []*UpstreamHost

// HealthCheck is used for performing healthcheck
// on a collection of upstream hosts and select
// one based on the policy.
type HealthCheck struct {
	wg          sync.WaitGroup // Used to wait for running goroutines to stop.
	stop        chan struct{}  // Signals running goroutines to stop.
	Hosts       HostPool
	Policy      Policy
	Spray       Policy
	FailTimeout time.Duration
	MaxFails    int32
	Path        string
	Port        string
	Interval    time.Duration
}

// Start starts the healthcheck
func (u *HealthCheck) Start() {
	for i, h := range u.Hosts {
		u.Hosts[i].CheckURL = u.normalizeCheckURL(h.Name)
	}

	u.stop = make(chan struct{})
	if u.Path != "" {
		u.wg.Add(1)
		go func() {
			defer u.wg.Done()
			u.healthCheckWorker(u.stop)
		}()
	}
}

// Stop sends a signal to all goroutines started by this staticUpstream to exit
// and waits for them to finish before returning.
func (u *HealthCheck) Stop() error {
	close(u.stop)
	u.wg.Wait()
	return nil
}

// This was moved into a thread so that each host could throw a health
// check at the same time.  The reason for this is that if we are checking
// 3 hosts, and the first one is gone, and we spend minutes timing out to
// fail it, we would not have been doing any other health checks in that
// time.  So we now have a per-host lock and a threaded health check.
//
// We use the Checking bool to avoid concurrent checks against the same
// host; if one is taking a long time, the next one will find a check in
// progress and simply return before trying.
//
// We are carefully avoiding having the mutex locked while we check,
// otherwise checks will back up, potentially a lot of them if a host is
// absent for a long time.  This arrangement makes checks quickly see if
// they are the only one running and abort otherwise.

// HealthCheckURL performs the http.Get that implements healthcheck.
func (uh *UpstreamHost) HealthCheckURL() {
	// Lock for our bool check.  We don't just defer the unlock because
	// we don't want the lock held while http.Get runs.
	uh.Lock()

	// We call HealthCheckURL from proxy.go and lookup.go, bail out when nothing
	// is configured to healthcheck. Or we mid check?  Don't run another one.
	if uh.CheckURL == "" || uh.Checking { // nothing configured
		uh.Unlock()
		return
	}

	uh.Checking = true
	uh.Unlock()

	// default timeout (5s)
	r, err := healthClient.Get(uh.CheckURL)

	defer func() {
		uh.Lock()
		uh.Checking = false
		uh.Unlock()
	}()

	if err != nil {
		log.Printf("[WARNING] Host %s health check probe failed: %v", uh.Name, err)
		return
	}

	if err == nil {
		io.Copy(ioutil.Discard, r.Body)
		r.Body.Close()

		if r.StatusCode < 200 || r.StatusCode >= 400 {
			log.Printf("[WARNING] Host %s health check returned HTTP code %d", uh.Name, r.StatusCode)
			return
		}

		// We are healthy again, reset fails.
		atomic.StoreInt32(&uh.Fails, 0)
		return
	}
}

func (u *HealthCheck) healthCheck() {
	for _, host := range u.Hosts {
		// locks/bools should prevent requests backing up
		go host.HealthCheckURL()
	}
}

func (u *HealthCheck) healthCheckWorker(stop chan struct{}) {
	ticker := time.NewTicker(u.Interval)
	u.healthCheck()
	for {
		select {
		case <-ticker.C:
			u.healthCheck()
		case <-stop:
			ticker.Stop()
			return
		}
	}
}

// Select selects an upstream host based on the policy
// and the healthcheck result.
func (u *HealthCheck) Select() *UpstreamHost {
	pool := u.Hosts
	if len(pool) == 1 {
		if pool[0].Down() && u.Spray == nil {
			return nil
		}
		return pool[0]
	}
	allDown := true
	for _, host := range pool {
		if !host.Down() {
			allDown = false
			break
		}
	}
	if allDown {
		if u.Spray == nil {
			return nil
		}
		return u.Spray.Select(pool)
	}

	if u.Policy == nil {
		h := (&Random{}).Select(pool)
		if h != nil {
			return h
		}
		if h == nil && u.Spray == nil {
			return nil
		}
		return u.Spray.Select(pool)
	}

	h := u.Policy.Select(pool)
	if h != nil {
		return h
	}

	if u.Spray == nil {
		return nil
	}
	return u.Spray.Select(pool)
}

// normalizeCheckURL creates a proper URL for the health check.
func (u *HealthCheck) normalizeCheckURL(name string) string {
	// The DNS server might be an HTTP server.  If so, extract its name.
	hostName := name
	ret, err := url.Parse(name)
	if err == nil && len(ret.Host) > 0 {
		hostName = ret.Host
	}

	// Extract the port number from the parsed server name.
	checkHostName, checkPort, err := net.SplitHostPort(hostName)
	if err != nil {
		checkHostName = hostName
	}

	if u.Port != "" {
		checkPort = u.Port
	}

	checkURL := "http://" + net.JoinHostPort(checkHostName, checkPort) + u.Path
	return checkURL
}

var healthClient = func() *http.Client { return &http.Client{Timeout: 5 * time.Second} }()
