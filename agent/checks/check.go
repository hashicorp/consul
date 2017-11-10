package checks

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/consul/types"
	"github.com/hashicorp/go-cleanhttp"
)

const (
	// MinInterval is the minimal interval between
	// two checks. Do not allow for a interval below this value.
	// Otherwise we risk fork bombing a system.
	MinInterval = time.Second

	// BufSize is the maximum size of the captured
	// check output. Prevents an enormous buffer
	// from being captured
	BufSize = 4 * 1024 // 4KB

	// UserAgent is the value of the User-Agent header
	// for HTTP health checks.
	UserAgent = "Consul Health Check"
)

// CheckNotifier interface is used by the CheckMonitor
// to notify when a check has a status update. The update
// should take care to be idempotent.
type CheckNotifier interface {
	UpdateCheck(checkID types.CheckID, status, output string)
}

// CheckMonitor is used to periodically invoke a script to
// determine the health of a given check. It is compatible with
// nagios plugins and expects the output in the same format.
type CheckMonitor struct {
	Notify     CheckNotifier
	CheckID    types.CheckID
	Script     string
	ScriptArgs []string
	Interval   time.Duration
	Timeout    time.Duration
	Logger     *log.Logger

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// Start is used to start a check monitor.
// Monitor runs until stop is called
func (c *CheckMonitor) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run()
}

// Stop is used to stop a check monitor.
func (c *CheckMonitor) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

// run is invoked by a goroutine to run until Stop() is called
func (c *CheckMonitor) run() {
	// Get the randomized initial pause time
	initialPauseTime := lib.RandomStagger(c.Interval)
	next := time.After(initialPauseTime)
	for {
		select {
		case <-next:
			c.check()
			next = time.After(c.Interval)
		case <-c.stopCh:
			return
		}
	}
}

// check is invoked periodically to perform the script check
func (c *CheckMonitor) check() {
	// Create the command
	var cmd *osexec.Cmd
	var err error
	if len(c.ScriptArgs) > 0 {
		cmd, err = exec.Subprocess(c.ScriptArgs)
	} else {
		cmd, err = exec.Script(c.Script)
	}
	if err != nil {
		c.Logger.Printf("[ERR] agent: Check %q failed to setup: %s", c.CheckID, err)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	// Collect the output
	output, _ := circbuf.NewBuffer(BufSize)
	cmd.Stdout = output
	cmd.Stderr = output
	exec.SetSysProcAttr(cmd)

	truncateAndLogOutput := func() string {
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}
		c.Logger.Printf("[TRACE] agent: Check %q output: %s", c.CheckID, outputStr)
		return outputStr
	}

	// Start the check
	if err := cmd.Start(); err != nil {
		c.Logger.Printf("[ERR] agent: Check %q failed to invoke: %s", c.CheckID, err)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	// Wait for the check to complete
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	timeout := 30 * time.Second
	if c.Timeout > 0 {
		timeout = c.Timeout
	}
	select {
	case <-time.After(timeout):
		if err := exec.KillCommandSubtree(cmd); err != nil {
			c.Logger.Printf("[WARN] Check %q failed to kill after timeout: %s", c.CheckID, err)
		}

		msg := fmt.Sprintf("Timed out (%s) running check", timeout.String())
		c.Logger.Printf("[WARN] Check %q: %s", c.CheckID, msg)

		outputStr := truncateAndLogOutput()
		if len(outputStr) > 0 {
			msg += "\n\n" + outputStr
		}
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, msg)

		// Now wait for the process to exit so we never start another
		// instance concurrently.
		<-waitCh
		return

	case err = <-waitCh:
		// The process returned before the timeout, proceed normally
	}

	// Check if the check passed
	outputStr := truncateAndLogOutput()
	if err == nil {
		c.Logger.Printf("[DEBUG] agent: Check %q is passing", c.CheckID)
		c.Notify.UpdateCheck(c.CheckID, api.HealthPassing, outputStr)
		return
	}

	// If the exit code is 1, set check as warning
	exitErr, ok := err.(*osexec.ExitError)
	if ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			code := status.ExitStatus()
			if code == 1 {
				c.Logger.Printf("[WARN] agent: Check %q is now warning", c.CheckID)
				c.Notify.UpdateCheck(c.CheckID, api.HealthWarning, outputStr)
				return
			}
		}
	}

	// Set the health as critical
	c.Logger.Printf("[WARN] agent: Check %q is now critical", c.CheckID)
	c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, outputStr)
}

