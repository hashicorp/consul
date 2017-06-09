package agent

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/consul/agent/mock"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/testutil/retry"
	"github.com/hashicorp/consul/types"
)

func expectStatus(t *testing.T, script, status string) {
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		Script:   script,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates("foo"), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State("foo"), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckMonitor_Passing(t *testing.T) {
	t.Parallel()
	expectStatus(t, "exit 0", api.HealthPassing)
}

func TestCheckMonitor_Warning(t *testing.T) {
	t.Parallel()
	expectStatus(t, "exit 1", api.HealthWarning)
}

func TestCheckMonitor_Critical(t *testing.T) {
	t.Parallel()
	expectStatus(t, "exit 2", api.HealthCritical)
}

func TestCheckMonitor_BadCmd(t *testing.T) {
	t.Parallel()
	expectStatus(t, "foobarbaz", api.HealthCritical)
}

func TestCheckMonitor_Timeout(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		Script:   "sleep 1 && exit 0",
		Interval: 10 * time.Millisecond,
		Timeout:  5 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(150 * time.Millisecond)

	// Should have at least 2 updates
	if notif.Updates("foo") < 2 {
		t.Fatalf("should have at least 2 updates %v", notif.UpdatesMap())
	}
	if notif.State("foo") != "critical" {
		t.Fatalf("should be critical %v", notif.StateMap())
	}
}

func TestCheckMonitor_RandomStagger(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		Script:   "exit 0",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 1 update
	if notif.Updates("foo") < 1 {
		t.Fatalf("should have 1 or more updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthPassing {
		t.Fatalf("should be %v %v", api.HealthPassing, notif.StateMap())
	}
}

func TestCheckMonitor_LimitOutput(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckMonitor{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		Script:   "od -N 81920 /dev/urandom",
		Interval: 25 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(notif.Output("foo")) > CheckBufSize+100 {
		t.Fatalf("output size is too long")
	}
}

func TestCheckTTL(t *testing.T) {
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckTTL{
		Notify:  notif,
		CheckID: types.CheckID("foo"),
		TTL:     100 * time.Millisecond,
		Logger:  log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)
	check.SetStatus(api.HealthPassing, "test-output")

	if notif.Updates("foo") != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthPassing {
		t.Fatalf("should be passing %v", notif.StateMap())
	}

	// Ensure we don't fail early
	time.Sleep(75 * time.Millisecond)
	if notif.Updates("foo") != 1 {
		t.Fatalf("should have 1 updates %v", notif.UpdatesMap())
	}

	// Wait for the TTL to expire
	time.Sleep(75 * time.Millisecond)

	if notif.Updates("foo") != 2 {
		t.Fatalf("should have 2 updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != api.HealthCritical {
		t.Fatalf("should be critical %v", notif.StateMap())
	}

	if !strings.Contains(notif.Output("foo"), "test-output") {
		t.Fatalf("should have retained output %v", notif.OutputMap())
	}
}

func TestCheckHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		code   int
		method string
		header http.Header
		status string
	}{
		// passing
		{code: 200, status: api.HealthPassing},
		{code: 201, status: api.HealthPassing},
		{code: 250, status: api.HealthPassing},
		{code: 299, status: api.HealthPassing},

		// warning
		{code: 429, status: api.HealthWarning},

		// critical
		{code: 150, status: api.HealthCritical},
		{code: 199, status: api.HealthCritical},
		{code: 300, status: api.HealthCritical},
		{code: 400, status: api.HealthCritical},
		{code: 500, status: api.HealthCritical},

		// custom method
		{desc: "custom method GET", code: 200, method: "GET", status: api.HealthPassing},
		{desc: "custom method POST", code: 200, header: http.Header{"Content-Length": []string{"0"}}, method: "POST", status: api.HealthPassing},
		{desc: "custom method abc", code: 200, method: "abc", status: api.HealthPassing},

		// custom header
		{desc: "custom header", code: 200, header: http.Header{"A": []string{"b", "c"}}, status: api.HealthPassing},
	}

	for _, tt := range tests {
		desc := tt.desc
		if desc == "" {
			desc = fmt.Sprintf("code %d -> status %s", tt.code, tt.status)
		}
		t.Run(desc, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.method != "" && tt.method != r.Method {
					w.WriteHeader(999)
					return
				}

				expectedHeader := http.Header{
					"Accept":          []string{"text/plain, text/*, */*"},
					"Accept-Encoding": []string{"gzip"},
					"Connection":      []string{"close"},
					"User-Agent":      []string{"Consul Health Check"},
				}
				for k, v := range tt.header {
					expectedHeader[k] = v
				}
				if !reflect.DeepEqual(expectedHeader, r.Header) {
					w.WriteHeader(999)
					return
				}

				// Body larger than 4k limit
				body := bytes.Repeat([]byte{'a'}, 2*CheckBufSize)
				w.WriteHeader(tt.code)
				w.Write(body)
			}))
			defer server.Close()

			notif := mock.NewNotify()
			check := &CheckHTTP{
				Notify:   notif,
				CheckID:  types.CheckID("foo"),
				HTTP:     server.URL,
				Method:   tt.method,
				Header:   tt.header,
				Interval: 10 * time.Millisecond,
				Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
			}
			check.Start()
			defer check.Stop()

			retry.Run(t, func(r *retry.R) {
				if got, want := notif.Updates("foo"), 2; got < want {
					r.Fatalf("got %d updates want at least %d", got, want)
				}
				if got, want := notif.State("foo"), tt.status; got != want {
					r.Fatalf("got state %q want %q", got, want)
				}
				// Allow slightly more data than CheckBufSize, for the header
				if n := len(notif.Output("foo")); n > (CheckBufSize + 256) {
					r.Fatalf("output too long: %d (%d-byte limit)", n, CheckBufSize)
				}
			})
		})
	}
}

