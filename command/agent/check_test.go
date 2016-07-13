package agent

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul/consul/structs"
	"github.com/hashicorp/consul/testutil"
	"github.com/hashicorp/consul/types"
)

type MockNotify struct {
	state   map[types.CheckID]string
	updates map[types.CheckID]int
	output  map[types.CheckID]string
}

func (m *MockNotify) UpdateCheck(id types.CheckID, status, output string) {
	m.state[id] = status
	old := m.updates[id]
	m.updates[id] = old + 1
	m.output[id] = output
}

func expectStatus(t *testing.T, script, status string) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		Script:   script,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
		ReapLock: &sync.RWMutex{},
	}
	check.Start()
	defer check.Stop()

	testutil.WaitForResult(func() (bool, error) {
		// Should have at least 2 updates
		if mock.updates["foo"] < 2 {
			return false, fmt.Errorf("should have 2 updates %v", mock.updates)
		}

		if mock.state["foo"] != status {
			return false, fmt.Errorf("should be %v %v", status, mock.state)
		}

		return true, nil
	}, func(err error) {
		t.Fatalf("err: %s", err)
	})
}

func TestCheckMonitor_Passing(t *testing.T) {
	expectStatus(t, "exit 0", structs.HealthPassing)
}

func TestCheckMonitor_Warning(t *testing.T) {
	expectStatus(t, "exit 1", structs.HealthWarning)
}

func TestCheckMonitor_Critical(t *testing.T) {
	expectStatus(t, "exit 2", structs.HealthCritical)
}

func TestCheckMonitor_BadCmd(t *testing.T) {
	expectStatus(t, "foobarbaz", structs.HealthCritical)
}

func TestCheckMonitor_Timeout(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		Script:   "sleep 1 && exit 0",
		Interval: 10 * time.Millisecond,
		Timeout:  5 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
		ReapLock: &sync.RWMutex{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["foo"] < 2 {
		t.Fatalf("should have at least 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != "critical" {
		t.Fatalf("should be critical %v", mock.state)
	}
}

func TestCheckMonitor_RandomStagger(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		Script:   "exit 0",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
		ReapLock: &sync.RWMutex{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 1 update
	if mock.updates["foo"] < 1 {
		t.Fatalf("should have 1 or more updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthPassing {
		t.Fatalf("should be %v %v", structs.HealthPassing, mock.state)
	}
}

func TestCheckMonitor_LimitOutput(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckMonitor{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		Script:   "od -N 81920 /dev/urandom",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
		ReapLock: &sync.RWMutex{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(mock.output["foo"]) > CheckBufSize+100 {
		t.Fatalf("output size is too long")
	}
}

func TestCheckTTL(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckTTL{
		Notify:  mock,
		CheckID: types.CheckID("foo"),
		TTL:     100 * time.Millisecond,
		Logger:  log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)
	check.SetStatus(structs.HealthPassing, "test-output")

	if mock.updates["foo"] != 1 {
		t.Fatalf("should have 1 updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthPassing {
		t.Fatalf("should be passing %v", mock.state)
	}

	// Ensure we don't fail early
	time.Sleep(75 * time.Millisecond)
	if mock.updates["foo"] != 1 {
		t.Fatalf("should have 1 updates %v", mock.updates)
	}

	// Wait for the TTL to expire
	time.Sleep(75 * time.Millisecond)

	if mock.updates["foo"] != 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != structs.HealthCritical {
		t.Fatalf("should be critical %v", mock.state)
	}

	if !strings.Contains(mock.output["foo"], "test-output") {
		t.Fatalf("should have retained output %v", mock.output)
	}
}

func mockHTTPServer(responseCode int) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Body larger than 4k limit
		body := bytes.Repeat([]byte{'a'}, 2*CheckBufSize)
		w.WriteHeader(responseCode)
		w.Write(body)
		return
	})

	return httptest.NewServer(mux)
}

func expectHTTPStatus(t *testing.T, url string, status string) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckHTTP{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		HTTP:     url,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["foo"] < 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != status {
		t.Fatalf("should be %v %v", status, mock.state)
	}

	// Allow slightly more data than CheckBufSize, for the header
	if n := len(mock.output["foo"]); n > (CheckBufSize + 256) {
		t.Fatalf("output too long: %d (%d-byte limit)", n, CheckBufSize)
	}
}

func TestCheckHTTPCritical(t *testing.T) {
	// var server *httptest.Server

	server := mockHTTPServer(150)
	fmt.Println(server.URL)
	expectHTTPStatus(t, server.URL, structs.HealthCritical)
	server.Close()

	// 2xx - 1
	server = mockHTTPServer(199)
	expectHTTPStatus(t, server.URL, structs.HealthCritical)
	server.Close()

	// 2xx + 1
	server = mockHTTPServer(300)
	expectHTTPStatus(t, server.URL, structs.HealthCritical)
	server.Close()

	server = mockHTTPServer(400)
	expectHTTPStatus(t, server.URL, structs.HealthCritical)
	server.Close()

	server = mockHTTPServer(500)
	expectHTTPStatus(t, server.URL, structs.HealthCritical)
	server.Close()
}

func TestCheckHTTPPassing(t *testing.T) {
	var server *httptest.Server

	server = mockHTTPServer(200)
	expectHTTPStatus(t, server.URL, structs.HealthPassing)
	server.Close()

	server = mockHTTPServer(201)
	expectHTTPStatus(t, server.URL, structs.HealthPassing)
	server.Close()

	server = mockHTTPServer(250)
	expectHTTPStatus(t, server.URL, structs.HealthPassing)
	server.Close()

	server = mockHTTPServer(299)
	expectHTTPStatus(t, server.URL, structs.HealthPassing)
	server.Close()
}

func TestCheckHTTPWarning(t *testing.T) {
	server := mockHTTPServer(429)
	expectHTTPStatus(t, server.URL, structs.HealthWarning)
	server.Close()
}

func mockSlowHTTPServer(responseCode int, sleep time.Duration) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(sleep)
		w.WriteHeader(responseCode)
		return
	})

	return httptest.NewServer(mux)
}

func TestCheckHTTPTimeout(t *testing.T) {
	server := mockSlowHTTPServer(200, 10*time.Millisecond)
	defer server.Close()

	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}

	check := &CheckHTTP{
		Notify:   mock,
		CheckID:  types.CheckID("bar"),
		HTTP:     server.URL,
		Timeout:  5 * time.Millisecond,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}

	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["bar"] < 2 {
		t.Fatalf("should have at least 2 updates %v", mock.updates)
	}

	if mock.state["bar"] != structs.HealthCritical {
		t.Fatalf("should be critical %v", mock.state)
	}
}