// CheckTTL is used to apply a TTL to check status,
// and enables clients to set the status of a check
// but upon the TTL expiring, the check status is
// automatically set to critical.
type CheckTTL struct {
	Notify  CheckNotifier
	CheckID types.CheckID
	TTL     time.Duration
	Logger  *log.Logger

	timer *time.Timer

	lastOutput     string
	lastOutputLock sync.RWMutex

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// Start is used to start a check ttl, runs until Stop()
func (c *CheckTTL) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	c.timer = time.NewTimer(c.TTL)
	go c.run()
}

// Stop is used to stop a check ttl.
func (c *CheckTTL) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.timer.Stop()
		c.stop = true
		close(c.stopCh)
	}
}

// run is used to handle TTL expiration and to update the check status
func (c *CheckTTL) run() {
	for {
		select {
		case <-c.timer.C:
			c.Logger.Printf("[WARN] agent: Check %q missed TTL, is now critical",
				c.CheckID)
			c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, c.getExpiredOutput())

		case <-c.stopCh:
			return
		}
	}
}

// getExpiredOutput formats the output for the case when the TTL is expired.
func (c *CheckTTL) getExpiredOutput() string {
	c.lastOutputLock.RLock()
	defer c.lastOutputLock.RUnlock()

	const prefix = "TTL expired"
	if c.lastOutput == "" {
		return prefix
	}

	return fmt.Sprintf("%s (last output before timeout follows): %s", prefix, c.lastOutput)
}

// SetStatus is used to update the status of the check,
// and to renew the TTL. If expired, TTL is restarted.
func (c *CheckTTL) SetStatus(status, output string) {
	c.Logger.Printf("[DEBUG] agent: Check %q status is now %s", c.CheckID, status)
	c.Notify.UpdateCheck(c.CheckID, status, output)

	// Store the last output so we can retain it if the TTL expires.
	c.lastOutputLock.Lock()
	c.lastOutput = output
	c.lastOutputLock.Unlock()

	c.timer.Reset(c.TTL)
}

// CheckHTTP is used to periodically make an HTTP request to
// determine the health of a given check.
// The check is passing if the response code is 2XX.
// The check is warning if the response code is 429.
// The check is critical if the response code is anything else
// or if the request returns an error
type CheckHTTP struct {
	Notify          CheckNotifier
	CheckID         types.CheckID
	HTTP            string
	Header          map[string][]string
	Method          string
	Interval        time.Duration
	Timeout         time.Duration
	Logger          *log.Logger
	TLSClientConfig *tls.Config

	httpClient *http.Client
	stop       bool
	stopCh     chan struct{}
	stopLock   sync.Mutex
}

// Start is used to start an HTTP check.
// The check runs until stop is called
func (c *CheckHTTP) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.httpClient == nil {
		// Create the transport. We disable HTTP Keep-Alive's to prevent
		// failing checks due to the keepalive interval.
		trans := cleanhttp.DefaultTransport()
		trans.DisableKeepAlives = true

		// Take on the supplied TLS client config.
		trans.TLSClientConfig = c.TLSClientConfig

		// Create the HTTP client.
		c.httpClient = &http.Client{
			Timeout:   10 * time.Second,
			Transport: trans,
		}

		// For long (>10s) interval checks the http timeout is 10s, otherwise the
		// timeout is the interval. This means that a check *should* return
		// before the next check begins.
		if c.Timeout > 0 && c.Timeout < c.Interval {
			c.httpClient.Timeout = c.Timeout
		} else if c.Interval < 10*time.Second {
			c.httpClient.Timeout = c.Interval
		}
	}

	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run()
}

