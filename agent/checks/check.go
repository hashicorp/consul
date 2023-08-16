// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: BUSL-1.1

package checks

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
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

	http2 "golang.org/x/net/http2"

	"github.com/hashicorp/consul/agent/structs"
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
	RPC(ctx context.Context, method string, args interface{}, reply interface{}) error
}

// CheckNotifier interface is used by the CheckMonitor
// to notify when a check has a status update. The update
// should take care to be idempotent.
type CheckNotifier interface {
	UpdateCheck(checkID structs.CheckID, status, output string)
	// ServiceExists return true if the given service does exists
	ServiceExists(serviceID structs.ServiceID) bool
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
	CheckID          structs.CheckID
	ServiceID        structs.ServiceID
	HTTP             string
	Header           map[string][]string
	Method           string
	Body             string
	Interval         time.Duration
	Timeout          time.Duration
	Logger           hclog.Logger
	TLSClientConfig  *tls.Config
	OutputMaxSize    int
	StatusHandler    *StatusHandler
	DisableRedirects bool

	httpClient *http.Client
	stop       bool
	stopCh     chan struct{}
	stopLock   sync.Mutex
	stopWg     sync.WaitGroup

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
		if c.DisableRedirects {
			c.httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
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
	c.stopWg.Add(1)
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

	// Wait for the c.run() goroutine to complete before returning.
	c.stopWg.Wait()
}

// run is invoked by a goroutine to run until Stop() is called
func (c *CheckHTTP) run() {
	defer c.stopWg.Done()
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

type CheckH2PING struct {
	CheckID         structs.CheckID
	ServiceID       structs.ServiceID
	H2PING          string
	Interval        time.Duration
	Timeout         time.Duration
	Logger          hclog.Logger
	TLSClientConfig *tls.Config
	StatusHandler   *StatusHandler

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
	stopWg   sync.WaitGroup
}

func shutdownHTTP2ClientConn(clientConn *http2.ClientConn, timeout time.Duration, checkIDString string, logger hclog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout/2)
	defer cancel()
	err := clientConn.Shutdown(ctx)
	if err != nil {
		logger.Warn("Shutdown of H2Ping check client connection gave an error",
			"check", checkIDString,
			"error", err)
	}
}

func (c *CheckH2PING) check() {
	t := &http2.Transport{}
	var dialFunc func(ctx context.Context, network, address string, tlscfg *tls.Config) (net.Conn, error)
	if c.TLSClientConfig != nil {
		t.TLSClientConfig = c.TLSClientConfig
		dialFunc = func(ctx context.Context, network, address string, tlscfg *tls.Config) (net.Conn, error) {
			dialer := &tls.Dialer{Config: tlscfg}
			return dialer.DialContext(ctx, network, address)
		}
	} else {
		t.AllowHTTP = true
		dialFunc = func(ctx context.Context, network, address string, tlscfg *tls.Config) (net.Conn, error) {
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, network, address)
		}
	}
	target := c.H2PING
	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()
	conn, err := dialFunc(ctx, "tcp", target, c.TLSClientConfig)
	if err != nil {
		message := fmt.Sprintf("Failed to dial to %s: %s", target, err)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, message)
		return
	}
	defer conn.Close()
	clientConn, err := t.NewClientConn(conn)
	if err != nil {
		message := fmt.Sprintf("Failed to create client connection %s", err)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, message)
		return
	}
	defer shutdownHTTP2ClientConn(clientConn, c.Timeout, c.CheckID.String(), c.Logger)
	err = clientConn.Ping(ctx)
	if err == nil {
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, "HTTP2 ping was successful")
	} else {
		message := fmt.Sprintf("HTTP2 ping failed: %s", err)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, message)
	}
}

// Stop is used to stop an H2PING check.
func (c *CheckH2PING) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
	c.stopWg.Wait()
}

func (c *CheckH2PING) run() {
	defer c.stopWg.Done()
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

func (c *CheckH2PING) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	c.stop = false
	c.stopCh = make(chan struct{})
	c.stopWg.Add(1)
	go c.run()
}