func TestCheckHTTP_disablesKeepAlives(t *testing.T) {
	check := &CheckHTTP{
		CheckID:  types.CheckID("foo"),
		HTTP:     "http://foo.bar/baz",
		Interval: 10 * time.Second,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).DisableKeepAlives {
		t.Fatalf("should have disabled keepalives")
	}
}

func mockTCPServer(network string) net.Listener {
	var (
		addr string
	)

	if network == `tcp6` {
		addr = `[::1]:0`
	} else {
		addr = `127.0.0.1:0`
	}

	listener, err := net.Listen(network, addr)
	if err != nil {
		panic(err)
	}

	return listener
}

func expectTCPStatus(t *testing.T, tcp string, status string) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckTCP{
		Notify:   mock,
		CheckID:  types.CheckID("foo"),
		TCP:      tcp,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(os.Stderr, "", log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["foo"] < 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != status {
		t.Fatalf("should be %v %v", status, mock.state)
	}
}

func TestCheckTCPCritical(t *testing.T) {
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, `127.0.0.1:0`, structs.HealthCritical)
	tcpServer.Close()
}

func TestCheckTCPPassing(t *testing.T) {
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, tcpServer.Addr().String(), structs.HealthPassing)
	tcpServer.Close()

	tcpServer = mockTCPServer(`tcp6`)
	expectTCPStatus(t, tcpServer.Addr().String(), structs.HealthPassing)
	tcpServer.Close()
}

// A fake docker client to test happy path scenario
type fakeDockerClientWithNoErrors struct {
}

func (d *fakeDockerClientWithNoErrors) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithNoErrors) StartExec(id string, opts docker.StartExecOptions) error {
	fmt.Fprint(opts.OutputStream, "output")
	return nil
}

func (d *fakeDockerClientWithNoErrors) InspectExec(id string) (*docker.ExecInspect, error) {
	return &docker.ExecInspect{
		ID:       "123",
		ExitCode: 0,
	}, nil
}

// A fake docker client to test truncation of output
type fakeDockerClientWithLongOutput struct {
}

func (d *fakeDockerClientWithLongOutput) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithLongOutput) StartExec(id string, opts docker.StartExecOptions) error {
	b, _ := exec.Command("od", "-N", "81920", "/dev/urandom").Output()
	fmt.Fprint(opts.OutputStream, string(b))
	return nil
}

func (d *fakeDockerClientWithLongOutput) InspectExec(id string) (*docker.ExecInspect, error) {
	return &docker.ExecInspect{
		ID:       "123",
		ExitCode: 0,
	}, nil
}

// A fake docker client to test non-zero exit codes from exec invocation
type fakeDockerClientWithExecNonZeroExitCode struct {
}

func (d *fakeDockerClientWithExecNonZeroExitCode) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithExecNonZeroExitCode) StartExec(id string, opts docker.StartExecOptions) error {
	return nil
}

func (d *fakeDockerClientWithExecNonZeroExitCode) InspectExec(id string) (*docker.ExecInspect, error) {
	return &docker.ExecInspect{
		ID:       "123",
		ExitCode: 127,
	}, nil
}

// A fake docker client to test exit code which result into Warning
type fakeDockerClientWithExecExitCodeOne struct {
}

func (d *fakeDockerClientWithExecExitCodeOne) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithExecExitCodeOne) StartExec(id string, opts docker.StartExecOptions) error {
	fmt.Fprint(opts.OutputStream, "output")
	return nil
}