// Stop is used to stop an HTTP check.
func (c *CheckHTTP) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

// run is invoked by a goroutine to run until Stop() is called
func (c *CheckHTTP) run() {
	// Get the randomized initial pause time
	initialPauseTime := lib.RandomStagger(c.Interval)
	next := time.After(initialPauseTime)
	for {
		select {
		case <-next:
			c.check()
			next = time.After(c.Interval)
		case <-c.stopCh:
			return
		}
	}
}

// check is invoked periodically to perform the HTTP check
func (c *CheckHTTP) check() {
	method := c.Method
	if method == "" {
		method = "GET"
	}

	req, err := http.NewRequest(method, c.HTTP, nil)
	if err != nil {
		c.Logger.Printf("[WARN] agent: Check %q HTTP request failed: %s", c.CheckID, err)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	req.Header = http.Header(c.Header)

	// this happens during testing but not in prod
	if req.Header == nil {
		req.Header = make(http.Header)
	}

	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}
	if req.Header.Get("Accept") == "" {
		req.Header.Set("Accept", "text/plain, text/*, */*")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.Logger.Printf("[WARN] agent: Check %q HTTP request failed: %s", c.CheckID, err)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}
	defer resp.Body.Close()

	// Read the response into a circular buffer to limit the size
	output, _ := circbuf.NewBuffer(BufSize)
	if _, err := io.Copy(output, resp.Body); err != nil {
		c.Logger.Printf("[WARN] agent: Check %q error while reading body: %s", c.CheckID, err)
	}

	// Format the response body
	result := fmt.Sprintf("HTTP GET %s: %s Output: %s", c.HTTP, resp.Status, output.String())

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// PASSING (2xx)
		c.Logger.Printf("[DEBUG] agent: Check %q is passing", c.CheckID)
		c.Notify.UpdateCheck(c.CheckID, api.HealthPassing, result)

	} else if resp.StatusCode == 429 {
		// WARNING
		// 429 Too Many Requests (RFC 6585)
		// The user has sent too many requests in a given amount of time.
		c.Logger.Printf("[WARN] agent: Check %q is now warning", c.CheckID)
		c.Notify.UpdateCheck(c.CheckID, api.HealthWarning, result)

	} else {
		// CRITICAL
		c.Logger.Printf("[WARN] agent: Check %q is now critical", c.CheckID)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, result)
	}
}

// CheckTCP is used to periodically make an TCP/UDP connection to
// determine the health of a given check.
// The check is passing if the connection succeeds
// The check is critical if the connection returns an error
type CheckTCP struct {
	Notify   CheckNotifier
	CheckID  types.CheckID
	TCP      string
	Interval time.Duration
	Timeout  time.Duration
	Logger   *log.Logger

	dialer   *net.Dialer
	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

// Start is used to start a TCP check.
// The check runs until stop is called
func (c *CheckTCP) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.dialer == nil {
		// Create the socket dialer
		c.dialer = &net.Dialer{DualStack: true}

		// For long (>10s) interval checks the socket timeout is 10s, otherwise
		// the timeout is the interval. This means that a check *should* return
		// before the next check begins.
		if c.Timeout > 0 && c.Timeout < c.Interval {
			c.dialer.Timeout = c.Timeout
		} else if c.Interval < 10*time.Second {
			c.dialer.Timeout = c.Interval
		}
	}

	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run()
}

// Stop is used to stop a TCP check.
func (c *CheckTCP) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

// run is invoked by a goroutine to run until Stop() is called
func (c *CheckTCP) run() {
	// Get the randomized initial pause time
	initialPauseTime := lib.RandomStagger(c.Interval)
	next := time.After(initialPauseTime)
	for {
		select {
		case <-next:
			c.check()
			next = time.After(c.Interval)
		case <-c.stopCh:
			return
		}
	}
}

