package checks

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	osexec "os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/consul/agent/structs"
	"github.com/hashicorp/consul/sdk/testutil"
	"github.com/hashicorp/go-hclog"

	"github.com/armon/circbuf"
	"github.com/hashicorp/consul/agent/exec"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/lib"
	"github.com/hashicorp/go-cleanhttp"
)

const (
	// MinInterval is the minimal interval between
	// two checks. Do not allow for a interval below this value.
	// Otherwise we risk fork bombing a system.
	MinInterval = time.Second

	// DefaultBufSize is the maximum size of the captured
	// check output by default. Prevents an enormous buffer
	// from being captured
	DefaultBufSize = 4 * 1024 // 4KB

	// UserAgent is the value of the User-Agent header
	// for HTTP health checks.
	UserAgent = "Consul Health Check"
)

// RPC is an interface that an RPC client must implement. This is a helper
// interface that is implemented by the agent delegate for checks that need
// to make RPC calls.
type RPC interface {
	RPC(method string, args interface{}, reply interface{}) error
}

// CheckNotifier interface is used by the CheckMonitor
// to notify when a check has a status update. The update
// should take care to be idempotent.
type CheckNotifier interface {
	UpdateCheck(checkID structs.CheckID, status, output string)
}

// CheckMonitor is used to periodically invoke a script to
// determine the health of a given check. It is compatible with
// nagios plugins and expects the output in the same format.
// Supports failures_before_critical and success_before_passing.
type CheckMonitor struct {
	Notify        CheckNotifier
	CheckID       structs.CheckID
	ServiceID     structs.ServiceID
	Script        string
	ScriptArgs    []string
	Interval      time.Duration
	Timeout       time.Duration
	Logger        hclog.Logger
	OutputMaxSize int
	StatusHandler *StatusHandler

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
		c.Logger.Error("Check failed to setup",
			"check", c.CheckID.String(),
			"error", err,
		)
		c.Notify.UpdateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	// Collect the output
	output, _ := circbuf.NewBuffer(int64(c.OutputMaxSize))
	cmd.Stdout = output
	cmd.Stderr = output
	exec.SetSysProcAttr(cmd)

	truncateAndLogOutput := func() string {
		outputStr := string(output.Bytes())
		if output.TotalWritten() > output.Size() {
			outputStr = fmt.Sprintf("Captured %d of %d bytes\n...\n%s",
				output.Size(), output.TotalWritten(), outputStr)
		}
		c.Logger.Trace("Check output",
			"check", c.CheckID.String(),
			"output", outputStr,
		)
		return outputStr
	}

	// Start the check
	if err := cmd.Start(); err != nil {
		c.Logger.Error("Check failed to invoke",
			"check", c.CheckID.String(),
			"error", err,
		)
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
			c.Logger.Warn("Check failed to kill after timeout",
				"check", c.CheckID.String(),
				"error", err,
			)
		}

		msg := fmt.Sprintf("Timed out (%s) running check", timeout.String())
		c.Logger.Warn("Timed out running check",
			"check", c.CheckID.String(),
			"timeout", timeout.String(),
		)

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
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, outputStr)
		return
	}

	// If the exit code is 1, set check as warning
	exitErr, ok := err.(*osexec.ExitError)
	if ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			code := status.ExitStatus()
			if code == 1 {
				c.StatusHandler.updateCheck(c.CheckID, api.HealthWarning, outputStr)
				return
			}
		}
	}

	// Set the health as critical
	c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, outputStr)
}

// CheckTTL is used to apply a TTL to check status,
// and enables clients to set the status of a check
// but upon the TTL expiring, the check status is
// automatically set to critical.
type CheckTTL struct {
	Notify    CheckNotifier
	CheckID   structs.CheckID
	ServiceID structs.ServiceID
	TTL       time.Duration
	Logger    hclog.Logger

	timer *time.Timer

	lastOutput     string
	lastOutputLock sync.RWMutex

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex

	OutputMaxSize int
}

// Start is used to start a check ttl, runs until Stop()
func (c *CheckTTL) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if c.OutputMaxSize < 1 {
		c.OutputMaxSize = DefaultBufSize
	}
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
			c.Logger.Warn("Check missed TTL, is now critical",
				"check", c.CheckID.String(),
			)
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
// output is returned (might be truncated)
func (c *CheckTTL) SetStatus(status, output string) string {
	c.Logger.Debug("Check status updated",
		"check", c.CheckID.String(),
		"status", status,
	)
	total := len(output)
	if total > c.OutputMaxSize {
		output = fmt.Sprintf("%s ... (captured %d of %d bytes)",
			output[:c.OutputMaxSize], c.OutputMaxSize, total)
	}
	c.Notify.UpdateCheck(c.CheckID, status, output)
	// Store the last output so we can retain it if the TTL expires.
	c.lastOutputLock.Lock()
	c.lastOutput = output
	c.lastOutputLock.Unlock()

	c.timer.Reset(c.TTL)
	return output
}