// CheckTCP is used to periodically make a TCP connection to determine the
// health of a given check.
// The check is passing if the connection succeeds
// The check is critical if the connection returns an error
// Supports failures_before_critical and success_before_passing.
type CheckTCP struct {
	CheckID         structs.CheckID
	ServiceID       structs.ServiceID
	TCP             string
	Interval        time.Duration
	Timeout         time.Duration
	Logger          hclog.Logger
	TLSClientConfig *tls.Config
	StatusHandler   *StatusHandler

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
			Timeout: 10 * time.Second,
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
	logAndUpdate := func(checkType string, err error) {
		msg := fmt.Sprintf("%s connect %s: ", checkType, c.TCP)
		if err != nil {
			c.Logger.Warn(msg+"failed", "check", c.CheckID.String(), "error", err)
			c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
			return
		}
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, msg+"Success")
	}

	if c.TLSClientConfig == nil {
		if conn, err := c.dialer.Dial(`tcp`, c.TCP); err == nil {
			defer conn.Close()
			logAndUpdate("TCP", err)
		}
	} else {
		if tlsConn, err := tls.DialWithDialer(c.dialer, `tcp`, c.TCP, c.TLSClientConfig); err == nil {
			defer tlsConn.Close()
			logAndUpdate("TCP+TLS", err)
		}
	}
	}
}

// CheckUDP is used to periodically send a UDP datagram to determine the health of a given check.
// The check is passing if the connection succeeds, the response is bytes.Equal to the bytes passed
// in or if the error returned is a timeout error
// The check is critical if: the connection succeeds but the response is not equal to the bytes passed in,
// the connection succeeds but the error returned is not a timeout error or the connection fails
type CheckUDP struct {
	CheckID       structs.CheckID
	ServiceID     structs.ServiceID
	UDP           string
	Message       string
	Interval      time.Duration
	Timeout       time.Duration
	Logger        hclog.Logger
	StatusHandler *StatusHandler

	dialer   *net.Dialer
	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
}

func (c *CheckUDP) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()

	if c.dialer == nil {
		// Create the socket dialer
		c.dialer = &net.Dialer{
			Timeout: 10 * time.Second,
		}
		if c.Timeout > 0 {
			c.dialer.Timeout = c.Timeout
		}
	}

	c.stop = false
	c.stopCh = make(chan struct{})
	go c.run()
}

func (c *CheckUDP) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}
}

func (c *CheckUDP) run() {
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

func (c *CheckUDP) check() {

	conn, err := c.dialer.Dial(`udp`, c.UDP)

	if err != nil {
		if e, ok := err.(net.Error); ok && e.Timeout() {
			c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("UDP connect %s: Success", c.UDP))
			return
		} else {
			c.Logger.Warn("Check socket connection failed",
				"check", c.CheckID.String(),
				"error", err,
			)
			c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
			return
		}
	}
	defer conn.Close()

	n, err := fmt.Fprintf(conn, c.Message)
	if err != nil {
		c.Logger.Warn("Check socket write failed",
			"check", c.CheckID.String(),
			"error", err,
		)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	if n != len(c.Message) {
		c.Logger.Warn("Check socket short write",
			"check", c.CheckID.String(),
			"error", err,
		)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}

	if err != nil {
		c.Logger.Warn("Check socket write failed",
			"check", c.CheckID.String(),
			"error", err,
		)
		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
		return
	}
	_, err = bufio.NewReader(conn).Read(make([]byte, 1))
	if err != nil {
		if strings.Contains(err.Error(), "i/o timeout") {
			c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("UDP connect %s: Success", c.UDP))
			return
		} else {
			c.Logger.Warn("Check socket read failed",
				"check", c.CheckID.String(),
				"error", err,
			)
			c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, err.Error())
			return
		}
	} else if err == nil {
		c.StatusHandler.updateCheck(c.CheckID, api.HealthPassing, fmt.Sprintf("UDP connect %s: Success", c.UDP))
	}
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
		c.Logger = hclog.New(&hclog.LoggerOptions{Output: io.Discard})
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