// check is invoked periodically to perform the TCP check
func (c *CheckTCP) check() {
	conn, err := c.dialer.Dial(`tcp`, c.TCP)
	if err != nil {
		c.Logger.Printf("[WARN] agent: Check %q socket connection failed: %s", c.CheckID, err)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}
	conn.Close()
	c.Logger.Printf("[DEBUG] agent: Check %q is passing", c.CheckID)
	c.Notify.UpdateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("TCP connect %s: Success", c.TCP))
}

// CheckDocker is used to periodically invoke a script to
// determine the health of an application running inside a
// Docker Container. We assume that the script is compatible
// with nagios plugins and expects the output in the same format.
type CheckDocker struct {
	Notify            CheckNotifier
	CheckID           types.CheckID
	Script            string
	ScriptArgs        []string
	DockerContainerID string
	Shell             string
	Interval          time.Duration
	Logger            *log.Logger
	Client            *DockerClient

	stop chan struct{}
}

func (c *CheckDocker) Start() {
	if c.stop != nil {
		panic("Docker check already started")
	}

	if c.Logger == nil {
		c.Logger = log.New(ioutil.Discard, "", 0)
	}

	if c.Shell == "" {
		c.Shell = os.Getenv("SHELL")
		if c.Shell == "" {
			c.Shell = "/bin/sh"
		}
	}
	c.stop = make(chan struct{})
	go c.run()
}

func (c *CheckDocker) Stop() {
	if c.stop == nil {
		panic("Stop called before start")
	}
	close(c.stop)
}

func (c *CheckDocker) run() {
	defer c.Client.Close()
	firstWait := lib.RandomStagger(c.Interval)
	next := time.After(firstWait)
	for {
		select {
		case <-next:
			c.check()
			next = time.After(c.Interval)
		case <-c.stop:
			return
		}
	}
}

func (c *CheckDocker) check() {
	var out string
	status, b, err := c.doCheck()
	if err != nil {
		c.Logger.Printf("[DEBUG] agent: Check %q: %s", c.CheckID, err)
		out = err.Error()
	} else {
		// out is already limited to CheckBufSize since we're getting a
		// limited buffer. So we don't need to truncate it just report
		// that it was truncated.
		out = string(b.Bytes())
		if int(b.TotalWritten()) > len(out) {
			out = fmt.Sprintf("Captured %d of %d bytes\n...\n%s", len(out), b.TotalWritten(), out)
		}
		c.Logger.Printf("[TRACE] agent: Check %q output: %s", c.CheckID, out)
	}

	if status == api.HealthCritical {
		c.Logger.Printf("[WARN] agent: Check %q is now critical", c.CheckID)
	}

	c.Notify.UpdateCheck(c.CheckID, status, out)
}

func (c *CheckDocker) doCheck() (string, *circbuf.Buffer, error) {
	var cmd []string
	if len(c.ScriptArgs) > 0 {
		cmd = c.ScriptArgs
	} else {
		cmd = []string{c.Shell, "-c", c.Script}
	}

	execID, err := c.Client.CreateExec(c.DockerContainerID, cmd)
	if err != nil {
		return api.HealthCritical, nil, err
	}

	buf, err := c.Client.StartExec(c.DockerContainerID, execID)
	if err != nil {
		return api.HealthCritical, nil, err
	}

	exitCode, err := c.Client.InspectExec(c.DockerContainerID, execID)
	if err != nil {
		return api.HealthCritical, nil, err
	}

	switch exitCode {
	case 0:
		return api.HealthPassing, buf, nil
	case 1:
		c.Logger.Printf("[DEBUG] Check %q failed with exit code: %d", c.CheckID, exitCode)
		return api.HealthWarning, buf, nil
	default:
		c.Logger.Printf("[DEBUG] Check %q failed with exit code: %d", c.CheckID, exitCode)
		return api.HealthCritical, buf, nil
	}
}