// CheckHTTP is used to periodically make an HTTP request to
// determine the health of a given check.
// The check is passing if the response code is 2XX.
// The check is warning if the response code is 429.
// The check is critical if the response code is anything else
// or if the request returns an error
// Supports failures_before_critical and success_before_passing.
type CheckHTTP struct {
	CheckID         structs.CheckID
	ServiceID       structs.ServiceID
	HTTP            string
	Header          map[string][]string
	Method          string
	Body            string
	Interval        time.Duration
	Timeout         time.Duration
	Logger          hclog.Logger
	TLSClientConfig *tls.Config
	OutputMaxSize   int
	StatusHandler   *StatusHandler

	httpClient *http.Client
	stop       bool
	stopCh     chan struct{}
	stopLock   sync.Mutex

	// Set if checks are exposed through Connect proxies
	// If set, this is the target of check()
	ProxyHTTP string
}

func (c *CheckHTTP) CheckType() structs.CheckType {
	return structs.CheckType{
		CheckID:       c.CheckID.ID,
		HTTP:          c.HTTP,
		Method:        c.Method,
		Body:          c.Body,
		Header:        c.Header,
		Interval:      c.Interval,
		ProxyHTTP:     c.ProxyHTTP,
		Timeout:       c.Timeout,
		OutputMaxSize: c.OutputMaxSize,
	}
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
		if c.Timeout > 0 {
			c.httpClient.Timeout = c.Timeout
		}

		if c.OutputMaxSize < 1 {
			c.OutputMaxSize = DefaultBufSize
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

	target := c.HTTP
	if c.ProxyHTTP != "" {
		target = c.ProxyHTTP
	}

	bodyReader := strings.NewReader(c.Body)
	req, err := http.NewRequest(method, target, bodyReader)
	if err != nil {
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
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
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}
	defer resp.Body.Close()

	// Read the response into a circular buffer to limit the size
	output, _ := circbuf.NewBuffer(int64(c.OutputMaxSize))
	if _, err := io.Copy(output, resp.Body); err != nil {
		c.Logger.Warn("Check error while reading body",
			"check", c.CheckID.String(),
			"error", err,
		)
	}

	// Format the response body
	result := fmt.Sprintf("HTTP %s %s: %s Output: %s", method, target, resp.Status, output.String())

	if resp.StatusCode >= 200 && resp.StatusCode <= 299 {
		// PASSING (2xx)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, result)
	} else if resp.StatusCode == 429 {
		// WARNING
		// 429 Too Many Requests (RFC 6585)
		// The user has sent too many requests in a given amount of time.
		c.StatusHandler.updateCheck(c.CheckID, api.HealthWarning, result)
	} else {
		// CRITICAL
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, result)
	}
}

// CheckTCP is used to periodically make an TCP/UDP connection to
// determine the health of a given check.
// The check is passing if the connection succeeds
// The check is critical if the connection returns an error
// Supports failures_before_critical and success_before_passing.
type CheckTCP struct {
	CheckID       structs.CheckID
	ServiceID     structs.ServiceID
	TCP           string
	Interval      time.Duration
	Timeout       time.Duration
	Logger        hclog.Logger
	StatusHandler *StatusHandler

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
		c.dialer = &net.Dialer{
			Timeout:   10 * time.Second,
			DualStack: true,
		}
		if c.Timeout > 0 {
			c.dialer.Timeout = c.Timeout
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
		c.Logger.Warn("Check socket connection failed",
			"check", c.CheckID.String(),
			"error", err,
		)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}
	conn.Close()
	c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("TCP connect %s: Success", c.TCP))
}

// CheckDocker is used to periodically invoke a script to
// determine the health of an application running inside a
// Docker Container. We assume that the script is compatible
// with nagios plugins and expects the output in the same format.
// Supports failures_before_critical and success_before_passing.
type CheckDocker struct {
	CheckID           structs.CheckID
	ServiceID         structs.ServiceID
	Script            string
	ScriptArgs        []string
	DockerContainerID string
	Shell             string
	Interval          time.Duration
	Logger            hclog.Logger
	Client            *DockerClient
	StatusHandler     *StatusHandler

	stop chan struct{}
}