func TestCheckHTTPTimeout(t *testing.T) {
	t.Parallel()
	timeout := 5 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(2 * timeout)
	}))
	defer server.Close()

	notif := mock.NewNotify()
	check := &CheckHTTP{
		Notify:   notif,
		CheckID:  types.CheckID("bar"),
		HTTP:     server.URL,
		Timeout:  timeout,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}

	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates("bar"), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State("bar"), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_disablesKeepAlives(t *testing.T) {
	t.Parallel()
	check := &CheckHTTP{
		CheckID:  types.CheckID("foo"),
		HTTP:     "http://foo.bar/baz",
		Interval: 10 * time.Second,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).DisableKeepAlives {
		t.Fatalf("should have disabled keepalives")
	}
}

func TestCheckHTTP_TLSSkipVerify_defaultFalse(t *testing.T) {
	t.Parallel()
	check := &CheckHTTP{
		CheckID:  "foo",
		HTTP:     "https://foo.bar/baz",
		Interval: 10 * time.Second,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}

	check.Start()
	defer check.Stop()

	if check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should default to false")
	}
}

func mockTLSHTTPServer(code int) *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Body larger than 4k limit
		body := bytes.Repeat([]byte{'a'}, 2*CheckBufSize)
		w.WriteHeader(code)
		w.Write(body)
	}))
}

func TestCheckHTTP_TLSSkipVerify_true_pass(t *testing.T) {
	t.Parallel()
	server := mockTLSHTTPServer(200)
	defer server.Close()

	notif := mock.NewNotify()

	check := &CheckHTTP{
		Notify:        notif,
		CheckID:       types.CheckID("skipverify_true"),
		HTTP:          server.URL,
		Interval:      5 * time.Millisecond,
		Logger:        log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
		TLSSkipVerify: true,
	}

	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should be true")
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State("skipverify_true"), api.HealthPassing; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_TLSSkipVerify_true_fail(t *testing.T) {
	t.Parallel()
	server := mockTLSHTTPServer(500)
	defer server.Close()

	notif := mock.NewNotify()

	check := &CheckHTTP{
		Notify:        notif,
		CheckID:       types.CheckID("skipverify_true"),
		HTTP:          server.URL,
		Interval:      5 * time.Millisecond,
		Logger:        log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
		TLSSkipVerify: true,
	}
	check.Start()
	defer check.Stop()

	if !check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should be true")
	}
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.State("skipverify_true"), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckHTTP_TLSSkipVerify_false(t *testing.T) {
	t.Parallel()
	server := mockTLSHTTPServer(200)
	defer server.Close()

	notif := mock.NewNotify()

	check := &CheckHTTP{
		Notify:        notif,
		CheckID:       types.CheckID("skipverify_false"),
		HTTP:          server.URL,
		Interval:      100 * time.Millisecond,
		Logger:        log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
		TLSSkipVerify: false,
	}

	check.Start()
	defer check.Stop()

	if check.httpClient.Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify {
		t.Fatalf("should be false")
	}
	retry.Run(t, func(r *retry.R) {
		// This should fail due to an invalid SSL cert
		if got, want := notif.State("skipverify_false"), api.HealthCritical; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
		if !strings.Contains(notif.Output("skipverify_false"), "certificate signed by unknown authority") {
			r.Fatalf("should fail with certificate error %v", notif.OutputMap())
		}
	})
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
	notif := mock.NewNotify()
	check := &CheckTCP{
		Notify:   notif,
		CheckID:  types.CheckID("foo"),
		TCP:      tcp,
		Interval: 10 * time.Millisecond,
		Logger:   log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
	}
	check.Start()
	defer check.Stop()
	retry.Run(t, func(r *retry.R) {
		if got, want := notif.Updates("foo"), 2; got < want {
			r.Fatalf("got %d updates want at least %d", got, want)
		}
		if got, want := notif.State("foo"), status; got != want {
			r.Fatalf("got state %q want %q", got, want)
		}
	})
}

func TestCheckTCPCritical(t *testing.T) {
	t.Parallel()
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, `127.0.0.1:0`, api.HealthCritical)
	tcpServer.Close()
}