func (d *fakeDockerClientWithExecExitCodeOne) InspectExec(id string) (*docker.ExecInspect, error) {
	return &docker.ExecInspect{
		ID:       "123",
		ExitCode: 1,
	}, nil
}

// A fake docker client to simulate create exec failing
type fakeDockerClientWithCreateExecFailure struct {
}

func (d *fakeDockerClientWithCreateExecFailure) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return nil, errors.New("Exec Creation Failed")
}

func (d *fakeDockerClientWithCreateExecFailure) StartExec(id string, opts docker.StartExecOptions) error {
	return errors.New("Exec doesn't exist")
}

func (d *fakeDockerClientWithCreateExecFailure) InspectExec(id string) (*docker.ExecInspect, error) {
	return nil, errors.New("Exec doesn't exist")
}

// A fake docker client to simulate start exec failing
type fakeDockerClientWithStartExecFailure struct {
}

func (d *fakeDockerClientWithStartExecFailure) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithStartExecFailure) StartExec(id string, opts docker.StartExecOptions) error {
	return errors.New("Couldn't Start Exec")
}

func (d *fakeDockerClientWithStartExecFailure) InspectExec(id string) (*docker.ExecInspect, error) {
	return nil, errors.New("Exec doesn't exist")
}

// A fake docker client to test exec info query failures
type fakeDockerClientWithExecInfoErrors struct {
}

func (d *fakeDockerClientWithExecInfoErrors) CreateExec(opts docker.CreateExecOptions) (*docker.Exec, error) {
	return &docker.Exec{ID: "123"}, nil
}

func (d *fakeDockerClientWithExecInfoErrors) StartExec(id string, opts docker.StartExecOptions) error {
	return nil
}

func (d *fakeDockerClientWithExecInfoErrors) InspectExec(id string) (*docker.ExecInspect, error) {
	return nil, errors.New("Unable to query exec info")
}

func expectDockerCheckStatus(t *testing.T, dockerClient DockerClient, status string, output string) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckDocker{
		Notify:            mock,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Shell:             "/bin/sh",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(os.Stderr, "", log.LstdFlags),
		dockerClient:      dockerClient,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if mock.updates["foo"] < 2 {
		t.Fatalf("should have 2 updates %v", mock.updates)
	}

	if mock.state["foo"] != status {
		t.Fatalf("should be %v %v", status, mock.state)
	}

	if mock.output["foo"] != output {
		t.Fatalf("should be %v %v", output, mock.output)
	}
}

func TestDockerCheckWhenExecReturnsSuccessExitCode(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithNoErrors{}, structs.HealthPassing, "output")
}

func TestDockerCheckWhenExecCreationFails(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithCreateExecFailure{}, structs.HealthCritical, "Unable to create Exec, error: Exec Creation Failed")
}

func TestDockerCheckWhenExitCodeIsNonZero(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithExecNonZeroExitCode{}, structs.HealthCritical, "")
}

func TestDockerCheckWhenExitCodeIsone(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithExecExitCodeOne{}, structs.HealthWarning, "output")
}

func TestDockerCheckWhenExecStartFails(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithStartExecFailure{}, structs.HealthCritical, "Unable to start Exec: Couldn't Start Exec")
}

func TestDockerCheckWhenExecInfoFails(t *testing.T) {
	expectDockerCheckStatus(t, &fakeDockerClientWithExecInfoErrors{}, structs.HealthCritical, "Unable to inspect Exec: Unable to query exec info")
}

func TestDockerCheckDefaultToSh(t *testing.T) {
	os.Setenv("SHELL", "")
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckDocker{
		Notify:            mock,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(os.Stderr, "", log.LstdFlags),
		dockerClient:      &fakeDockerClientWithNoErrors{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)
	if check.Shell != "/bin/sh" {
		t.Fatalf("Shell should be: %v , actual: %v", "/bin/sh", check.Shell)
	}
}

func TestDockerCheckUseShellFromEnv(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	os.Setenv("SHELL", "/bin/bash")
	check := &CheckDocker{
		Notify:            mock,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(os.Stderr, "", log.LstdFlags),
		dockerClient:      &fakeDockerClientWithNoErrors{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)
	if check.Shell != "/bin/bash" {
		t.Fatalf("Shell should be: %v , actual: %v", "/bin/bash", check.Shell)
	}
	os.Setenv("SHELL", "")
}

func TestDockerCheckTruncateOutput(t *testing.T) {
	mock := &MockNotify{
		state:   make(map[types.CheckID]string),
		updates: make(map[types.CheckID]int),
		output:  make(map[types.CheckID]string),
	}
	check := &CheckDocker{
		Notify:            mock,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Shell:             "/bin/sh",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(os.Stderr, "", log.LstdFlags),
		dockerClient:      &fakeDockerClientWithLongOutput{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(mock.output["foo"]) > CheckBufSize+100 {
		t.Fatalf("output size is too long")
	}

}