func (c *CheckDocker) Start() {
	if c.stop != nil {
		panic("Docker check already started")
	}

	if c.Logger == nil {
		c.Logger = testutil.NewDiscardLogger()
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
		c.Logger.Debug("Check failed",
			"check", c.CheckID.String(),
			"error", err,
		)
		out = err.Error()
	} else {
		// out is already limited to CheckBufSize since we're getting a
		// limited buffer. So we don't need to truncate it just report
		// that it was truncated.
		out = string(b.Bytes())
		if int(b.TotalWritten()) > len(out) {
			out = fmt.Sprintf("Captured %d of %d bytes\n...\n%s", len(out), b.TotalWritten(), out)
		}
		c.Logger.Trace("Check output",
			"check", c.CheckID.String(),
			"output", out,
		)
	}
	c.StatusHandler.updateCheck(c.CheckID, status, out)
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
		c.Logger.Debug("Check failed",
			"check", c.CheckID.String(),
			"exit_code", exitCode,
		)
		return api.HealthWarning, buf, nil
	default:
		c.Logger.Debug("Check failed",
			"check", c.CheckID.String(),
			"exit_code", exitCode,
		)
		return api.HealthCritical, buf, nil
	}
}

// CheckGRPC is used to periodically send request to a gRPC server
// application that implements gRPC health-checking protocol.
// The check is passing if returned status is SERVING.
// The check is critical if connection fails or returned status is
// not SERVING.
// Supports failures_before_critical and success_before_passing.
type CheckGRPC struct {
	CheckID         structs.CheckID
	ServiceID       structs.ServiceID
	GRPC            string
	Interval        time.Duration
	Timeout         time.Duration
	TLSClientConfig *tls.Config
	Logger          hclog.Logger
	StatusHandler   *StatusHandler

	probe    *GrpcHealthProbe
	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex

	// Set if checks are exposed through Connect proxies
	// If set, this is the target of check()
	ProxyGRPC string
}

func (c *CheckGRPC) CheckType() structs.CheckType {
	return structs.CheckType{
		CheckID:   c.CheckID.ID,
		GRPC:      c.GRPC,
		ProxyGRPC: c.ProxyGRPC,
		Interval:  c.Interval,
		Timeout:   c.Timeout,
	}
}

func (c *CheckGRPC) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	timeout := 10 * time.Second
	if c.Timeout > 0 {
		timeout = c.Timeout
	}
	c.probe = NewGrpcHealthProbe(c.GRPC, timeout, c.TLSClientConfig)
	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run()
}

func (c *CheckGRPC) run() {
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

func (c *CheckGRPC) check() {
	target := c.GRPC
	if c.ProxyGRPC != "" {
		target = c.ProxyGRPC
	}

	err := c.probe.Check(target)
	if err != nil {
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
	} else {
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("gRPC check %s: success", target))
	}
}

func (c *CheckGRPC) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

// StatusHandler keep tracks of successive error/success counts and ensures
// that status can be set to critical/passing only once the successive number of event
// reaches the given threshold.
type StatusHandler struct {
	inner                  CheckNotifier
	logger                 hclog.Logger
	successBeforePassing   int
	successCounter         int
	failuresBeforeCritical int
	failuresCounter        int
}

// NewStatusHandler set counters values to threshold in order to immediatly update status after first check.
func NewStatusHandler(inner CheckNotifier, logger hclog.Logger, successBeforePassing, failuresBeforeCritical int) *StatusHandler {
	return &StatusHandler{
		logger:                 logger,
		inner:                  inner,
		successBeforePassing:   successBeforePassing,
		successCounter:         successBeforePassing,
		failuresBeforeCritical: failuresBeforeCritical,
		failuresCounter:        failuresBeforeCritical,
	}
}

func (s *StatusHandler) updateCheck(checkID structs.CheckID, status, output string) {

	if status == api.HealthPassing || status == api.HealthWarning {
		s.successCounter++
		s.failuresCounter = 0
		if s.successCounter >= s.successBeforePassing {
			s.logger.Debug("Check status updated",
				"check", checkID.String(),
				"status", status,
			)
			s.inner.UpdateCheck(checkID, status, output)
			return
		}
		s.logger.Warn("Check passed but has not reached success threshold",
			"check", checkID.String(),
			"status", status,
			"success_count", s.successCounter,
			"success_threshold", s.successBeforePassing,
		)
	} else {
		s.failuresCounter++
		s.successCounter = 0
		if s.failuresCounter >= s.failuresBeforeCritical {
			s.logger.Warn("Check is now critical", "check", checkID.String())
			s.inner.UpdateCheck(checkID, status, output)
			return
		}
		s.logger.Warn("Check failed but has not reached failure threshold",
			"check", checkID.String(),
			"status", status,
			"failure_count", s.failuresCounter,
			"failure_threshold", s.failuresBeforeCritical,
		)
	}
}