func TestCheckTCPPassing(t *testing.T) {
	t.Parallel()
	var (
		tcpServer net.Listener
	)

	tcpServer = mockTCPServer(`tcp`)
	expectTCPStatus(t, tcpServer.Addr().String(), api.HealthPassing)
	tcpServer.Close()

	tcpServer = mockTCPServer(`tcp6`)
	expectTCPStatus(t, tcpServer.Addr().String(), api.HealthPassing)
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
	notif := mock.NewNotify()
	check := &CheckDocker{
		Notify:            notif,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Shell:             "/bin/sh",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
		dockerClient:      dockerClient,
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should have at least 2 updates
	if notif.Updates("foo") < 2 {
		t.Fatalf("should have 2 updates %v", notif.UpdatesMap())
	}

	if notif.State("foo") != status {
		t.Fatalf("should be %v %v", status, notif.StateMap())
	}

	if notif.Output("foo") != output {
		t.Fatalf("should be %v %v", output, notif.OutputMap())
	}
}

func TestDockerCheckWhenExecReturnsSuccessExitCode(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithNoErrors{}, api.HealthPassing, "output")
}

func TestDockerCheckWhenExecCreationFails(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithCreateExecFailure{}, api.HealthCritical, "Unable to create Exec, error: Exec Creation Failed")
}

func TestDockerCheckWhenExitCodeIsNonZero(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithExecNonZeroExitCode{}, api.HealthCritical, "")
}

func TestDockerCheckWhenExitCodeIsone(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithExecExitCodeOne{}, api.HealthWarning, "output")
}

func TestDockerCheckWhenExecStartFails(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithStartExecFailure{}, api.HealthCritical, "Unable to start Exec: Couldn't Start Exec")
}

func TestDockerCheckWhenExecInfoFails(t *testing.T) {
	t.Parallel()
	expectDockerCheckStatus(t, &fakeDockerClientWithExecInfoErrors{}, api.HealthCritical, "Unable to inspect Exec: Unable to query exec info")
}

func TestDockerCheckDefaultToSh(t *testing.T) {
	t.Parallel()
	os.Setenv("SHELL", "")
	notif := mock.NewNotify()
	check := &CheckDocker{
		Notify:            notif,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
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
	t.Parallel()
	notif := mock.NewNotify()
	os.Setenv("SHELL", "/bin/bash")
	check := &CheckDocker{
		Notify:            notif,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
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
	t.Parallel()
	notif := mock.NewNotify()
	check := &CheckDocker{
		Notify:            notif,
		CheckID:           types.CheckID("foo"),
		Script:            "/health.sh",
		DockerContainerID: "54432bad1fc7",
		Shell:             "/bin/sh",
		Interval:          10 * time.Millisecond,
		Logger:            log.New(ioutil.Discard, UniqueID(), log.LstdFlags),
		dockerClient:      &fakeDockerClientWithLongOutput{},
	}
	check.Start()
	defer check.Stop()

	time.Sleep(50 * time.Millisecond)

	// Allow for extra bytes for the truncation message
	if len(notif.Output("foo")) > CheckBufSize+100 {
		t.Fatalf("output size is too long")
	}
}