type CheckOSService struct {
	CheckID       structs.CheckID
	ServiceID     structs.ServiceID
	OSService     string
	Interval      time.Duration
	Timeout       time.Duration
	Logger        hclog.Logger
	StatusHandler *StatusHandler
	Client        *OSServiceClient

	stop     bool
	stopCh   chan struct{}
	stopLock sync.Mutex
	stopWg   sync.WaitGroup
}

func (c *CheckOSService) CheckType() structs.CheckType {
	return structs.CheckType{
		CheckID:   c.CheckID.ID,
		OSService: c.OSService,
		Interval:  c.Interval,
		Timeout:   c.Timeout,
	}
}

func (c *CheckOSService) Start() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	c.stop = false
	c.stopCh = make(chan struct{})
	c.stopWg.Add(1)
	go c.run()
}

func (c *CheckOSService) Stop() {
	c.stopLock.Lock()
	defer c.stopLock.Unlock()
	if !c.stop {
		c.stop = true
		close(c.stopCh)
	}

	// Wait for the c.run() goroutine to complete before returning.
	c.stopWg.Wait()
}

func (c *CheckOSService) run() {
	defer c.stopWg.Done()
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

func (c *CheckOSService) doCheck() (string, error) {
	err := c.Client.Check(c.OSService)
	if err == nil {
		return api.HealthPassing, nil
	}
	if errors.Is(err, ErrOSServiceStatusCritical) {
		return api.HealthCritical, err
	}

	return api.HealthWarning, err
}

func (c *CheckOSService) check() {
	var out string
	var status string
	var err error

	waitCh := make(chan error, 1)
	go func() {
		status, err = c.doCheck()
		waitCh <- err
	}()

	timeout := 30 * time.Second
	if c.Timeout > 0 {
		timeout = c.Timeout
	}
	select {
	case <-time.After(timeout):
		msg := fmt.Sprintf("Timed out (%s) running check", timeout.String())
		c.Logger.Warn("Timed out running check",
			"check", c.CheckID.String(),
			"timeout", timeout.String(),
		)

		c.StatusHandler.updateCheck(c.CheckID, api.HealthCritical, msg)

		// Now wait for the process to exit so we never start another
		// instance concurrently.
		<-waitCh
		return

	case err = <-waitCh:
		// The process returned before the timeout, proceed normally
	}

	out = fmt.Sprintf("Service \"%s\" is healthy", c.OSService)
	if err != nil {
		c.Logger.Debug("Check failed",
			"check", c.CheckID.String(),
			"error", err,
		)
		out = err.Error()
	}
	c.StatusHandler.updateCheck(c.CheckID, status, out)
}

// StatusHandler keep tracks of successive error/success counts and ensures
// that status can be set to critical/passing only once the successive number of event
// reaches the given threshold.
type StatusHandler struct {
	inner                  CheckNotifier
	logger                 hclog.Logger
	successBeforePassing   int
	successCounter         int
	failuresBeforeWarning  int
	failuresBeforeCritical int
	failuresCounter        int
}

// NewStatusHandler set counters values to threshold in order to immediatly update status after first check.
func NewStatusHandler(inner CheckNotifier, logger hclog.Logger, successBeforePassing, failuresBeforeWarning, failuresBeforeCritical int) *StatusHandler {
	return &StatusHandler{
		logger:                 logger,
		inner:                  inner,
		successBeforePassing:   successBeforePassing,
		successCounter:         successBeforePassing,
		failuresBeforeWarning:  failuresBeforeWarning,
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
		// Defaults to same value as failuresBeforeCritical if not set.
		if s.failuresCounter >= s.failuresBeforeWarning {
			s.logger.Warn("Check is now warning", "check", checkID.String())
			s.inner.UpdateCheck(checkID, api.HealthWarning, output)
			return
		}
		s.logger.Warn("Check failed but has not reached warning/failure threshold",
			"check", checkID.String(),
			"status", status,
			"failure_count", s.failuresCounter,
			"warning_threshold", s.failuresBeforeWarning,
			"failure_threshold", s.failuresBeforeCritical,
		)
	}
}
